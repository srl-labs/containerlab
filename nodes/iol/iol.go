// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_iol

import (
	"context"
	_ "embed"
	"fmt"
	"path"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"cisco_iol"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	//go:embed iol.cfg
	cfgTemplate string

	InterfaceRegexp = regexp.MustCompile(`(?:e|Ethernet)\s?0/(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "e0/X or Ethernet0/X (where X >= 1) or ethX (where X >= 1)"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(iol)
	}, defaultCredentials)
}

type iol struct {
	nodes.VRNode
}

func (n *iol) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *nodes.NewVRNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.Binds = append(n.Cfg.Binds,
		// mount nvram so that config persists
		fmt.Sprint(path.Join(n.Cfg.LabDir, "nvram"), ":/opt/iol/nvram_00001"),

		// mount launch config
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "startup.cfg"), ":/iol/config.txt"),

		// mount IOYAP and NETMAP for interface mapping
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "iouyap.ini"), ":/iol/iouyap.ini"),
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "NETMAP"), ":/iol/NETMAP"),
	)

	// generate management interface MAC
	hwa, err := utils.GenMac("00:1c:73")
	if err != nil {
		return err
	}

	n.Cfg.EnforceStartupConfig = true

	n.Cfg.MacAddress = hwa.String()

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

	return n.GenStartupConfig(ctx)
}

func (n *iol) GenStartupConfig(ctx context.Context) error {

	nodeCfg := n.Config()

	// set mgmt ipv4 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in iol.Cfg template to configure default route for mgmt
	nodeCfg.MgmtIPv4Gateway = n.Runtime.Mgmt().IPv4Gw
	nodeCfg.MgmtIPv6Gateway = n.Runtime.Mgmt().IPv6Gw

	// os.Remove(path.Join(n.Cfg.LabDir, "startup.cfg"))

	err := n.GenerateConfig(filepath.Join(n.Cfg.LabDir, "startup.cfg"), cfgTemplate)
	if err != nil {
		return err
	}

	return err
}

func (n *iol) CreateIOLFiles(ctx context.Context) error {

	// If NVRAM already exists, don't need to create
	// otherwise saved configs in NVRAM are overwritten.
	if !utils.FileExists(path.Join(n.Cfg.LabDir, "nvram")) {
		// create nvram file
		utils.CreateFile(path.Join(n.Cfg.LabDir, "nvram"), "")
	}

	utils.CreateFile(path.Join(n.Cfg.LabDir, "startup.cfg"), "")

	n.GenInterfaceCfg(ctx)

	return nil
}

func (n *iol) GenInterfaceCfg(_ context.Context) error {

	slot := 0
	port := 0
	iouyapData := "[default]\nbase_port = 49000\nnetmap = /iol/NETMAP\n"
	netmapdata := ""

	for i, intf := range n.Endpoints {

		slot = i / 4
		port = i % 4

		fmt.Printf("Interface: %v, Ethernet%d/%d\n", intf, slot, port)

		iouyapData += fmt.Sprintf("[513:%d/%d]\neth_dev = eth%d", slot, port, i)
		netmapdata += fmt.Sprintf("1:%d/%d 513:%d/%d\n", slot, port, slot, port)
	}

	// create IOYAP and NETMAP file for interface mappings
	utils.CreateFile(path.Join(n.Cfg.LabDir, "iouyap.ini"), iouyapData)
	utils.CreateFile(path.Join(n.Cfg.LabDir, "NETMAP"), netmapdata)

	return nil
}
