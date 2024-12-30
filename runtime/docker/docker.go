// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-units"
	"golang.org/x/sys/unix"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	networkapi "github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/dustin/go-humanize"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

const (
	RuntimeName    = "docker"
	sysctlBase     = "/proc/sys"
	defaultTimeout = 30 * time.Second
	rLimitMaxValue = 1048576
	// defaultDockerNetwork is a name of a docker network that docker uses by default when creating containers.
	defaultDockerNetwork = "bridge"
)

func init() {
	runtime.Register(RuntimeName, func() runtime.ContainerRuntime {
		return &DockerRuntime{
			mgmt: new(types.MgmtNet),
		}
	})
}

type DockerRuntime struct {
	config runtime.RuntimeConfig
	Client *dockerC.Client
	mgmt   *types.MgmtNet
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
		// if the network is succesfully found, set the bridge used by it
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

	// linux bridge name that is used by docker network
	bridgeName := d.mgmt.Bridge

	log.Debugf("Checking if docker network %q exists", d.mgmt.Network)
	netResource, err := d.Client.NetworkInspect(nctx, d.mgmt.Network, networkapi.InspectOptions{})
	switch {
	case dockerC.IsErrNotFound(err):
		bridgeName, err = d.createMgmtBridge(nctx, bridgeName)
		if err != nil {
			return err
		}
	case err == nil:
		log.Debugf("network %q was found. Reusing it...", d.mgmt.Network)
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

	default:
		return err
	}

	if d.mgmt.Bridge == "" {
		d.mgmt.Bridge = bridgeName
	}

	// get management bridge v4/6 addresses and save it under mgmt struct
	// so that nodes can use this information prior to being deployed
	// this was added to allow mgmt network gw ip to be available in a startup config templation step (ceos)
	d.mgmt.IPv4Gw, d.mgmt.IPv6Gw, err = getMgmtBridgeIPs(bridgeName, netResource)
	if err != nil {
		return err
	}

	log.Debugf("Docker network %q, bridge name %q", d.mgmt.Network, bridgeName)

	return d.postCreateNetActions()
}

// skipcq: GO-R1005
func (d *DockerRuntime) createMgmtBridge(nctx context.Context, bridgeName string) (string, error) {
	var err error
	log.Debugf("Network %q does not exist", d.mgmt.Network)
	log.Infof("Creating docker network: Name=%q, IPv4Subnet=%q, IPv6Subnet=%q, MTU=%d",
		d.mgmt.Network, d.mgmt.IPv4Subnet, d.mgmt.IPv6Subnet, d.mgmt.MTU)

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

	// didnt find any gateways, fallthrough to returning the error
	if v4 == "" && v6 == "" {
		return "", "", err
	}

	return v4, v6, nil
}

// postCreateNetActions performs additional actions after the network has been created.
func (d *DockerRuntime) postCreateNetActions() (err error) {
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

	err = os.WriteFile(file, []byte(strconv.Itoa(16384)), 0640) // skipcq: GO-S2306
	if err != nil {
		log.Warnf("failed to enable LLDP on docker bridge: %v", err)
	}

	log.Debugf("Disabling TX checksum offloading for the %s bridge interface...", d.mgmt.Bridge)
	err = utils.EthtoolTXOff(d.mgmt.Bridge)
	if err != nil {
		log.Warnf("failed to disable TX checksum offloading for the %s bridge interface: %v", d.mgmt.Bridge, err)
	}
	err = d.installFwdRule()
	if err != nil {
		log.Warnf("errors during iptables rules install: %v", err)
	}

	return nil
}

// DeleteNet deletes a docker bridge.
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
	err = d.Client.NetworkRemove(nctx, network)
	if err != nil {
		return err
	}

	err = d.deleteFwdRule()
	if err != nil {
		log.Warnf("errors during iptables rules removal: %v", err)
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
	log.Infof("Creating container: %q", node.ShortName)
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

	if err := d.processNetworkMode(ctx, containerNetworkingConfig, containerHostConfig, containerConfig, node); err != nil {
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
func (d *DockerRuntime) PullImage(ctx context.Context, imageName string, pullpolicy types.PullPolicyValue) error {
	log.Debugf("Looking up %s Docker image", imageName)

	canonicalImageName := utils.GetCanonicalImageName(imageName)

	_, b, _ := d.Client.ImageInspectWithRaw(ctx, canonicalImageName)
	switch pullpolicy {
	case types.PullPolicyNever:
		if b == nil {
			// image not found but pull policy = never
			return fmt.Errorf("image %s not found locally, and image-pull-policy=%s prevents containerlab from pulling it", imageName, pullpolicy)
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

	// If Image doesn't exist or pullpolicy=always, we need to pull it
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

	log.Infof("Pulling %s Docker image", canonicalImageName)
	reader, err := d.Client.ImagePull(ctx, canonicalImageName, image.PullOptions{
		RegistryAuth: authString,
	})
	if err != nil {
		return err
	}

	// must read from reader, otherwise image is not properly pulled
	_, _ = io.Copy(io.Discard, reader)
	log.Infof("Done pulling %s", canonicalImageName)

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
		timeout := int(d.config.Timeout)
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
	log.Infof("Removed container: %s", cID)
	return nil
}

// setSysctl writes sysctl data by writing to a specific file.
func setSysctl(sysctl string, newVal int) error {
	return os.WriteFile(path.Join(sysctlBase, sysctl), []byte(strconv.Itoa(newVal)), 0600)
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
