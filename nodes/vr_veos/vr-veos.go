// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_veos

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func init() {
	nodes.Register(nodes.NodeKindVrVEOS, func() nodes.Node {
		return new(vrVEOS)
	})
}

type vrVEOS struct {
	cfg  *types.NodeConfig
	mgmt *types.MgmtNet
}

func (s *vrVEOS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           "admin",
		"PASSWORD":           "admin",
		"DOCKER_NET_V4_ADDR": s.mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.mgmt.IPv6Subnet,
	}
	s.cfg.Env = utils.MergeStringMaps(defEnv, s.cfg.Env)

	if s.cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.cfg.Binds = append(s.cfg.Binds, "/dev:/dev")
	}

	s.cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		s.cfg.Env["USERNAME"], s.cfg.Env["PASSWORD"], s.cfg.ShortName, s.cfg.Env["CONNECTION_MODE"])
	return nil
}

func (s *vrVEOS) Config() *types.NodeConfig { return s.cfg }

func (s *vrVEOS) PreDeploy(configName, labCADir, labCARoot string) error { return nil }

func (s *vrVEOS) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, s.cfg)
}

func (s *vrVEOS) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}

func (s *vrVEOS) WithMgmtNet(mgmt *types.MgmtNet) { s.mgmt = mgmt }

func (s *vrVEOS) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
	err := utils.SaveCfgViaNetconf(s.cfg.LongName,
		nodes.DefaultCredentials[s.cfg.Kind][0],
		nodes.DefaultCredentials[s.cfg.Kind][0],
	)

	if err != nil {
		return err
	}

	log.Infof("saved %s running configuration to startup configuration file\n", s.cfg.ShortName)
	return nil
}
