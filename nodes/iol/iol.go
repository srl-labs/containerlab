// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_iol

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	typeIOL = "iol"
	typeL2  = "l2"

	iol_workdir = "/iol"

	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "cisco_ios"
	NapalmPlatformName  = "ios"
)

var (
	kindNames          = []string{"cisco_iol"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	//go:embed iol.cfg.tmpl
	cfgTemplate string

	// IntfRegexp with named capture groups for extracting slot and port.
	CapturingIntfRegexp = regexp.MustCompile(`(?:e|Ethernet)\s?(?P<slot>\d+)/(?P<port>\d+)$`)
	// ethX naming is the "raw" or "default" interface naming.
	DefaultIntfRegexp = regexp.MustCompile(`eth[1-9]\d*$`)
	// Match on the management interface.
	MgmtIntfRegexp = regexp.MustCompile(`(eth0|e0/0|Ethernet0/0)$`)
	// Matches on any allowed/legal interface name.
	AllowedIntfRegexp = regexp.MustCompile(`(e|Ethernet)((0/[123])|([1-9]/[0-3]))$|eth[1-9]\d*$`)
	IntfHelpMsg       = "Interfaces should follow Ethernet<slot>/<port> or e<slot>/<port> naming convention, where <slot> is a number from 0-9 and <port> is a number from 0-3. You can also use ethX-based interface naming."

	validTypes = []string{typeIOL, typeL2}
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformAttrs,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(iol)
	}, nrea)
}

type iol struct {
	clabnodes.DefaultNode

	isL2Node          bool
	Pid               string
	nvramFile         string
	partialStartupCfg string
	bootCfg           string
	interfaces        []IOLInterface
	firstBoot         bool
}

func (n *iol) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.firstBoot = false

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	nodeType := strings.ToLower(n.Cfg.NodeType)

	n.Pid = strconv.Itoa(n.Cfg.Index + 1) // n.Cfg.Index is zero-indexed, PID needs to be >= 1

	env := map[string]string{
		"IOL_PID": n.Pid,
	}

	n.Cfg.Env = clabutils.MergeStringMaps(env, n.Cfg.Env)

	// check if user submitted node type is valid
	switch nodeType {
	case "", typeIOL:
		n.isL2Node = false
	case typeL2:
		n.isL2Node = true
	default:
		return fmt.Errorf("invalid node type '%s'. Valid types are: %s",
			n.Cfg.NodeType, strings.Join(validTypes, ", "))
	}

	n.nvramFile = fmt.Sprint("nvram_", fmt.Sprintf("%05s", n.Pid))

	n.Cfg.Binds = append(n.Cfg.Binds,
		// mount nvram so that config persists
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.nvramFile), ":", path.Join(iol_workdir, n.nvramFile)),

		// mount launch config
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "boot_config.txt"), ":/iol/config.txt"),

		// mount IOYAP and NETMAP for interface mapping
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "iouyap.ini"), ":/iol/iouyap.ini"),
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "NETMAP"), ":/iol/NETMAP"),
	)

	return nil
}

func (n *iol) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.CreateIOLFiles(ctx)
}

func (n *iol) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Cisco IOL '%s' node", n.Cfg.ShortName)

	n.GenBootConfig(ctx)

	// Must update mgmt IP if not first boot
	if !n.firstBoot {
		// iol has a 5sec boot delay, wait a few extra secs for the console
		time.Sleep(10 * time.Second)

		return n.UpdateMgmtIntf(ctx)
	}

	return nil
}

func (n *iol) CreateIOLFiles(ctx context.Context) error {
	// If NVRAM already exists, don't need to create
	// otherwise saved configs in NVRAM are overwritten.
	if !clabutils.FileExists(path.Join(n.Cfg.LabDir, n.nvramFile)) {
		// create nvram file
		clabutils.CreateFile(path.Join(n.Cfg.LabDir, n.nvramFile), "")
		n.firstBoot = true
	}

	// create these files so the bind monut doesn't automatically
	// make folders.
	clabutils.CreateFile(path.Join(n.Cfg.LabDir, "boot_config.txt"), "")

	return n.GenInterfaceConfig(ctx)
}

// Generate interfaces configuration for IOL (and iouyap/netmap).
func (n *iol) GenInterfaceConfig(_ context.Context) error {
	// add default 'boilerplate' to NETMAP and iouyap.ini for management port (e0/0)
	iouyapData := "[default]\nbase_port = 49000\nnetmap = /iol/NETMAP\n[513:0/0]\neth_dev = eth0\n"
	netmapdata := fmt.Sprintf("%s:0/0 513:0/0\n", n.Pid)

	slot, port := 0, 0

	// Regexp to pull number out of linux'ethX' interface naming
	IntfRegExpr := regexp.MustCompile(`\d+`)

	for _, intf := range n.Endpoints {
		// get numeric interface number and cast to int
		x, _ := strconv.Atoi(IntfRegExpr.FindString(intf.GetIfaceName()))

		// Interface naming is Ethernet{slot}/{port}. Each slot contains max 4 ports
		slot = x / 4
		port = x % 4

		// append data to write to NETMAP and IOUYAP files
		iouyapData += fmt.Sprintf("[513:%d/%d]\neth_dev = %s\n", slot, port, intf.GetIfaceName())
		netmapdata += fmt.Sprintf("%s:%d/%d 513:%d/%d\n", n.Pid, slot, port, slot, port)

		// populate template array for config
		ipv4Addr := ""
		ipv4Mask := ""
		v4Prefix := intf.GetIPv4Addr()
		ipv6Addr := ""

		if v4Prefix.IsValid() {
			ipv4Addr = v4Prefix.Addr().String()
			ipv4Mask = clabutils.CIDRToDDN(v4Prefix.Bits())
		}

		if a := intf.GetIPv6Addr(); a.IsValid() {
			ipv6Addr = a.String()
		}

		n.interfaces = append(n.interfaces,
			IOLInterface{
				IfaceName: intf.GetIfaceName(),
				IfaceIdx:  x,
				Slot:      slot,
				Port:      port,
				IPv4Addr:  ipv4Addr,
				IPv4Mask:  ipv4Mask,
				IPv6Addr:  ipv6Addr,
			},
		)
	}

	// create IOUYAP and NETMAP file for interface mappings
	err := clabutils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), iouyapData)
	if err != nil {
		return err
	}
	err = clabutils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), netmapdata)

	return err
}

func (n *iol) GenBootConfig(_ context.Context) error {
	n.bootCfg = cfgTemplate

	if n.Cfg.StartupConfig != "" {
		cfg, err := os.ReadFile(n.Cfg.StartupConfig)
		if err != nil {
			return err
		}

		if isPartialConfigFile(n.Cfg.StartupConfig) {
			n.partialStartupCfg = string(cfg)
		} else {
			n.bootCfg = string(cfg)
		}
	}

	// create startup config template
	tpl := IOLTemplateData{
		Hostname:           n.Cfg.ShortName,
		IsL2Node:           n.isL2Node,
		MgmtIPv4Addr:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4SubnetMask: clabutils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
		MgmtIPv4GW:         n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Addr:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLen:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv6GW:         n.Cfg.MgmtIPv6Gateway,
		DataIFaces:         n.interfaces,
		PartialCfg:         n.partialStartupCfg,
	}

	IOLCfgTpl, _ := template.New("clab-iol-default-config").Funcs(
		clabutils.CreateFuncs()).Parse(n.bootCfg)

	// generate the config
	buf := new(bytes.Buffer)
	err := IOLCfgTpl.Execute(buf, tpl)
	if err != nil {
		return err
	}

	return clabutils.CreateFile(path.Join(n.Cfg.LabDir, "boot_config.txt"), buf.String())
}

type IOLTemplateData struct {
	Hostname           string
	IsL2Node           bool
	MgmtIPv4Addr       string
	MgmtIPv4SubnetMask string
	MgmtIPv4GW         string
	MgmtIPv6Addr       string
	MgmtIPv6PrefixLen  int
	MgmtIPv6GW         string
	DataIFaces         []IOLInterface
	PartialCfg         string
}

// IOLInterface struct stores mapping info between
// IOL interface name and linux container interface.
type IOLInterface struct {
	IfaceName string
	IfaceIdx  int
	Slot      int
	Port      int
	IPv4Addr  string
	IPv4Mask  string
	IPv6Addr  string
}

func (*iol) GetMappedInterfaceName(ifName string) (string, error) {
	captureGroups, err := clabutils.GetRegexpCaptureGroups(CapturingIntfRegexp, ifName)
	if err != nil {
		return "", err
	}

	indexGroups := []string{"slot", "port"}
	parsedIndices := make(map[string]int)
	foundIndices := make(map[string]bool)

	for _, indexKey := range indexGroups {
		if index, found := captureGroups[indexKey]; found && index != "" {
			foundIndices[indexKey] = true
			parsedIndices[indexKey], err = strconv.Atoi(index)
			if err != nil {
				return "", fmt.Errorf(
					"%q parsed %s index %q could not be cast to an integer",
					ifName,
					indexKey,
					index,
				)
			}
			if parsedIndices[indexKey] < 0 {
				return "", fmt.Errorf(
					"%q parsed %q index %q does not match requirement >= 0",
					ifName,
					indexKey,
					index,
				)
			}
		} else {
			foundIndices[indexKey] = false
		}
	}

	// return an ethX interface name. Slots are in 'groups' of 4 interfaces each
	if foundIndices["slot"] && foundIndices["port"] {
		return fmt.Sprintf("eth%d", (parsedIndices["slot"]*4)+parsedIndices["port"]), nil
	} else {
		return "", fmt.Errorf("%q missing slot or port index", ifName)
	}
}

// AddEndpoint override maps the endpoint name to an ethX-based naming where necessary, before
// adding it to the node endpoints. Returns an error if the mapping goes wrong or if the
// interface name is NOT allowed.
func (n *iol) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	var IFaceName, IFaceAlias string

	IFaceName = endpointName

	if !(DefaultIntfRegexp.MatchString(endpointName)) &&
		AllowedIntfRegexp.MatchString(endpointName) {
		log.Debugf("%s: %s needs mapping", n.Cfg.ShortName, endpointName)
		mappedName, err := n.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf(
				"%q interface name %q could not be mapped to an ethX-based interface name: %w\n%s",
				n.Cfg.ShortName,
				e.GetIfaceName(),
				err,
				IntfHelpMsg,
			)
		}
		log.Debugf(
			"Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)",
			endpointName,
			mappedName,
		)
		IFaceName = mappedName
		IFaceAlias = endpointName
	}

	e.SetIfaceName(IFaceName)
	// should be nil if ethX naming is used.
	e.SetIfaceAlias(IFaceAlias)
	n.Endpoints = append(n.Endpoints, e)

	return nil
}

func (n *iol) CheckInterfaceName() error {
	err := n.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, e := range n.Endpoints {
		IFaceName := e.GetIfaceName()
		if MgmtIntfRegexp.MatchString(IFaceName) {
			return fmt.Errorf(
				"IOL Node: %q. Management interface Ethernet0/0, e0/0 or eth0 is not allowed",
				n.Cfg.ShortName,
			)
		}

		if !DefaultIntfRegexp.MatchString(IFaceName) {
			return fmt.Errorf(
				"IOL Node %q has an interface named %q which doesn't match the required pattern. %s",
				n.Cfg.ShortName,
				IFaceName,
				IntfHelpMsg,
			)
		}
	}

	return nil
}

// from vr-sros.go
// isPartialConfigFile returns true if the config file name contains .partial substring.
func isPartialConfigFile(c string) bool {
	return strings.Contains(strings.ToUpper(c), ".PARTIAL")
}

func (n *iol) UpdateMgmtIntf(ctx context.Context) error {
	var mgmt_str string
	
	if n.isL2Node {
		// L2 switch needs SVI-based management
		mgmt_str = fmt.Sprintf(
			"\renable\rconfig terminal\rvlan 999\rname clab-mgmt\rexit\rinterface Ethernet0/0\rswitchport mode access\rswitchport access vlan 999\rno shutdown\rexit\rinterface Vlan999\rvrf forwarding clab-mgmt\rip address %s %s\rno ipv6 address\ripv6 address %s/%d\rno shutdown\rexit\rip route vrf clab-mgmt 0.0.0.0 0.0.0.0 %s\ripv6 route vrf clab-mgmt ::/0 %s\rend\rwr\r",
			n.Cfg.MgmtIPv4Address,
			clabutils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
			n.Cfg.MgmtIPv6Address,
			n.Cfg.MgmtIPv6PrefixLength,
			n.Cfg.MgmtIPv4Gateway,
			n.Cfg.MgmtIPv6Gateway,
		)
	} else {
		// L3 router/switch can have routed interface
		mgmt_str = fmt.Sprintf(
			"\renable\rconfig terminal\rinterface Ethernet0/0\rno switchport\rvrf forwarding clab-mgmt\rip address %s %s\rno ipv6 address\ripv6 address %s/%d\rno shutdown\rexit\rip route vrf clab-mgmt 0.0.0.0 0.0.0.0 Ethernet0/0 %s\ripv6 route vrf clab-mgmt ::/0 Ethernet0/0 %s\rend\rwr\r",
			n.Cfg.MgmtIPv4Address,
			clabutils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
			n.Cfg.MgmtIPv6Address,
			n.Cfg.MgmtIPv6PrefixLength,
			n.Cfg.MgmtIPv4Gateway,
			n.Cfg.MgmtIPv6Gateway,
		)
	}

	return n.Runtime.WriteToStdinNoWait(ctx, n.Cfg.ContainerID, []byte(mgmt_str))
}

// SaveConfig is used for "clab save" functionality -- it saves the running config to the startup
// configuration.
func (n *iol) SaveConfig(_ context.Context) error {
	p, err := platform.NewPlatform(
		"cisco_iosxe",
		n.Cfg.LongName,
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(defaultCredentials.GetUsername()),
		options.WithAuthPassword(defaultCredentials.GetPassword()),
	)
	if err != nil {
		return fmt.Errorf("failed to create platform; error: %+v", err)
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		return fmt.Errorf("failed to fetch network driver from the platform; error: %+v", err)
	}

	err = d.Open()
	if err != nil {
		return fmt.Errorf("failed to open driver; error: %+v", err)
	}

	defer d.Close()

	_, err = d.SendCommand("write memory")
	if err != nil {
		return fmt.Errorf("failed to send command; error: %+v", err)
	}

	log.Infof(
		"Successfully copied running configuration to startup configuration file for node: %q\n",
		n.Cfg.ShortName,
	)
	return nil
}
