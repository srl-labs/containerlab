// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ros

import (
	"context"
	"fmt"
	"path"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vr-ros", "vr-mikrotik_ros"}
var defaultCredentials = nodes.NewCredentials("admin", "admin")

const (
	configDirName   = "ftpboot"
	startupCfgFName = "config.auto.rsc"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrRos)
	}, defaultCredentials)
}

type vrRos struct {
	nodes.DefaultNode
}

func (s *vrRos) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)
	// set virtualization requirement
	s.HostRequirements.VirtRequired = true

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": s.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.Mgmt.IPv6Subnet,
	}
	s.Cfg.Env = utils.MergeStringMaps(defEnv, s.Cfg.Env)

	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(path.Join(s.Cfg.LabDir, "ftpboot"), ":/ftpboot"))

	if s.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.Cfg.Binds = append(s.Cfg.Binds, "/dev:/dev")
	}

	s.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), s.Cfg.ShortName, s.Cfg.Env["CONNECTION_MODE"])

	return nil
}

func (s *vrRos) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return nodes.LoadStartupConfigFileVr(s, configDirName, startupCfgFName)
}
