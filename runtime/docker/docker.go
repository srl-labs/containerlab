// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path"
	"strconv"
	"strings"
	"time"
	"net"
	"regexp"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-units"
	"golang.org/x/sys/unix"

	"github.com/charmbracelet/log"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	networkapi "github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/dustin/go-humanize"
	"github.com/google/shlex"
	"github.com/moby/term"
	"github.com/srl-labs/containerlab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"golang.org/x/mod/semver"
)

const (
	RuntimeName    = "docker"
	sysctlBase     = "/proc/sys"
	defaultTimeout = 30 * time.Second
	rLimitMaxValue = 1048576
	// defaultDockerNetwork is a name of a docker network that docker uses by default when creating containers.
	defaultDockerNetwork = "bridge"

	natUnprotectedValue         = "nat-unprotected"
	bridgeGatewayModeIPv4Option = "com.docker.network.bridge.gateway_mode_ipv4"
	bridgeGatewayModeIPv6Option = "com.docker.network.bridge.gateway_mode_ipv6"
)

// DeviceMapping represents the device mapping between the host and the container.
type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

func init() {
	runtime.Register(RuntimeName, func() runtime.ContainerRuntime {
		return &DockerRuntime{
			mgmt: new(types.MgmtNet),
		}
	})
}

type DockerRuntime struct {
	config  runtime.RuntimeConfig
	Client  *dockerC.Client
	mgmt    *types.MgmtNet
	version string
}

func (d *DockerRuntime) Init(opts ...runtime.RuntimeOption) error {
	var err error
	log.Debug("Runtime: Docker")
	d.Client, err = dockerC.NewClientWithOpts(dockerC.FromEnv, dockerC.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	for _, o := range opts {
		o(d)
	}
	d.config.VerifyLinkParams = links.NewVerifyLinkParams()

	// Retrieve Docker version to determine whether to apply certain overrides
	dockerVersion, err := d.Client.ServerVersion(context.Background())
	if err != nil {
		return err
	}

	// Needs to be proper SemVer for comparison
	d.version = "v" + dockerVersion.Version

	return nil
}

func (d *DockerRuntime) WithKeepMgmtNet() {
	d.config.KeepMgmtNet = true
}
func (*DockerRuntime) GetName() string                 { return RuntimeName }
func (d *DockerRuntime) Config() runtime.RuntimeConfig { return d.config }

// Mgmt return management network struct of a runtime.
func (d *DockerRuntime) Mgmt() *types.MgmtNet { return d.mgmt }

func (d *DockerRuntime) WithConfig(cfg *runtime.RuntimeConfig) {
	d.config.Timeout = cfg.Timeout
	d.config.Debug = cfg.Debug
	d.config.GracefulShutdown = cfg.GracefulShutdown
	if d.config.Timeout <= 0 {
		d.config.Timeout = defaultTimeout
	}
}

func (d *DockerRuntime) WithMgmtNet(n *types.MgmtNet) {
	d.mgmt = n
	// return if MTU value was set by a user via config file
	if n.MTU != 0 {
		return
	}

	// if the network name is not "clab", which is a default network name used by containerlab
	// then likely a user wants to keep the custom network mtu value.
	// however, if the network name is "clab" and mtu is not provided in the topology file
	// we should detect the mtu value of the default docker network and set it for the clab network
	// as most often this is desired.
	if n.Network == "clab" {
		netRes, err := d.Client.NetworkInspect(context.TODO(), defaultDockerNetwork, networkapi.InspectOptions{})
		if err != nil {
			d.mgmt.MTU = 1500
			log.Debugf("an error occurred when trying to detect docker default network mtu")
		}

		if mtu, ok := netRes.Options["com.docker.network.driver.mtu"]; ok {
			log.Debugf("detected docker network mtu value - %s", mtu)
			d.mgmt.MTU, err = strconv.Atoi(mtu)
			if err != nil {
				log.Errorf("Error parsing MTU value of %q as int", mtu)
			}
		}
	}

	// if bridge was not set in the topo file, find out the bridge name
	// used by the network used in the topology
	if d.mgmt.Bridge == "" && d.mgmt.Network != "" {
		// fetch the network by the name set in the topo and populate the bridge name used by this network
		netRes, err := d.Client.NetworkInspect(context.TODO(), d.mgmt.Network, networkapi.InspectOptions{})
		// if the network is successfully found, set the bridge used by it
		if err == nil {
			if name, exists := netRes.Options["com.docker.network.bridge.name"]; exists {
				d.mgmt.Bridge = name
			} else {
				d.mgmt.Bridge = "br-" + netRes.ID[:12]
			}
			log.Debugf("detected network name in use: %s, backed by a bridge %s", d.mgmt.Network, d.mgmt.Bridge)
		}
	}
}

// CreateNet creates a docker network or reusing if it exists.
func (d *DockerRuntime) CreateNet(ctx context.Context) (err error) {
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	// Determine the driver to use (default to bridge if not specified)
	driver := d.mgmt.Driver
	if driver == "" {
		driver = "bridge"
	}

	// linux bridge name that is used by docker network (only relevant for bridge driver)
	bridgeName := d.mgmt.Bridge

	log.Debugf("Checking if docker network %q exists", d.mgmt.Network)
	netResource, err := d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
	
	switch {
	case dockerC.IsErrNotFound(err):
		// Network doesn't exist, create it based on driver type
		switch driver {
		case "bridge":
			bridgeName, err = d.createMgmtBridge(nctx, bridgeName)
			if err != nil {
				return err
			}
			
		case "macvlan":
			err = d.createMgmtMacvlan(nctx)
			if err != nil {
				return err
			}
			// For macvlan, we don't need to track a bridge name
			bridgeName = ""
			
		default:
			return fmt.Errorf("unsupported network driver: %s", driver)
		}
		
	case err == nil:
		// Network exists, validate it matches expected driver
		log.Debugf("network %q was found. Reusing it...", d.mgmt.Network)
		if netResource.Driver != driver {
			return fmt.Errorf("existing network %q has driver %q but configuration specifies %q", 
				d.mgmt.Network, netResource.Driver, driver)
		}
		
		// Handle existing network based on driver type
		if driver == "bridge" {
			if len(netResource.ID) < 12 {
				return fmt.Errorf("could not get bridge ID")
			}
			switch d.mgmt.Network {
			case "bridge":
				bridgeName = "docker0"
			default:
				if netResource.Options["com.docker.network.bridge.name"] != "" {
					bridgeName = netResource.Options["com.docker.network.bridge.name"]
				} else {
					bridgeName = "br-" + netResource.ID[:12]
				}
			}
		}
		// For macvlan, we just reuse the existing network without any special handling
		
	default:
		return err
	}

	// Only set bridge name for bridge driver
	if driver == "bridge" {
		if d.mgmt.Bridge == "" {
			d.mgmt.Bridge = bridgeName
		}

		// get management bridge v4/6 addresses and save it under mgmt struct
		// so that nodes can use this information prior to being deployed
		// this was added to allow mgmt network gw ip to be available in a startup config template step (ceos)
		d.mgmt.IPv4Gw, d.mgmt.IPv6Gw, err = getMgmtBridgeIPs(bridgeName, netResource)
		if err != nil {
			return err
		}

		log.Debugf("Docker network %q, bridge name %q", d.mgmt.Network, bridgeName)
		return d.postCreateNetActions()
	}

	// For macvlan networks
	if driver == "macvlan" {
		// Re-inspect to get gateway information if network was just created
		if dockerC.IsErrNotFound(err) {
			netResource, err = d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
			if err != nil {
				return fmt.Errorf("failed to inspect newly created macvlan network: %w", err)
			}
		}
		
		// Extract gateway IPs from IPAM config for macvlan
		for _, ipamConfig := range netResource.IPAM.Config {
			if ipamConfig.Gateway != "" {
				// Determine if it's IPv4 or IPv6 based on the address format
				if strings.Count(ipamConfig.Gateway, ".") == 3 {
					d.mgmt.IPv4Gw = ipamConfig.Gateway
				} else if strings.Contains(ipamConfig.Gateway, ":") {
					d.mgmt.IPv6Gw = ipamConfig.Gateway
				}
			}
		}
		
		log.Debugf("Docker macvlan network %q created/reused with parent interface %q", d.mgmt.Network, d.mgmt.MacvlanParent)
		return d.postCreateMacvlanActions()
	}

	return nil
}

// createMgmtMacvlan creates a macvlan network for management
func (d *DockerRuntime) createMgmtMacvlan(nctx context.Context) error {
	// Validate parent interface is specified
	if d.mgmt.MacvlanParent == "" {
		return fmt.Errorf("macvlan-parent interface must be specified for macvlan driver")
	}
	
	// Check if parent interface exists
	if _, err := netlink.LinkByName(d.mgmt.MacvlanParent); err != nil {
		return fmt.Errorf("parent interface %q not found: %v", d.mgmt.MacvlanParent, err)
	}
	
	log.Info("Creating macvlan network",
		"name", d.mgmt.Network,
		"parent", d.mgmt.MacvlanParent,
		"IPv4 subnet", d.mgmt.IPv4Subnet,
		"IPv6 subnet", d.mgmt.IPv6Subnet,
		"mode", d.mgmt.MacvlanMode,
		"aux-address", d.mgmt.MacvlanAux)

	// Prepare IPAM configuration
	var ipamConfig []networkapi.IPAMConfig
	
	// Handle IPv4 configuration
	if d.mgmt.IPv4Subnet != "" && d.mgmt.IPv4Subnet != "auto" {
		ipamCfg := networkapi.IPAMConfig{
			Subnet: d.mgmt.IPv4Subnet,
		}
		if d.mgmt.IPv4Gw != "" {
			ipamCfg.Gateway = d.mgmt.IPv4Gw
		}
		if d.mgmt.IPv4Range != "" {
			ipamCfg.IPRange = d.mgmt.IPv4Range
		}
		// Add aux address if specified
		if d.mgmt.MacvlanAux != "" {
			ipamCfg.AuxAddress = map[string]string{
				"host": d.mgmt.MacvlanAux,
			}
		}
		ipamConfig = append(ipamConfig, ipamCfg)
	}
	
	// Handle IPv6 configuration
	var enableIPv6 bool
	var ipv6_subnet string
	
	if d.mgmt.IPv6Subnet == "auto" {
		var err error
		ipv6_subnet, err = utils.GenerateIPv6ULASubnet()
		if err != nil {
			return err
		}
	} else {
		ipv6_subnet = d.mgmt.IPv6Subnet
	}
	
	if ipv6_subnet != "" {
		ipamCfg := networkapi.IPAMConfig{
			Subnet: ipv6_subnet,
		}
		if d.mgmt.IPv6Gw != "" {
			ipamCfg.Gateway = d.mgmt.IPv6Gw
		}
		if d.mgmt.IPv6Range != "" {
			ipamCfg.IPRange = d.mgmt.IPv6Range
		}
		ipamConfig = append(ipamConfig, ipamCfg)
		enableIPv6 = true
	}
	
	// Prepare network options
	netwOpts := map[string]string{
		"parent": d.mgmt.MacvlanParent,
	}
	
	// Set macvlan mode (default to bridge if not specified)
	macvlanMode := d.mgmt.MacvlanMode
	if macvlanMode == "" {
		macvlanMode = "bridge"
	}
	netwOpts["macvlan_mode"] = macvlanMode
	
	// Add any additional driver options from config
	for k, v := range d.mgmt.DriverOpts {
		log.Debug("Adding macvlan network driver option", "option", k, "value", v)
		netwOpts[k] = v
	}
	
	// Create the network
	opts := networkapi.CreateOptions{
		Driver:     "macvlan",
		EnableIPv6: utils.Pointer(enableIPv6),
		IPAM: &networkapi.IPAM{
			Driver: "default",
			Config: ipamConfig,
		},
		Internal:   false,
		Attachable: false,
		Labels: map[string]string{
			"containerlab": "",
		},
		Options: netwOpts,
	}
	
	_, err := d.Client.NetworkCreate(nctx, d.mgmt.Network, opts)
	if err != nil {
		return fmt.Errorf("failed to create macvlan network: %w", err)
	}
	
	log.Info("Macvlan network created successfully", "name", d.mgmt.Network)
	
	// If aux address was specified, provide instructions for creating host macvlan interface
	if d.mgmt.MacvlanAux != "" {
		log.Info("To enable host communication with containers, create a macvlan interface on the host:")
		log.Infof("  sudo ip link add %s-host link %s type macvlan mode bridge", d.mgmt.Network, d.mgmt.MacvlanParent)
		log.Infof("  sudo ip addr add %s/32 dev %s-host", d.mgmt.MacvlanAux, d.mgmt.Network)
		log.Infof("  sudo ip link set %s-host up", d.mgmt.Network)
		log.Infof("  sudo ip route add %s dev %s-host", d.mgmt.IPv4Subnet, d.mgmt.Network)
	}
	
	return nil
}

// skipcq: GO-R1005
func (d *DockerRuntime) createMgmtBridge(nctx context.Context, bridgeName string) (string, error) {
	var err error
	log.Debug("Network does not exist", "name", d.mgmt.Network)
	log.Info("Creating docker network",
		"name", d.mgmt.Network,
		"IPv4 subnet", d.mgmt.IPv4Subnet,
		"IPv6 subnet", d.mgmt.IPv6Subnet,
		"MTU", d.mgmt.MTU)

	enableIPv6 := false
	var ipamConfig []networkapi.IPAMConfig

	var v4gw, v6gw string
	// check if IPv4/6 addr are assigned to a mgmt bridge
	if d.mgmt.Bridge != "" {
		v4gw, v6gw, err = utils.FirstLinkIPs(d.mgmt.Bridge)
		if err != nil {
			// only return error if the error is not about link not found
			// we will create the bridge if it doesn't exist
			if !errors.As(err, &netlink.LinkNotFoundError{}) {
				return "", err
			}
		}
		log.Debugf("bridge %q has ipv4 addr of %q and ipv6 addr of %q", d.mgmt.Bridge, v4gw, v6gw)
	}

	if d.mgmt.IPv4Subnet != "" && d.mgmt.IPv4Subnet != "auto" {
		if d.mgmt.IPv4Gw != "" {
			v4gw = d.mgmt.IPv4Gw
		}
		ipamCfg := networkapi.IPAMConfig{
			Subnet:  d.mgmt.IPv4Subnet,
			Gateway: v4gw,
		}
		if d.mgmt.IPv4Range != "" {
			ipamCfg.IPRange = d.mgmt.IPv4Range
		}
		ipamConfig = append(ipamConfig, ipamCfg)
	}

	var ipv6_subnet string
	if d.mgmt.IPv6Subnet == "auto" {
		ipv6_subnet, err = utils.GenerateIPv6ULASubnet()
		if err != nil {
			return "", err
		}
	} else {
		ipv6_subnet = d.mgmt.IPv6Subnet
	}

	if ipv6_subnet != "" {
		if d.mgmt.IPv6Gw != "" {
			v6gw = d.mgmt.IPv6Gw
		}
		ipamCfg := networkapi.IPAMConfig{
			Subnet:  ipv6_subnet,
			Gateway: v6gw,
		}
		if d.mgmt.IPv6Range != "" {
			ipamCfg.IPRange = d.mgmt.IPv6Range
		}
		ipamConfig = append(ipamConfig, ipamCfg)
		enableIPv6 = true
	}

	ipam := &networkapi.IPAM{
		Driver: "default",
		Config: ipamConfig,
	}

	netwOpts := map[string]string{
		"com.docker.network.driver.mtu": strconv.Itoa(d.mgmt.MTU),
	}

	if bridgeName != "" {
		netwOpts["com.docker.network.bridge.name"] = bridgeName
	}

	// nat-unprotected mode is needed starting in Docker release 28 to access all ports without exposing them explicitly
	// see https://github.com/srl-labs/containerlab/issues/2638
	if semver.Compare(d.version, "v28.0.0") > 0 {
		log.Debug("Using Docker version 28 or later, enabling NAT unprotected mode on bridge")
		netwOpts[bridgeGatewayModeIPv4Option] = natUnprotectedValue
		netwOpts[bridgeGatewayModeIPv6Option] = natUnprotectedValue
	}

	// Merge in bridge network driver options from topology file
	for k, v := range d.mgmt.DriverOpts {
		log.Debug("Adding bridge network driver option", "option", k, "value", v)
		netwOpts[k] = v
	}

	opts := networkapi.CreateOptions{
		Driver:     "bridge",
		EnableIPv6: utils.Pointer(enableIPv6),
		IPAM:       ipam,
		Internal:   false,
		Attachable: false,
		Labels: map[string]string{
			"containerlab": "",
		},
		Options: netwOpts,
	}

	netCreateResponse, err := d.Client.NetworkCreate(nctx, d.mgmt.Network, opts)
	if err != nil {
		return "", err
	}

	if len(netCreateResponse.ID) < 12 {
		return "", fmt.Errorf("could not get bridge ID")
	}
	// when bridge is not set by a user explicitly
	// we use the 12 chars of docker net as its name
	if bridgeName == "" {
		bridgeName = "br-" + netCreateResponse.ID[:12]
	}
	return bridgeName, nil
}

// getMgmtBridgeIPs gets the management bridge v4/6 addresses.
func getMgmtBridgeIPs(bridgeName string, netResource networkapi.Inspect) (string, string, error) {
	var err error
	var v4, v6 string
	if v4, v6, err = utils.FirstLinkIPs(bridgeName); err != nil {
		log.Warn(
			"failed gleaning v4 and/or v6 addresses from bridge via netlink," +
				" falling back to docker network inspect data",
		)

		for _, ipamEntry := range netResource.IPAM.Config {
			addr := ipamEntry.Gateway

			// this is terribly janky, *but* we know docker will be only putting valid v4/v6
			// addresses in here and we only care about differentiating between the two flavors
			if strings.Count(addr, ".") == 3 {
				v4 = addr
			} else if netResource.EnableIPv6 {
				v6 = addr
			}

			if v4 != "" && v6 != "" {
				break
			}
		}
	}

	// didn't find any gateways, fallthrough to returning the error
	if v4 == "" && v6 == "" {
		return "", "", err
	}

	return v4, v6, nil
}

// postCreateNetActions performs additional actions after the network has been created.
func (d *DockerRuntime) postCreateNetActions() (err error) {
	// Skip all post-creation actions for macvlan networks
	if d.mgmt.Driver == "macvlan" {
		log.Debug("Skipping post-creation actions for macvlan network")
		return nil
	}
	
	// Original bridge-specific actions below...
	log.Debug("Disable RPF check on the docker host")
	err = setSysctl("net/ipv4/conf/all/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'all' scope: %v", err)
	}
	err = setSysctl("net/ipv4/conf/default/rp_filter", 0)
	if err != nil {
		return fmt.Errorf("failed to disable RP filter on docker host for the 'default' scope: %v", err)
	}

	log.Debugf("Enable LLDP on the linux bridge %s", d.mgmt.Bridge)
	file := "/sys/class/net/" + d.mgmt.Bridge + "/bridge/group_fwd_mask"

	err = os.WriteFile(file, []byte(strconv.Itoa(16384)), 0o640) // skipcq: GO-S2306
	if err != nil {
		log.Warnf("failed to enable LLDP on docker bridge: %v", err)
	}

	log.Debugf("Disabling TX checksum offloading for the %s bridge interface...", d.mgmt.Bridge)
	err = utils.EthtoolTXOff(d.mgmt.Bridge)
	if err != nil {
		log.Warnf("failed to disable TX checksum offloading for the %s bridge interface: %v", d.mgmt.Bridge, err)
	}
	err = d.installMgmtNetworkFwdRule()
	if err != nil {
		log.Warnf("errors during iptables rules install: %v", err)
	}

	return nil
}

// postCreateMacvlanActions performs macvlan-specific post-creation actions
func (d *DockerRuntime) postCreateMacvlanActions() error {
	//debugging
	log.Info("Starting macvlan post-creation actions")
	log.Debugf("MacvlanAux: %s, IPv4Subnet: %s", d.mgmt.MacvlanAux, d.mgmt.IPv4Subnet)
	// 1. Verify parent interface exists and is UP
	parentLink, err := netlink.LinkByName(d.mgmt.MacvlanParent)
	if err != nil {
		return fmt.Errorf("failed to get parent interface %s: %w", d.mgmt.MacvlanParent, err)
	}
	
	// Check if interface is UP
	if parentLink.Attrs().OperState != netlink.OperUp {
		log.Warnf("Parent interface %s is not UP (state: %s), containers may not have connectivity", 
			d.mgmt.MacvlanParent, parentLink.Attrs().OperState)
	}
	
	// 2. Check promiscuous mode
	if parentLink.Attrs().Promisc == 0 {
		log.Debugf("Parent interface %s is not in promiscuous mode, enabling it for better macvlan compatibility", 
			d.mgmt.MacvlanParent)
		// Enable promiscuous mode using command execution as a fallback
		if err := enablePromiscuousMode(d.mgmt.MacvlanParent); err != nil {
			log.Warnf("failed to enable promiscuous mode on %s: %v", d.mgmt.MacvlanParent, err)
		}
	}
	
	// 3. Log MTU information
	parentMTU := parentLink.Attrs().MTU
	log.Debugf("Parent interface %s has MTU %d, macvlan interfaces will inherit this", 
		d.mgmt.MacvlanParent, parentMTU)
	
	// 4. Create host macvlan interface if aux address is specified
	if d.mgmt.MacvlanAux != "" {
		if err := d.createHostMacvlanInterface(); err != nil {
			// Don't fail the entire operation, just warn
			log.Warnf("Failed to create host macvlan interface: %v", err)
			log.Info("You can manually create it with:")
			log.Infof("  sudo ip link add %s-host link %s type macvlan mode bridge", 
				d.mgmt.Network, d.mgmt.MacvlanParent)
			log.Infof("  sudo ip addr add %s/%s dev %s-host", 
				d.mgmt.MacvlanAux, getSubnetPrefix(d.mgmt.IPv4Subnet), d.mgmt.Network)
			log.Infof("  sudo ip link set %s-host up", d.mgmt.Network)
		} else {
			log.Infof("Created host macvlan interface %s-host with IP %s", 
				d.mgmt.Network, d.mgmt.MacvlanAux)
		}
	} else {
		// Still warn about the limitation
		log.Info("Note: Host cannot directly communicate with macvlan containers due to kernel limitations. " +
			"Consider setting 'macvlan-aux' to create a host interface.")
	}
	
	return nil
}

func sanitizeAlphanumeric(input string) string {
    re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
    return re.ReplaceAllString(input, "")
}

// createHostMacvlanInterface creates a macvlan interface on the host for container communication
func (d *DockerRuntime) createHostMacvlanInterface() error {
	hostIfNameNonAlpha := d.mgmt.Network + "-host"
	hostIfName := sanitizeAlphanumeric(hostIfNameNonAlpha)
	
	log.Debugf("Creating host macvlan interface: name=%s, parent=%s, mode=%s", 
		hostIfName, d.mgmt.MacvlanParent, d.mgmt.MacvlanMode)
	
	// Check if interface already exists
	if existingLink, err := netlink.LinkByName(hostIfName); err == nil {
		log.Debugf("Host macvlan interface %s already exists", hostIfName)
		// Check if it has the correct IP
		addrs, err := netlink.AddrList(existingLink, netlink.FAMILY_V4)
		if err == nil {
			for _, addr := range addrs {
				if addr.IP.String() == d.mgmt.MacvlanAux {
					log.Debugf("Interface %s already has IP %s", hostIfName, d.mgmt.MacvlanAux)
					return nil
				}
			}
		}
		// Interface exists but might not have the right IP, delete and recreate
		log.Debugf("Removing existing interface %s to recreate with correct settings", hostIfName)
		if err := netlink.LinkDel(existingLink); err != nil {
			log.Warnf("Failed to delete existing interface: %v", err)
		}
	}
	
	// Get parent link
	parentLink, err := netlink.LinkByName(d.mgmt.MacvlanParent)
	if err != nil {
		return fmt.Errorf("parent interface %s not found: %w", d.mgmt.MacvlanParent, err)
	}
	
	log.Debugf("Parent interface details: name=%s, index=%d, mtu=%d, state=%s", 
		parentLink.Attrs().Name, 
		parentLink.Attrs().Index, 
		parentLink.Attrs().MTU,
		parentLink.Attrs().OperState)
	

	// Determine macvlan mode - explicitly handle all cases
	var mode netlink.MacvlanMode
	switch d.mgmt.MacvlanMode {
	case "", "bridge": // default to bridge if empty
		mode = netlink.MACVLAN_MODE_BRIDGE
	case "vepa":
		mode = netlink.MACVLAN_MODE_VEPA
	case "private":
		mode = netlink.MACVLAN_MODE_PRIVATE
	case "passthru":
		mode = netlink.MACVLAN_MODE_PASSTHRU
	default:
		log.Warnf("Unknown macvlan mode %s, defaulting to bridge", d.mgmt.MacvlanMode)
		mode = netlink.MACVLAN_MODE_BRIDGE
	}
	
	log.Debugf("Creating macvlan with mode='%s' value=%d", d.mgmt.MacvlanMode, mode)
	
	log.Debugf("Creating macvlan with mode=%d (0=private, 1=vepa, 2=bridge, 3=passthru)", mode)
	
	// Create macvlan link with minimal attributes first
	macvlan := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        hostIfName,
			ParentIndex: parentLink.Attrs().Index,
		},
		Mode: mode,
	}
	
	// Log the structure before creation
	log.Debugf("Macvlan struct: Name=%s, ParentIndex=%d, Mode=%d", 
		macvlan.LinkAttrs.Name, 
		macvlan.LinkAttrs.ParentIndex, 
		macvlan.Mode)
	
	// Try to create using command line as a test
	cmd := osexec.Command("ip", "link", "add", hostIfName, "link", d.mgmt.MacvlanParent, "type", "macvlan", "mode", "bridge")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Debugf("Command line test failed: %v, output: %s", err, string(output))
		// Don't return here, continue with netlink
	} else {
		log.Debug("Command line creation succeeded, removing to use netlink")
		// Remove it so we can create via netlink
		delCmd := osexec.Command("ip", "link", "del", hostIfName)
		delCmd.Run()
	}
	
	// Create the interface via netlink
	if err := netlink.LinkAdd(macvlan); err != nil {
		// Try to get more error details
		if strings.Contains(err.Error(), "numerical result") {
			log.Errorf("Netlink error details - this often indicates an issue with the parent interface index or mode value")
			log.Errorf("Parent index: %d, Mode: %d", parentLink.Attrs().Index, mode)
		}
		return fmt.Errorf("failed to create macvlan interface: %w", err)
	}
	
	// Rest of the function remains the same...
	log.Debug("Macvlan interface created successfully via netlink")
	
	// Get the created interface
	link, err := netlink.LinkByName(hostIfName)
	if err != nil {
		// Cleanup on failure
		netlink.LinkDel(macvlan)
		return fmt.Errorf("failed to get created interface: %w", err)
	}
	
	// Parse and add IP address
	// Use /32 for the host interface to avoid subnet conflicts
	addrStr := d.mgmt.MacvlanAux + "/32"
	log.Debugf("Adding IP address %s to interface %s", addrStr, hostIfName)
	
	addr, err := netlink.ParseAddr(addrStr)
	if err != nil {
		// Cleanup on failure
		netlink.LinkDel(link)
		return fmt.Errorf("failed to parse IP address %s: %w", addrStr, err)
	}
	
	if err := netlink.AddrAdd(link, addr); err != nil {
		// Cleanup on failure
		netlink.LinkDel(link)
		return fmt.Errorf("failed to add IP address: %w", err)
	}
	
	// Bring the interface up
	if err := netlink.LinkSetUp(link); err != nil {
		// Cleanup on failure
		netlink.LinkDel(link)
		return fmt.Errorf("failed to bring interface up: %w", err)
	}
	
	log.Infof("Created host macvlan interface %s with IP %s", hostIfName, d.mgmt.MacvlanAux)
	
	// Add route to the subnet
	_, ipnet, err := net.ParseCIDR(d.mgmt.IPv4Subnet)
	if err != nil {
		log.Warnf("Failed to parse subnet for route: %v", err)
		return nil
	}
	
	// For the route, we need to use the macvlan interface as the output device
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       ipnet,
		Scope:     netlink.SCOPE_LINK,
	}
	
	if err := netlink.RouteAdd(route); err != nil {
		// Check if route already exists
		if !strings.Contains(err.Error(), "file exists") {
			log.Warnf("Failed to add route %s dev %s: %v", d.mgmt.IPv4Subnet, hostIfName, err)
		}
	} else {
		log.Infof("Added route %s dev %s", d.mgmt.IPv4Subnet, hostIfName)
	}
	
	return nil
}

// getSubnetPrefix extracts the prefix length from a CIDR notation
func getSubnetPrefix(subnet string) string {
	parts := strings.Split(subnet, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return "24" // default
}

// cleanupMacvlanPostActions reverses the changes made in postCreateMacvlanActions
func (d *DockerRuntime) cleanupMacvlanPostActions() error {
	// First, remove the static route if it exists
	if d.mgmt.MacvlanAux != "" && d.mgmt.IPv4Subnet != "" {
		_, ipnet, err := net.ParseCIDR(d.mgmt.IPv4Subnet)
		if err == nil {
			auxIP := net.ParseIP(d.mgmt.MacvlanAux)
			if auxIP != nil {
				// Find and delete the route
				routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
				if err == nil {
					for _, route := range routes {
						if route.Dst != nil && route.Dst.String() == ipnet.String() && 
						   route.Gw != nil && route.Gw.Equal(auxIP) {
							if err := netlink.RouteDel(&route); err != nil {
								log.Debugf("Failed to delete route %s via %s: %v", 
									d.mgmt.IPv4Subnet, d.mgmt.MacvlanAux, err)
							} else {
								log.Infof("Removed route %s via %s", 
									d.mgmt.IPv4Subnet, d.mgmt.MacvlanAux)
							}
							break
						}
					}
				}
			}
		}
	}
	
	// Then cleanup the host interface (this might also remove associated routes)
	if err := d.cleanupHostMacvlanInterface(); err != nil {
		log.Warnf("Failed to cleanup host macvlan interface: %v", err)
	}
	
	// Disable promiscuous mode on parent interface
	parentLink, err := netlink.LinkByName(d.mgmt.MacvlanParent)
	if err != nil {
		// Parent interface might not exist anymore
		log.Debugf("Parent interface %s not found during cleanup: %v", d.mgmt.MacvlanParent, err)
		return nil
	}
	
	// Check if there are other macvlan interfaces using this parent
	links, err := netlink.LinkList()
	if err == nil {
		otherMacvlans := false
		for _, link := range links {
			if macvlan, ok := link.(*netlink.Macvlan); ok {
				if macvlan.ParentIndex == parentLink.Attrs().Index && 
				   macvlan.Name != d.mgmt.Network+"-host" {
					otherMacvlans = true
					break
				}
			}
		}
		
		// Only disable promiscuous mode if no other macvlans are using this parent
		if !otherMacvlans {
			if err := disablePromiscuousMode(d.mgmt.MacvlanParent); err != nil {
				log.Warnf("Failed to disable promiscuous mode on %s: %v", d.mgmt.MacvlanParent, err)
			} else {
				log.Debugf("Disabled promiscuous mode on %s", d.mgmt.MacvlanParent)
			}
		} else {
			log.Debugf("Other macvlan interfaces exist on %s, keeping promiscuous mode enabled", d.mgmt.MacvlanParent)
		}
	}
	
	return nil
}

// enablePromiscuousMode enables promiscuous mode on an interface
func enablePromiscuousMode(ifName string) error {
	// Try using exec to run ip command as a fallback
	cmd := osexec.Command("ip", "link", "set", ifName, "promisc", "on")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable promiscuous mode: %w", err)
	}
	return nil
}

// disablePromiscuousMode disables promiscuous mode on an interface
func disablePromiscuousMode(ifName string) error {
	// Try using exec to run ip command as a fallback
	cmd := osexec.Command("ip", "link", "set", ifName, "promisc", "off")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable promiscuous mode: %w", err)
	}
	return nil
}

// cleanupHostMacvlanInterface removes the host macvlan interface if it exists
func (d *DockerRuntime) cleanupHostMacvlanInterface() error {
	if d.mgmt.MacvlanAux == "" {
		return nil
	}
	
    hostIfNameNonAlpha := d.mgmt.Network + "-host"
    hostIfName := sanitizeAlphanumeric(hostIfNameNonAlpha)

	link, err := netlink.LinkByName(hostIfName)
	if err != nil {
		// Interface doesn't exist, nothing to clean up
		return nil
	}
	
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete host macvlan interface: %w", err)
	}
	
	log.Infof("Removed host macvlan interface %s", hostIfName)
	return nil
}

// DeleteNet deletes a docker bridge or macvlan network.
func (d *DockerRuntime) DeleteNet(ctx context.Context) (err error) {
	network := d.mgmt.Network
	if network == "bridge" || d.config.KeepMgmtNet {
		log.Debugf("Skipping deletion of %q network", network)
		return nil
	}
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	nres, err := d.Client.NetworkInspect(ctx, network, networkapi.InspectOptions{})
	if err != nil {
		return err
	}
	numEndpoints := len(nres.Containers)
	if numEndpoints > 0 {
		if d.config.Debug {
			log.Debugf("network %q has %d active endpoints, deletion skipped", d.mgmt.Network, numEndpoints)
			for _, endp := range nres.Containers {
				log.Debugf("%q is connected to %s", endp.Name, network)
			}
		}
		return nil
	}
	
	// For macvlan networks, cleanup host interface first
	if d.mgmt.Driver == "macvlan" {
		if err := d.cleanupMacvlanPostActions(); err != nil {
			log.Warnf("Failed to cleanup macvlan post-actions: %v", err)
		}
	}
	
	err = d.Client.NetworkRemove(nctx, network)
	if err != nil {
		return err
	}

	// Only run bridge-specific cleanup for bridge networks
	if d.mgmt.Driver != "macvlan" {
		err = d.deleteMgmtNetworkFwdRule()
		if err != nil {
			log.Warnf("errors during iptables rules removal: %v", err)
		}
	}

	return nil
}

// PauseContainer Pauses a container identified by its name.
func (d *DockerRuntime) PauseContainer(ctx context.Context, cID string) error {
	return d.Client.ContainerPause(ctx, cID)
}

// UnpauseContainer UnPauses / resumes a container identified by its name.
func (d *DockerRuntime) UnpauseContainer(ctx context.Context, cID string) error {
	return d.Client.ContainerUnpause(ctx, cID)
}

// CreateContainer creates a docker container (but does not start it).
func (d *DockerRuntime) CreateContainer(ctx context.Context, node *types.NodeConfig) (string, error) { // skipcq: GO-R1005
	log.Info("Creating container", "name", node.ShortName)
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	cmd, err := shlex.Split(node.Cmd)
	if err != nil {
		return "", err
	}

	var entrypoint []string
	if node.Entrypoint != "" {
		entrypoint, err = shlex.Split(node.Entrypoint)
		if err != nil {
			return "", err
		}
	}

	containerConfig := &container.Config{
		Image:        node.Image,
		Entrypoint:   entrypoint,
		Cmd:          cmd,
		Env:          utils.ConvertEnvs(node.Env),
		AttachStdout: true,
		AttachStderr: true,
		Hostname:     node.ShortName,
		Tty:          true,
		OpenStdin:    true,
		User:         node.User,
		Labels:       node.Labels,
		ExposedPorts: node.PortSet,
		MacAddress:   node.MacAddress,
	}

	if node.Healthcheck != nil {
		containerConfig.Healthcheck = &container.HealthConfig{
			Test:        node.Healthcheck.Test,
			Interval:    node.Healthcheck.GetIntervalDuration(),
			Timeout:     node.Healthcheck.GetTimeoutDuration(),
			StartPeriod: node.Healthcheck.GetStartPeriodDuration(),
			Retries:     node.Healthcheck.Retries,
		}
	}

	var resources container.Resources
	if node.Memory != "" {
		mem, err := humanize.ParseBytes(node.Memory)
		if err != nil {
			return "", err
		}
		resources.Memory = int64(mem)
	}
	if node.CPU != 0 {
		resources.CPUQuota = int64(node.CPU * 100000)
		resources.CPUPeriod = 100000
	}
	if node.CPUSet != "" {
		resources.CpusetCpus = node.CPUSet
	}
	var rlimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
		log.Warnf("Unable to retrieve rlimit_NOFILE value: %v", err)
		rlimit.Max = rLimitMaxValue
	}
	if rlimit.Max > rLimitMaxValue {
		rlimit.Max = rLimitMaxValue
	}
	// Iterate through each Device
	for _, str := range node.Devices {
		// Perform your action on each string
		mappings := container.DeviceMapping{}
		mappings.PathOnHost = str
		mappings.PathInContainer = str
		mappings.CgroupPermissions = "rwm"
		resources.Devices = append(resources.Devices, mappings)
	}

	ulimit := units.Ulimit{
		Name: "nofile",
		Hard: int64(rlimit.Max),
		Soft: int64(rlimit.Max),
	}
	resources.Ulimits = []*units.Ulimit{&ulimit}
	containerHostConfig := &container.HostConfig{
		Binds:        node.Binds,
		PortBindings: node.PortBindings,
		Sysctls:      node.Sysctls,
		Privileged:   true,
		PidMode:      "",
		// Network mode will be defined below via switch
		NetworkMode: "",
		ExtraHosts:  node.ExtraHosts, // add static /etc/hosts entries
		Resources:   resources,
		AutoRemove:  node.AutoRemove,
	}

	if node.DNS != nil {
		containerHostConfig.DNS = node.DNS.Servers
		containerHostConfig.DNSSearch = node.DNS.Search
		containerHostConfig.DNSOptions = node.DNS.Options
	}

	containerNetworkingConfig := &networkapi.NetworkingConfig{}

	if node.ShmSize != "" {
		shmsize, err := humanize.ParseBytes(node.ShmSize)
		if err != nil {
			return "", err
		}
		containerHostConfig.ShmSize = int64(shmsize)
	}

	if len(node.CapAdd) > 0 {
		containerHostConfig.CapAdd = append(containerHostConfig.CapAdd, node.CapAdd...)
	}

	if err := d.processNetworkMode(ctx, containerNetworkingConfig, containerHostConfig, containerConfig, node); err != nil {
		return "", err
	}

	if err := d.processPidMode(node, containerHostConfig); err != nil {
		return "", err
	}

	// regular linux containers may benefit from automatic restart on failure
	// note, that veth pairs added to this container (outside of eth0) will be lost on restart
	if !node.AutoRemove && node.RestartPolicy != "" {
		var rp container.RestartPolicyMode
		switch node.RestartPolicy {
		case "no":
			rp = container.RestartPolicyDisabled
		case "always":
			rp = container.RestartPolicyAlways
		case "on-failure":
			rp = container.RestartPolicyOnFailure
		case "unless-stopped":
			rp = container.RestartPolicyUnlessStopped
		}
		containerHostConfig.RestartPolicy.Name = rp
	}

	cont, err := d.Client.ContainerCreate(
		nctx,
		containerConfig,
		containerHostConfig,
		containerNetworkingConfig,
		nil,
		node.LongName,
	)
	log.Debugf("Container %q create response: %+v", node.ShortName, cont)
	if err != nil {
		return "", err
	}
	return cont.ID, nil
}

// GetNSPath inspects a container by its name/id and returns a netns path using the pid of a container.
func (d *DockerRuntime) GetNSPath(ctx context.Context, cID string) (string, error) {
	nctx, cancelFn := context.WithTimeout(ctx, d.config.Timeout)
	defer cancelFn()
	cJSON, err := d.Client.ContainerInspect(nctx, cID)
	if err != nil {
		return "", err
	}

	return "/proc/" + strconv.Itoa(cJSON.State.Pid) + "/ns/net", nil
}

// PullImage pulls the container image using the provided image pull policy value.
func (d *DockerRuntime) PullImage(ctx context.Context, imageName string, pullPolicy types.PullPolicyValue) error {
	log.Debugf("Looking up %s Docker image", imageName)

	canonicalImageName := utils.GetCanonicalImageName(imageName)

	_, b, _ := d.Client.ImageInspectWithRaw(ctx, canonicalImageName)
	switch pullPolicy {
	case types.PullPolicyNever:
		if b == nil {
			// image not found but pull policy = never
			return fmt.Errorf("image %s not found locally, and image-pull-policy=%s prevents containerlab from pulling it", imageName, pullPolicy)
		}
		// image present, all good
		log.Debugf("Image %s present, skip pulling", imageName)
		return nil
	case types.PullPolicyIfNotPresent:
		if b != nil {
			// pull policy == IfNotPresent and image is present
			log.Debugf("Image %s present, skip pulling", imageName)
			return nil
		}
	}

	// If Image doesn't exist or pullPolicy=always, we need to pull it
	authString := ""

	// get docker config based on an empty path (default docker config path will be assumed)
	dockerConfig, err := GetDockerConfig("")
	if err != nil {
		log.Debug("docker config file not found")
	} else {
		authString, err = GetDockerAuth(dockerConfig, canonicalImageName)
		if err != nil {
			return err
		}
	}

	log.Info("Pulling image", "image", canonicalImageName)
	reader, err := d.Client.ImagePull(ctx, canonicalImageName, image.PullOptions{
		RegistryAuth: authString,
	})
	if err != nil {
		return err
	}

	// show pull progress in term
	terminalFd, isTerminal := term.GetFdInfo(os.Stdout)
	_ = jsonmessage.DisplayJSONMessagesStream(reader, os.Stdout, terminalFd, isTerminal, nil)

	log.Info("Done pulling image", "image", canonicalImageName)

	return reader.Close()
}

// StartContainer starts a docker container.
func (d *DockerRuntime) StartContainer(ctx context.Context, cID string, node runtime.Node) (interface{}, error) {
	nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	nodecfg := node.Config()

	log.Debugf("Start container: %q", nodecfg.LongName)
	err := d.Client.ContainerStart(nctx,
		cID,
		container.StartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		},
	)
	if err != nil {
		return nil, err
	}
	log.Debugf("Container started: %q", nodecfg.LongName)
	err = d.postStartActions(ctx, cID, nodecfg)
	return nil, err
}

// postStartActions performs misc. tasks that are needed after the container starts.
func (d *DockerRuntime) postStartActions(ctx context.Context, cID string, node *types.NodeConfig) error {
	nspath, err := d.GetNSPath(ctx, cID)
	if err != nil {
		return err
	}
	err = utils.LinkContainerNS(nspath, node.LongName)
	return err
}

// ListContainers lists all containers using the provided filters.
func (d *DockerRuntime) ListContainers(ctx context.Context, gfilters []*types.GenericFilter) ([]runtime.GenericContainer, error) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	filter := d.buildFilterString(gfilters)

	ctrs, err := d.Client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, err
	}

	var nr []networkapi.Inspect
	if d.mgmt.Network == "" {
		nctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
		defer cancel()

		// fetch containerlab created networks
		f := filters.NewArgs()

		f.Add("label", "containerlab")

		nr, err = d.Client.NetworkList(nctx, networkapi.ListOptions{
			Filters: f,
		})
		if err != nil {
			return nil, err
		}

		// fetch default bridge network
		f = filters.NewArgs()

		f.Add("name", "bridge")

		bridgenet, err := d.Client.NetworkList(nctx, networkapi.ListOptions{
			Filters: f,
		})
		if err != nil {
			return nil, err
		}

		nr = append(nr, bridgenet...)
	}

	return d.produceGenericContainerList(ctx, ctrs, nr)
}

func (d *DockerRuntime) GetContainer(ctx context.Context, cID string) (*runtime.GenericContainer, error) {
	ctrs, err := d.ListContainers(ctx, []*types.GenericFilter{
		{
			FilterType: "name",
			Match:      cID,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(ctrs) != 1 {
		return nil, fmt.Errorf("found unexpected number of containers: %d", len(ctrs))
	}

	return &ctrs[0], nil
}

func (*DockerRuntime) buildFilterString(gFilters []*types.GenericFilter) filters.Args {
	filter := filters.NewArgs()
	for _, gF := range gFilters {
		filterStr := ""
		if gF.Operator == "exists" {
			filterStr = gF.Field
		} else if gF.FilterType == "name" {
			filterStr = fmt.Sprintf("^%s$", gF.Match) // this regexp ensure we have an exact match for name
		} else {
			filterStr = gF.Field + gF.Operator + gF.Match
		}

		log.Debugf("Filter key: %s, filter value: %s", gF.FilterType, filterStr)
		filter.Add(gF.FilterType, filterStr)
	}

	return filter
}

// Transform docker-specific to generic container format.
func (d *DockerRuntime) produceGenericContainerList(ctx context.Context, inputContainers []dockerTypes.Container,
	inputNetworkResources []networkapi.Inspect,
) ([]runtime.GenericContainer, error) {
	var result []runtime.GenericContainer

	for idx := range inputContainers {
		i := inputContainers[idx]

		var names []string
		for _, n := range i.Names {
			// the docker names seem to always come with a "/" in the first position
			// we trim it as slashes are not required in a single host setting
			names = append(names, strings.TrimLeft(n, "/"))
		}

		ctr := runtime.GenericContainer{
			Names:           names,
			ID:              i.ID,
			ShortID:         i.ID[:12],
			Image:           i.Image,
			State:           i.State,
			Status:          i.Status,
			Labels:          i.Labels,
			NetworkSettings: runtime.GenericMgmtIPs{},
		}

		ctr.Ports = make([]*types.GenericPortBinding, len(i.Ports))
		for x, p := range i.Ports {
			ctr.Ports[x] = genericPortFromDockerPort(p)
		}

		ctr.SetRuntime(d)

		bridgeName := d.mgmt.Network

		var err error
		ctr.Pid, err = d.containerPid(ctx, i.ID)
		if err != nil {
			return nil, err
		}

		// if bridgeName is empty, try to find a network created by clab that the container is connected to
		if bridgeName == "" && inputNetworkResources != nil {
			for idx := range inputNetworkResources {
				nr := inputNetworkResources[idx]

				if _, ok := i.NetworkSettings.Networks[nr.Name]; ok {
					bridgeName = nr.Name
					break
				}
			}
		}

		// check if global bridge name belongs to a container network settings
		// applicable for ext-containers
		_, ok := i.NetworkSettings.Networks[bridgeName]

		// if by now we failed to find a docker network name using the network resources created by docker
		// or (in case of external containers) the clab's bridge name doesn't belong to the container
		// we take whatever the first network is listed in the original container network settings
		// this is to derive the network name if the network is not created by clab
		if bridgeName == "" || !ok {
			// only if there is a single network associated with the container
			if len(i.NetworkSettings.Networks) == 1 {
				for n := range i.NetworkSettings.Networks {
					bridgeName = n
				}
			}
		}

		if bridgeName != "" {
			ctr.NetworkName = bridgeName
		} else {
			ctr.NetworkName = "unknown"
		}

		if ifcfg, ok := i.NetworkSettings.Networks[bridgeName]; ok {
			ctr.NetworkSettings.IPv4addr = ifcfg.IPAddress
			ctr.NetworkSettings.IPv4pLen = ifcfg.IPPrefixLen
			ctr.NetworkSettings.IPv6addr = ifcfg.GlobalIPv6Address
			ctr.NetworkSettings.IPv6pLen = ifcfg.GlobalIPv6PrefixLen
			ctr.NetworkSettings.IPv4Gw = ifcfg.Gateway
			ctr.NetworkSettings.IPv6Gw = ifcfg.IPv6Gateway
		}

		// populating mounts information
		var mount runtime.ContainerMount
		for _, m := range i.Mounts {
			mount.Source = m.Source
			mount.Destination = m.Destination
		}
		ctr.Mounts = append(ctr.Mounts, mount)

		result = append(result, ctr)
	}

	return result, nil
}

func genericPortFromDockerPort(p dockerTypes.Port) *types.GenericPortBinding {
	return &types.GenericPortBinding{
		HostIP:        p.IP,
		HostPort:      int(p.PublicPort),
		ContainerPort: int(p.PrivatePort),
		Protocol:      p.Type,
	}
}

// Exec executes cmd on container identified with id and returns stdout, stderr bytes and an error.
func (d *DockerRuntime) Exec(ctx context.Context, cID string, execCmd *exec.ExecCmd) (*exec.ExecResult, error) {
	cont, err := d.Client.ContainerInspect(ctx, cID)
	if err != nil {
		return nil, err
	}
	execID, err := d.Client.ContainerExecCreate(ctx, cID, container.ExecOptions{
		User:         "0", // root
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          execCmd.GetCmd(),
	})
	if err != nil {
		log.Errorf("failed to create exec in container %s: %v", cont.Name, err)
		return nil, err
	}
	log.Debugf("%s exec created %v", cont.Name, cID)

	rsp, err := d.Client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		log.Errorf("failed exec in container %s: %v", cont.Name, err)
		return nil, err
	}
	defer rsp.Close()
	log.Debugf("%s exec attached %v", cont.Name, cID)

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, rsp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// retrieve the exit code via inspect
	execInspect, err := d.Client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return nil, err
	}

	execResult := exec.NewExecResult(execCmd)

	// set the result fields in the exec struct
	execResult.SetReturnCode(execInspect.ExitCode)
	execResult.SetStdOut(outBuf.Bytes())
	execResult.SetStdErr(errBuf.Bytes())
	return execResult, nil
}

// ExecNotWait executes cmd on container identified with id but doesn't wait for output nor attaches stdout/err.
func (d *DockerRuntime) ExecNotWait(_ context.Context, cID string, execCmd *exec.ExecCmd) error {
	execConfig := container.ExecOptions{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: execCmd.GetCmd()}
	respID, err := d.Client.ContainerExecCreate(context.Background(), cID, execConfig)
	if err != nil {
		return err
	}

	execStartCheck := container.ExecStartOptions{}
	_, err = d.Client.ContainerExecAttach(context.Background(), respID.ID, execStartCheck)
	if err != nil {
		return err
	}
	return nil
}

// DeleteContainer tries to stop a container then remove it.
func (d *DockerRuntime) DeleteContainer(ctx context.Context, cID string) error {
	var err error
	force := !d.config.GracefulShutdown
	if d.config.GracefulShutdown {
		log.Infof("Stopping container: %s", cID)
		timeout := int(d.config.Timeout.Seconds())
		err = d.Client.ContainerStop(ctx, cID, container.StopOptions{Timeout: &timeout})
		if err != nil {
			log.Errorf("could not stop container %q: %v", cID, err)
			force = true
		}
	}
	log.Debugf("Removing container: %s", cID)
	err = d.Client.ContainerRemove(ctx, cID, container.RemoveOptions{Force: force, RemoveVolumes: true})
	if err != nil {
		return err
	}
	log.Info("Removed container", "name", cID)

	return nil
}

// setSysctl writes sysctl data by writing to a specific file.
func setSysctl(sysctl string, newVal int) error {
	return os.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0o600)
}

func (d *DockerRuntime) StopContainer(ctx context.Context, name string) error {
	return d.Client.ContainerKill(ctx, name, "kill")
}

// GetHostsPath returns fs path to a file which is mounted as /etc/hosts into a given container.
func (d *DockerRuntime) GetHostsPath(ctx context.Context, cID string) (string, error) {
	inspect, err := d.Client.ContainerInspect(ctx, cID)
	if err != nil {
		return "", err
	}
	hostsPath := inspect.HostsPath
	log.Debugf("Method GetHostsPath was called with a resulting path %q", hostsPath)
	return hostsPath, nil
}

func (*DockerRuntime) processPidMode(node *types.NodeConfig, containerHostConfig *container.HostConfig) error {
	pidMode := container.PidMode(node.PidMode)
	if !pidMode.Valid() {
		return fmt.Errorf("pid mode %q invalid", node.PidMode)
	}

	containerHostConfig.PidMode = pidMode

	return nil
}

func (d *DockerRuntime) processNetworkMode(
	ctx context.Context,
	containerNetworkingConfig *networkapi.NetworkingConfig,
	containerHostConfig *container.HostConfig,
	containerConfig *container.Config,
	node *types.NodeConfig,
) error {
	netMode := strings.SplitN(node.NetworkMode, ":", 2)

	switch netMode[0] {
	case "none":
		containerHostConfig.NetworkMode = "none"
	// clab allows its containers to be attached to a netns of another container
	// this can be a container that is managed by clab, or an external container.
	case "container":
		// We expect exactly two arguments in this case ("container" keyword & cont. name/ID)
		if len(netMode) != 2 {
			return fmt.Errorf("container network mode was specified for container %q, but we failed to parse the network-mode instruction: %q",
				node.ShortName, netMode)
		}

		// also cont. ID shouldn't be empty
		if netMode[1] == "" {
			return fmt.Errorf("container network mode was specified for container %q, referenced container name appears to be empty: %q",
				node.ShortName, netMode)
		}

		// to support using network stack of containers that are scheduled by clab
		// or containers deployed externally (i.e. by k8s kind)
		// we first check if container name referenced in network-mode node property exists internally,
		// aka within clab topology. If it doesn't, we assume that users referred to an external container.
		// extract lab/topo prefix to craft a full container name if an internal container is referenced.
		contName := strings.SplitN(node.LongName, node.ShortName, 2)[0] + netMode[1]

		_, err := d.GetContainer(ctx, contName)
		if err != nil {
			log.Debugf("container %q was not found by its name, assuming it is exists externally with unprefixed", contName)

			// container doesn't exist internally, so we assume it exists externally
			// with a non-prefixed name.
			contName = netMode[1]

			if _, err := d.GetContainer(ctx, contName); err != nil {
				return fmt.Errorf("container %q is referenced in network-mode, but was not found", netMode[1])
			}
		}

		// compile the net spec
		containerHostConfig.NetworkMode = container.NetworkMode("container:" + contName)

		// unset the hostname as it is not supported in this case
		containerConfig.Hostname = ""
	case "host":
		containerHostConfig.NetworkMode = "host"
	default:
		containerHostConfig.NetworkMode = container.NetworkMode(d.mgmt.Network)

		containerNetworkingConfig.EndpointsConfig = map[string]*networkapi.EndpointSettings{
			d.mgmt.Network: {
				IPAMConfig: &networkapi.EndpointIPAMConfig{
					IPv4Address: node.MgmtIPv4Address,
					IPv6Address: node.MgmtIPv6Address,
				},
				Aliases: node.Aliases,
			},
		}
	}

	return nil
}

// GetContainerStatus retrieves the ContainerStatus of the named container.
func (d *DockerRuntime) GetContainerStatus(ctx context.Context, cID string) runtime.ContainerStatus {
	inspect, err := d.Client.ContainerInspect(ctx, cID)
	if err != nil {
		return runtime.NotFound
	}
	switch inspect.State.Status {
	case "running":
		return runtime.Running
	case "created", "paused", "restarting", "removing", "exited", "dead":
		return runtime.Stopped
	}
	return runtime.NotFound
}

// containerPid returns the pid of a container by its ID using inspect.
func (d *DockerRuntime) containerPid(ctx context.Context, cID string) (int, error) {
	inspect, err := d.Client.ContainerInspect(ctx, cID)
	if err != nil {
		return 0, fmt.Errorf("container %q cannot be found", cID)
	}
	return inspect.State.Pid, nil
}

// IsHealthy returns true is the container is reported as being healthy, false otherwise.
func (d *DockerRuntime) IsHealthy(ctx context.Context, cID string) (bool, error) {
	inspect, err := d.Client.ContainerInspect(ctx, cID)
	if err != nil {
		return false, err
	}
	// catch no healthchecks defined
	if inspect.State.Health == nil {
		return false, fmt.Errorf("no health information available for container: %s", cID)
	}
	return inspect.State.Health.Status == "healthy", nil
}

func (d *DockerRuntime) WriteToStdinNoWait(ctx context.Context, cID string, data []byte) error {
	stdin, err := d.Client.ContainerAttach(ctx, cID, container.AttachOptions{
		Stdin:  true,
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return err
	}

	log.Debugf("Writing to %s: %v", cID, data)

	_, err = stdin.Conn.Write(data)
	if err != nil {
		return err
	}

	return stdin.Conn.Close()
}

func (d *DockerRuntime) CheckConnection(ctx context.Context) error {
	_, err := d.Client.Ping(ctx)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("could not connect to the Docker runtime, make sure your user is part of the `docker` group: %w", err)
		} else {
			return fmt.Errorf("could not connect to the Docker runtime: %w", err)
		}
	}

	return nil
}

func (*DockerRuntime) GetRuntimeSocket() (string, error) {
	return "/var/run/docker.sock", nil
}

func (*DockerRuntime) GetCooCBindMounts() types.Binds {
	return types.Binds{
		types.NewBind("/var/lib/docker/containers", "/var/lib/docker/containers", ""),
		types.NewBind("/run/netns", "/run/netns", ""),
	}
}
