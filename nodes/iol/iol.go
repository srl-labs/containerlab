// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_iol

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"cisco_iol"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	//go:embed iol.cfg.tmpl
	cfgTemplate string

	IOLCfgTpl, _ = template.New("clab-iol-default-config").Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).Parse(cfgTemplate)

	IOLMACBase = "1a:2b:3c"

	InterfaceRegexp = regexp.MustCompile(`(?:e|Ethernet)\s?0/(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "e0/X or EthernetX/Y (where X >= 0 and Y >= 1) or ethY (where Y >= 1)"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(iol)
	}, defaultCredentials)
}

type iol struct {
	nodes.VRNode

	isL2Node bool
}

func (n *iol) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *nodes.NewVRNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// check if user submitted node type is valid
	if n.Cfg.NodeType == "" || strings.ToLower(n.Cfg.NodeType) == "iol" {
		n.Cfg.NodeType = "iol"
		n.isL2Node = false
	} else if strings.ToLower(n.Cfg.NodeType) == "l2" {
		n.isL2Node = true
	} else {
		return fmt.Errorf("wrong node type. '%s' doesn't exist. should be any of %s",
			n.Cfg.NodeType, "iol, l2")
	}

	n.Cfg.Binds = append(n.Cfg.Binds,
		// mount nvram so that config persists
		fmt.Sprint(path.Join(n.Cfg.LabDir, "nvram"), ":/iol/nvram_00001"),

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
	if !utils.FileExists(path.Join(n.Cfg.LabDir, "nvram")) {
		// create nvram file
		utils.CreateFile(path.Join(n.Cfg.LabDir, "nvram"), "")
	}

	// create these files so the bind monut doesn't automatically
	// make folders.
	utils.CreateFile(path.Join(n.Cfg.LabDir, "startup.cfg"), "")
	utils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), "")
	utils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), "")

	return nil
}

// Generate interfaces configuration for IOL (and iouyap/netmap)
func (n *iol) GenInterfaceConfig(_ context.Context) error {

	// add default 'boilerplate' to NETMAP and iouyap.ini for management port (e0/0)
	iouyapData := "[default]\nbase_port = 49000\nnetmap = /iol/NETMAP\n[513:0/0]\neth_dev = eth0\n"
	netmapdata := "1:0/0 513:0/0\n"

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

		hwa, _ := utils.GenMac(IOLMACBase)

		// append data to write to NETMAP and IOUYAP files
		iouyapData += fmt.Sprintf("[513:%d/%d]\neth_dev = %s\n", slot, port, intf.GetIfaceName())
		netmapdata += fmt.Sprintf("1:%d/%d 513:%d/%d\n", slot, port, slot, port)

		// populate template array for config
		IOLInterfaces = append(IOLInterfaces,
			IOLInterface{
				intf.GetIfaceName(),
				x,
				slot,
				port,
				hwa.String(),
			},
		)

	}

	// create IOYAP and NETMAP file for interface mappings
	utils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), iouyapData)
	utils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), netmapdata)

	// generate mgmt MAC, it shouldn't be the same as the linux container
	hwa, _ := utils.GenMac(IOLMACBase)

	// create startup config template
	tpl := IOLTemplateData{
		Hostname:           n.Cfg.ShortName,
		IsL2Node:           n.isL2Node,
		MgmtIPv4Addr:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4SubnetMask: CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
		MgmtIPv4GW:         n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Addr:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLen:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv6GW:         n.Cfg.MgmtIPv6Gateway,
		MgmtIntfMacAddr:    hwa.String(),
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

// Convert CIDR bitlength mask to Dotted Decimal Notation
// for usage in Cisco config.
// ie CIDR: /24 is DDN: 255.255.255.0
func CIDRToDDN(length int) string {
	// check mask length is valid
	if length < 0 || length > 32 {
		log.Errorf("Invalid prefix length: %d", length)
		return ""
	}

	mask := net.CIDRMask(length, 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
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
	MgmtIntfMacAddr    string
	DataIFaces         []IOLInterface
}

// IOLinterface struct stores mapping info between
// IOL interface name and linux container interface
type IOLInterface struct {
	IfaceName string
	IfaceIdx  int
	Slot      int
	Port      int
	MacAddr   string
}
