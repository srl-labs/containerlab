// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_vios

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	typeRouter = "router"
	typeSwitch = "switch"
	typeL2     = "l2"

	scrapliPlatformName = "cisco_ios"
)

var (
	kindnames          = []string{"cisco_vios"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	//go:embed vios.cfg
	cfgTemplate string

	InterfaceRegexp = regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`)
	InterfaceOffset = 0
	InterfaceHelp   = "GiX or GigabitEthernetX (where X >= 0) or ethX (where X >= 1)"

	validTypes = []string{typeRouter, typeSwitch, typeL2}
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() clabnodes.Node {
		return new(vios)
	}, nrea)
}

type vios struct {
	clabnodes.VRNode
	isL2Node          bool
	bootCfg           string
	partialStartupCfg string
}

func (n *vios) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// Validate node type
	nodeType := strings.ToLower(n.Cfg.NodeType)
	switch nodeType {
	case "", typeRouter:
		// Default to router type
		n.isL2Node = false
	case typeSwitch, typeL2:
		// L2 switch type
		n.isL2Node = true
	default:
		return fmt.Errorf("invalid node type '%s'. Valid types are: %s",
			n.Cfg.NodeType, strings.Join(validTypes, ", "))
	}

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":       clabnodes.VrDefConnMode,
		"USERNAME":              defaultCredentials.GetUsername(),
		"PASSWORD":              defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR":    n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR":    n.Mgmt.IPv6Subnet,
		"CLAB_MGMT_PASSTHROUGH": "true", // force enable mgmt passthru
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *vios) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}

	configDir := filepath.Join(n.Cfg.LabDir, n.ConfigDirName)
	clabutils.CreateDirectory(configDir, clabconstants.PermissionsOpen)

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

	return nil
}

func (n *vios) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Info("Running postdeploy actions", "kind", n.Cfg.Kind, "node", n.Cfg.ShortName)
	return n.genBootConfig()
}

func (n *vios) genBootConfig() error {
	tplData := ViosTemplateData{
		Hostname:           n.Cfg.ShortName,
		Username:           defaultCredentials.GetUsername(),
		Password:           defaultCredentials.GetPassword(),
		IsL2Node:           n.isL2Node,
		MgmtIPv4Addr:       n.Cfg.MgmtIPv4Address,
		MgmtIPv4SubnetMask: clabutils.CIDRToDDN(n.Cfg.MgmtIPv4PrefixLength),
		MgmtIPv4GW:         n.Cfg.MgmtIPv4Gateway,
		MgmtIPv6Addr:       n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLen:  n.Cfg.MgmtIPv6PrefixLength,
		MgmtIPv6GW:         n.Cfg.MgmtIPv6Gateway,
		PartialCfg:         n.partialStartupCfg,
	}

	viosCfgTpl, err := template.New("vios-config").Funcs(clabutils.CreateFuncs()).Parse(n.bootCfg)
	if err != nil {
		return fmt.Errorf("failed to parse cfg template for node %q: %w", n.Cfg.ShortName, err)
	}

	buf := new(bytes.Buffer)
	err = viosCfgTpl.Execute(buf, tplData)
	if err != nil {
		return fmt.Errorf("failed to execute cfg template for node %q: %w", n.Cfg.ShortName, err)
	}

	configDir := filepath.Join(n.Cfg.LabDir, n.ConfigDirName)
	dstCfg := filepath.Join(configDir, n.StartupCfgFName)
	err = clabutils.CreateFile(dstCfg, buf.String())
	if err != nil {
		return fmt.Errorf("failed to write cfg file for node %q: %w", n.Cfg.ShortName, err)
	}

	return nil
}

// Stores the vars exposed in the config template
type ViosTemplateData struct {
	Hostname           string
	Username           string
	Password           string
	IsL2Node           bool
	MgmtIPv4Addr       string
	MgmtIPv4SubnetMask string
	MgmtIPv4GW         string
	MgmtIPv6Addr       string
	MgmtIPv6PrefixLen  int
	MgmtIPv6GW         string
	PartialCfg         string
}
