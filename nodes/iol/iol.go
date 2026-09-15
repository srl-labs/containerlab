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
	CapturingIntfRegexp = regexp.MustCompile(`^(?:e|Ethernet)\s?(?P<slot>\d+)/(?P<port>\d+)$`)
	// ethX naming is the "raw" or "default" interface naming.
	DefaultIntfRegexp = regexp.MustCompile(`eth[1-9]\d*$`)
	// Matches on any allowed/legal interface name.
	AllowedIntfRegexp = regexp.MustCompile(`(e|Ethernet)((0/[0-3])|([1-9]/[0-3]))$|eth[1-9]\d*$`)
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
	mgmtIntf          string
	mgmtSlot          int
	mgmtPort          int
	mgmtLinuxIdx      int
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

	pid := n.Cfg.Index + 1 // n.Cfg.Index is zero-indexed, PID needs to be >= 1

	// CLAB_IOL_PID_OFFSET shifts the auto-assigned PID, e.g. to keep IDs unique across labs.
	if v, ok := n.Cfg.Env["CLAB_IOL_PID_OFFSET"]; ok {
		offset, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid CLAB_IOL_PID_OFFSET %q: %w", v, err)
		}

		pid += offset
	}

	n.Pid = strconv.Itoa(pid)

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

	if err := n.parseMgmtIntf(); err != nil {
		return err
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

// parseMgmtIntf reads CLAB_IOL_MGMT_INTF from the node env; unset keeps the default Ethernet0/0.
func (n *iol) parseMgmtIntf() error {
	if v, ok := n.Cfg.Env["CLAB_IOL_MGMT_INTF"]; ok && v != "" {
		captureGroups, err := clabutils.GetRegexpCaptureGroups(CapturingIntfRegexp, v)
		if err != nil {
			return fmt.Errorf("invalid CLAB_IOL_MGMT_INTF %q: %w\n%s", v, err, IntfHelpMsg)
		}

		n.mgmtSlot, _ = strconv.Atoi(captureGroups["slot"])
		n.mgmtPort, _ = strconv.Atoi(captureGroups["port"])

		if n.mgmtSlot > 9 || n.mgmtPort > 3 {
			return fmt.Errorf(
				"invalid CLAB_IOL_MGMT_INTF %q: slot must be 0-9 and port must be 0-3", v)
		}
	}

	n.mgmtIntf = fmt.Sprintf("Ethernet%d/%d", n.mgmtSlot, n.mgmtPort)
	n.mgmtLinuxIdx = n.mgmtSlot*4 + n.mgmtPort

	return nil
}

// swapMgmtIdx swaps linux interface index 0 with the management interface's index,
// so Ethernet0/0 takes the index a relocated management interface vacated.
func (n *iol) swapMgmtIdx(idx int) int {
	switch idx {
	case 0:
		return n.mgmtLinuxIdx
	case n.mgmtLinuxIdx:
		return 0
	}

	return idx
}

// iolPortForLinuxIdx returns the IOL slot/port backed by linux interface index idx.
func (n *iol) iolPortForLinuxIdx(idx int) (slot, port int) {
	mapped := n.swapMgmtIdx(idx)
	return mapped / 4, mapped % 4
}

// ensureNumSlotsEnv exports CLAB_IOL_NUM_SLOTS when the management interface is
// relocated, so that its slot exists even when no link lands in it.
func (n *iol) ensureNumSlotsEnv() {
	if n.mgmtLinuxIdx == 0 || n.Cfg.Env["CLAB_IOL_NUM_SLOTS"] != "" {
		return
	}

	intfIdxRegexp := regexp.MustCompile(`\d+`)
	needed := n.mgmtSlot + 1

	for _, ep := range n.Endpoints {
		x, _ := strconv.Atoi(intfIdxRegexp.FindString(ep.GetIfaceName()))
		if slot, _ := n.iolPortForLinuxIdx(x); slot+1 > needed {
			needed = slot + 1
		}
	}

	n.Cfg.Env["CLAB_IOL_NUM_SLOTS"] = strconv.Itoa(needed)
}

func (n *iol) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	n.ensureNumSlotsEnv()

	return n.CreateIOLFiles(ctx)
}

func (n *iol) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Cisco IOL '%s' node", n.Cfg.ShortName)

	// Disable TX checksum offload on the host NS veth for the mgmt interface.
	var peerIfIndex int
	err := n.ExecFunction(ctx, clabutils.VethPeerIndex("eth0", &peerIfIndex))
	if err != nil {
		log.Warn("Failed to get veth peer index for IOL mgmt interface",
			"node", n.Cfg.ShortName,
			"error", err)
		return nil
	}

	if err := clabutils.DisableTxOffloadByIndex(peerIfIndex); err != nil {
		log.Warn("Failed to disable TX checksum offload on IOL mgmt host veth",
			"node", n.Cfg.ShortName,
			"error", err)
	}

	n.GenBootConfig(ctx)

	// Must update mgmt IP if not first boot
	if !n.firstBoot {
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
	// add 'boilerplate' to NETMAP and iouyap.ini for the management port
	iouyapData := fmt.Sprintf("[default]\nbase_port = 49000\nnetmap = /iol/NETMAP\n[513:%d/%d]\neth_dev = eth0\n",
		n.mgmtSlot, n.mgmtPort)
	netmapdata := fmt.Sprintf("%s:%d/%d 513:%d/%d\n", n.Pid, n.mgmtSlot, n.mgmtPort, n.mgmtSlot, n.mgmtPort)

	slot, port := 0, 0

	// Regexp to pull number out of linux'ethX' interface naming
	IntfRegExpr := regexp.MustCompile(`\d+`)

	for _, intf := range n.Endpoints {
		// get numeric interface number and cast to int
		x, _ := strconv.Atoi(IntfRegExpr.FindString(intf.GetIfaceName()))

		// Interface naming is Ethernet{slot}/{port}. Each slot contains max 4 ports
		slot, port = n.iolPortForLinuxIdx(x)

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

		if clabutils.IsPartialConfigFile(n.Cfg.StartupConfig) {
			n.partialStartupCfg = string(cfg)
		} else {
			n.bootCfg = string(cfg)
		}
	}

	// create startup config template
	tpl := IOLTemplateData{
		Hostname:           n.Cfg.ShortName,
		IsL2Node:           n.isL2Node,
		MgmtIntf:           n.mgmtIntf,
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
	MgmtIntf           string
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

func (n *iol) GetMappedInterfaceName(ifName string) (string, error) {
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
		idx := n.swapMgmtIdx((parsedIndices["slot"] * 4) + parsedIndices["port"])
		return fmt.Sprintf("eth%d", idx), nil
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
		// eth0 always backs the management interface
		if IFaceName == "eth0" {
			return fmt.Errorf(
				"IOL Node: %q. Management interface %s (eth0) is not allowed",
				n.Cfg.ShortName,
				n.mgmtIntf,
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

func (n *iol) UpdateMgmtIntf(ctx context.Context) error {
	// ponytail: fixed sleep heuristic; proper fix is to read PTY output until
	// an interactive prompt pattern is detected. Upgrade by implementing a
	// PTY reader in WriteToStdinNoWait or adding a console-ready probe.
	//
	// IOL has a 5s startup countdown followed by NVRAM loading (~10-20s
	// depending on config size and system load). Commands written to the PTY
	// during NVRAM loading are consumed mid-boot and silently lost, which
	// leaves the management interface unconfigured. Wait 25s so the first
	// attempt lands after the console is interactive.
	time.Sleep(25 * time.Second)

	// Prefix with \rend\r to exit any lingering config mode left by a
	// previous partial attempt before re-entering the command sequence.
	// All IOS commands here are idempotent; repeating them is safe.
	mgmt_str := fmt.Sprintf(
		"\rend\renable\rconfig terminal\rinterface %s\rip address %s %s\rno ipv6 address\ripv6 address %s/%d\rexit\rip route vrf clab-mgmt 0.0.0.0 0.0.0.0 %s %s\ripv6 route vrf clab-mgmt ::/0 %s %s\rend\rwr\r",
		n.mgmtIntf,
		n.Cfg.MgmtIPv4Address,
		clabutils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
		n.Cfg.MgmtIPv6Address,
		n.Cfg.MgmtIPv6PrefixLength,
		n.mgmtIntf,
		n.Cfg.MgmtIPv4Gateway,
		n.mgmtIntf,
		n.Cfg.MgmtIPv6Gateway,
	)
	data := []byte(mgmt_str)

	var lastErr error
	for i := range 3 {
		if i > 0 {
			time.Sleep(10 * time.Second)
		}
		lastErr = n.Runtime.WriteToStdinNoWait(ctx, n.Cfg.ContainerID, data)
		if lastErr != nil {
			log.Warnf("UpdateMgmtIntf: attempt %d/3 failed for %s: %v",
				i+1, n.Cfg.ShortName, lastErr)
		}
	}
	return lastErr
}

// SaveConfig is used for "clab save" functionality -- it saves the running config to the startup
// configuration.
func (n *iol) SaveConfig(_ context.Context) (*clabnodes.SaveConfigResult, error) {
	p, err := platform.NewPlatform(
		"cisco_iosxe",
		n.Cfg.LongName,
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(n.Cfg.Credentials.Username),
		options.WithAuthPassword(n.Cfg.Credentials.Password),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create platform; error: %+v", err)
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network driver from the platform; error: %+v", err)
	}

	err = d.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open driver; error: %+v", err)
	}

	defer d.Close()

	_, err = d.SendCommand("write memory")
	if err != nil {
		return nil, fmt.Errorf("failed to send command; error: %+v", err)
	}

	log.Infof(
		"Successfully copied running configuration to startup configuration file for node: %q\n",
		n.Cfg.ShortName,
	)
	return nil, nil
}
