// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_iol

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	typeIOL = "iol"
	typeL2  = "l2"

	iol_workdir = "/iol"
)

var (
	kindnames          = []string{"cisco_iol"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	//go:embed iol.cfg.tmpl
	cfgTemplate string

	IOLCfgTpl, _ = template.New("clab-iol-default-config").Funcs(
		gomplate.CreateFuncs(context.Background(), new(data.Data))).Parse(cfgTemplate)

	InterfaceRegexp = regexp.MustCompile(`(?:e|Ethernet)\s?(?P<slot>\d+)/(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "eX/Y or EthernetX/Y (where X >= 0 and Y >= 1)"

	validTypes = []string{typeIOL, typeL2}
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(iol)
	}, defaultCredentials)
}

type iol struct {
	nodes.DefaultNode

	isL2Node  bool
	Pid       string
	nvramFile string
}

func (n *iol) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	nodeType := strings.ToLower(n.Cfg.NodeType)

	n.Pid = strconv.Itoa(n.Cfg.Index + 1) // n.Cfg.Index is zero-indexed, PID needs to be >= 1

	env := map[string]string{
		"IOL_PID": n.Pid,
	}

	n.Cfg.Env = utils.MergeStringMaps(env, n.Cfg.Env)

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
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "startup.cfg"), ":/iol/config.txt"),

		// mount IOYAP and NETMAP for interface mapping
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "iouyap.ini"), ":/iol/iouyap.ini"),
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "NETMAP"), ":/iol/NETMAP"),
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *iol) PreDeploy(ctx context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.CreateIOLFiles(ctx)
}

func (n *iol) PostDeploy(ctx context.Context, params *nodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Cisco IOL '%s' node", n.Cfg.ShortName)

	return n.GenInterfaceConfig(ctx)
}

func (n *iol) CreateIOLFiles(ctx context.Context) error {
	// If NVRAM already exists, don't need to create
	// otherwise saved configs in NVRAM are overwritten.
	if !utils.FileExists(path.Join(n.Cfg.LabDir, n.nvramFile)) {
		// create nvram file
		utils.CreateFile(path.Join(n.Cfg.LabDir, n.nvramFile), "")
	}

	// create these files so the bind monut doesn't automatically
	// make folders.
	utils.CreateFile(path.Join(n.Cfg.LabDir, "startup.cfg"), "")
	utils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), "")
	utils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), "")

	return nil
}

// Generate interfaces configuration for IOL (and iouyap/netmap).
func (n *iol) GenInterfaceConfig(_ context.Context) error {
	// add default 'boilerplate' to NETMAP and iouyap.ini for management port (e0/0)
	iouyapData := "[default]\nbase_port = 49000\nnetmap = /iol/NETMAP\n[513:0/0]\neth_dev = eth0\n"
	netmapdata := fmt.Sprintf("%s:0/0 513:0/0\n", n.Pid)

	slot, port := 0, 0

	IOLInterfaces := []IOLInterface{}

	// Regexp to pull number out of linux'ethX' interface naming
	IntfRegExpr := regexp.MustCompile("[0-9]+")

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
		IOLInterfaces = append(IOLInterfaces,
			IOLInterface{
				intf.GetIfaceName(),
				x,
				slot,
				port,
			},
		)

	}

	// create IOYAP and NETMAP file for interface mappings
	utils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), iouyapData)
	utils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), netmapdata)

	// create startup config template
	tpl := IOLTemplateData{
		Hostname:           n.Cfg.ShortName,
		IsL2Node:           n.isL2Node,
		MgmtIPv4Addr:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4SubnetMask: utils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
		MgmtIPv4GW:         n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Addr:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLen:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv6GW:         n.Cfg.MgmtIPv6Gateway,
		DataIFaces:         IOLInterfaces,
	}

	// generate the config
	buf := new(bytes.Buffer)
	err := IOLCfgTpl.Execute(buf, tpl)
	if err != nil {
		return err
	}
	// write it to disk
	utils.CreateFile(path.Join(n.Cfg.LabDir, "startup.cfg"), buf.String())

	return err
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
}

// IOLinterface struct stores mapping info between
// IOL interface name and linux container interface.
type IOLInterface struct {
	IfaceName string
	IfaceIdx  int
	Slot      int
	Port      int
}

func (n *iol) GetMappedInterfaceName(ifName string) (string, error) {
	captureGroups, err := utils.GetRegexpCaptureGroups(n.InterfaceRegexp, ifName)
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
				return "", fmt.Errorf("%q parsed %s index %q could not be cast to an integer", ifName, indexKey, index)
			}
			if !(parsedIndices[indexKey] >= 0) {
				return "", fmt.Errorf("%q parsed %q index %q does not match requirement >= 0", ifName, indexKey, index)
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

var DefaultIntfRegexp = regexp.MustCompile(`eth[1-9][0-9]*$`)

// AddEndpoint override version maps the endpoint name to an ethX-based name before adding it to the node endpoints. Returns an error if the mapping goes wrong.
func (n *iol) AddEndpoint(e links.Endpoint) error {
	endpointName := e.GetIfaceName()
	// Slightly modified check: if it doesn't match the DefaultIntfRegexp, pass it to GetMappedInterfaceName. If it fails, then the interface name is wrong.
	if n.InterfaceRegexp != nil && !(DefaultIntfRegexp.MatchString(endpointName)) {
		mappedName, err := n.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf("%q interface name %q could not be mapped to an ethX-based interface name: %w",
				n.Cfg.ShortName, e.GetIfaceName(), err)
		}
		log.Debugf("Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)", endpointName, mappedName)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}
	n.Endpoints = append(n.Endpoints, e)

	return nil
}

func (n *iol) CheckInterfaceName() error {
	// allow interface naming as Ethernet<slot>/<port> or e<slot>/<port>
	InterfaceRegexp := regexp.MustCompile("Ethernet((0/[1-3])|([1-9]/[0-3]))$|e((0/[1-3])|([1-9]/[0-9]))$")

	err := n.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, e := range n.Endpoints {
		IFaceName := e.GetIfaceAlias()
		if !InterfaceRegexp.MatchString(IFaceName) {
			return fmt.Errorf("IOL Node %q has an interface named %q which doesn't match the required pattern. Interfaces should be defined contigiously and named as Ethernet<slot>/<port> or e<slot>/<port>, where <slot> is a number from 0-9 and <port> is a number from 0-3. Management interface Ethernet0/0 cannot be used", n.Cfg.ShortName, IFaceName)
		}
	}

	return nil
}
