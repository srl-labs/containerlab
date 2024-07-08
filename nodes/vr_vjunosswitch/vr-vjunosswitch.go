// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_vjunosswitch

import (
	"context"
	"fmt"
	"path"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"juniper_vjunosrouter", "juniper_vjunosswitch"}
	defaultCredentials = nodes.NewCredentials("admin", "admin@123")

	InterfaceRegexp = regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`)
	InterfaceOffset = 0
	InterfaceHelp   = "(et|xe|ge)-0/0/X (where X >= 0) or ethX (where X >= 1)"
)

const (
	scrapliPlatformName = "juniper_junos"

	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrVJUNOSSWITCH)
	}, defaultCredentials)
}

type vrVJUNOSSWITCH struct {
	nodes.VRNode
}

func (n *vrVJUNOSSWITCH) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *nodes.NewVRNode(n)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *vrVJUNOSSWITCH) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return nodes.LoadStartupConfigFileVr(n, configDirName, startupCfgFName)
}

func (n *vrVJUNOSSWITCH) SaveConfig(_ context.Context) error {
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
