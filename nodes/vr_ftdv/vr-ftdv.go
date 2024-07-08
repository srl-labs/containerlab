// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ftdv

import (
	"context"
	"fmt"
	"regexp"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"cisco_ftdv"}
	defaultCredentials = nodes.NewCredentials("admin", "Admin@123")

	InterfaceRegexp = regexp.MustCompile(`(?:GigabitEthernet|Gi)\s?0/(?P<port>\d+)`)
	InterfaceOffset = 0
	InterfaceHelp   = "GigabitEthernet0/X or Gi0/X (where X >= 0) or ethX (where X >= 1)"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrFtdv)
	}, defaultCredentials)
}

type vrFtdv struct {
	nodes.VRNode
}

func (n *vrFtdv) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
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
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *vrFtdv) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return nil
}
