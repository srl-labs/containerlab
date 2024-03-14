// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package c8000

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"c8000", "cisco_c8000"}
	defaultCredentials = kind_registry.NewCredentials("cisco", "cisco123")

	//go:embed c8000.cfg
	cfgTemplate string
)

const (
	scrapliPlatformName = "cisco_iosxr"
)

// Register registers the node in the NodeRegistry.
func Register(r *kind_registry.KindRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(c8000)
	}, defaultCredentials)
}

type c8000 struct {
	nodes.DefaultNode
}

func (n *c8000) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.Binds = append(n.Cfg.Binds,
		// mount first-boot config file
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "first-boot.cfg"), ":/startup.cfg"),
	)

	return nil
}

func (n *c8000) PreDeploy(ctx context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)

	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.create8000Files(ctx)
}

func (n *c8000) SaveConfig(_ context.Context) error {
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

func (n *c8000) create8000Files(_ context.Context) error {
	nodeCfg := n.Config()

	// generate first-boot config
	cfg := filepath.Join(n.Cfg.LabDir, "first-boot.cfg")
	nodeCfg.ResStartupConfig = cfg

	// set mgmt IPv4/IPv6 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in c8000.Cfg template to configure default route for mgmt
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

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *c8000) CheckInterfaceName() error {
	ifRe := regexp.MustCompile(`^(Hu|FH)0_0_0_\d+$`)

	for _, e := range n.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("cisco 8000 interface name %q doesn't match the required pattern. Cisco 8000 interfaces should be named as Hu0_0_0_X (100G interfaces) or FH0_0_0_X (400G interfaces) where X is the interface number", e.GetIfaceName())
		}
	}

	return nil
}
