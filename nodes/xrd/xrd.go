// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package xrd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"xrd", "cisco_xrd"}
	defaultCredentials = nodes.NewCredentials("clab", "clab@123")
	xrdEnv             = map[string]string{
		"XR_FIRST_BOOT_CONFIG": "/etc/xrd/first-boot.cfg",
		"XR_MGMT_INTERFACES":   "linux:eth0,xr_name=Mg0/RP0/CPU0/0,chksum,snoop_v4,snoop_v6",
		"XR_EVERY_BOOT_SCRIPT": "/etc/xrd/mgmt_intf_v6_addr.sh",
	}

	//go:embed mgmt_intf_v6_addr.sh.tmpl
	scriptTemplate string

	xrdMgmtScriptTpl, _ = template.New("clab-xrd-mgmt-ipv6-script").Funcs(
		gomplate.CreateFuncs(context.Background(), new(data.Data))).Parse(scriptTemplate)

	//go:embed xrd.cfg
	cfgTemplate string
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "cisco_iosxr"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes)

	r.Register(kindNames, func() nodes.Node {
		return new(xrd)
	}, nrea)
}

type xrd struct {
	nodes.DefaultNode
}

func (n *xrd) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.Binds = append(n.Cfg.Binds,
		// mount first-boot config file
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "first-boot.cfg"), ":/etc/xrd/first-boot.cfg"),
		// persist data by mounting /xr-storage
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "xr-storage"), ":/xr-storage"),
		// management IPv6 address script
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "mgmt_intf_v6_addr.sh"), ":/etc/xrd/mgmt_intf_v6_addr.sh"),
	)

	return nil
}

func (n *xrd) PreDeploy(ctx context.Context, params *nodes.PreDeployParams) error {
	n.genInterfacesEnv()

	utils.CreateDirectory(n.Cfg.LabDir, 0777)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.createXRDFiles(ctx)
}

func (n *xrd) PostDeploy(_ context.Context, _ *nodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Cisco XRd '%s' node", n.Cfg.ShortName)

	// create interface script template
	tpl := xrdScriptTmpl{
		MgmtIPv6Addr:      n.Cfg.MgmtIPv6Address,
		MgmtIPv6PrefixLen: n.Cfg.MgmtIPv6PrefixLength,
	}

	// create script from template
	buf := new(bytes.Buffer)
	err := xrdMgmtScriptTpl.Execute(buf, tpl)
	if err != nil {
		return err
	}
	// write it to disk
	utils.CreateFile(filepath.Join(n.Cfg.LabDir, "mgmt_intf_v6_addr.sh"), buf.String())

	return err
}

func (n *xrd) SaveConfig(_ context.Context) error {
	err := netconf.SaveConfig(n.Cfg.LongName,
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		scrapliPlatformName,
	)
	if err != nil {
		return err
	}

	log.Infof("saved %s running configuration to startup configuration file\n", n.Cfg.ShortName)
	return nil
}

func (n *xrd) createXRDFiles(_ context.Context) error {
	nodeCfg := n.Config()
	// generate xr-storage directory
	utils.CreateDirectory(filepath.Join(n.Cfg.LabDir, "xr-storage"), 0777)
	// generate first-boot config
	cfg := filepath.Join(n.Cfg.LabDir, "first-boot.cfg")
	nodeCfg.ResStartupConfig = cfg

	mgmt_script_path := filepath.Join(n.Cfg.LabDir, "mgmt_intf_v6_addr.sh")

	// generate script file
	if !utils.FileExists(mgmt_script_path) {
		utils.CreateFile(mgmt_script_path, "")
		os.Chmod(mgmt_script_path, 0775) // skipcq: GSC-G302
	}

	// set mgmt IPv4/IPv6 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in xrd.Cfg template to configure default route for mgmt
	nodeCfg.MgmtIPv4Gateway = n.Runtime.Mgmt().IPv4Gw
	nodeCfg.MgmtIPv6Gateway = n.Runtime.Mgmt().IPv6Gw

	// use startup config file provided by a user
	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err := n.GenerateConfig(nodeCfg.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}

	return err
}

// genInterfacesEnv populates the content of a required env var that sets the interface mapping rules.
func (n *xrd) genInterfacesEnv() {
	// xrd-control-plane variant needs XR_INTERFACE ENV var to be populated for all active interface
	// here we take the number of links users set in the topology to get the right # of links
	var interfaceEnvVar string

	for _, ep := range n.Endpoints {
		// ifName is a linux interface name with dashes swapped for slashes to be used in the config
		ifName := strings.ReplaceAll(ep.GetIfaceName(), "-", "/")
		interfaceEnvVar += fmt.Sprintf("linux:%s,xr_name=%s;", ep.GetIfaceName(), ifName)
	}

	interfaceEnv := map[string]string{"XR_INTERFACES": interfaceEnvVar}

	n.Cfg.Env = utils.MergeStringMaps(xrdEnv, interfaceEnv, n.Cfg.Env)
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *xrd) CheckInterfaceName() error {
	ifRe := regexp.MustCompile(`^Gi0-0-0-\d+$`)
	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("cisco XRd interface name %q doesn't match the required pattern. XRd interfaces should be named as Gi0-0-0-X where X is the interface number", e.GetIfaceName())
		}
	}

	return nil
}

type xrdScriptTmpl struct {
	MgmtIPv6Addr      string
	MgmtIPv6PrefixLen int
}
