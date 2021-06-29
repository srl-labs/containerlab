// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_xrv9k

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
	nodes.Register(nodes.NodeKindVrXRV9K, func() nodes.Node {
		return new(vrXRV9K)
	})
}

type vrXRV9K struct {
	cfg  *types.NodeConfig
	mgmt *types.MgmtNet
}

func (s *vrXRV9K) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           "clab",
		"PASSWORD":           "clab@123",
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"VCPU":               "2",
		"RAM":                "12288",
		"DOCKER_NET_V4_ADDR": s.mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.mgmt.IPv6Subnet,
	}
	s.cfg.Env = utils.MergeStringMaps(defEnv, s.cfg.Env)

	if s.cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.cfg.Binds = append(s.cfg.Binds, "/dev:/dev")
	}

	s.cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --vcpu %s --ram %s --trace",
		s.cfg.Env["USERNAME"], s.cfg.Env["PASSWORD"], s.cfg.ShortName, s.cfg.Env["CONNECTION_MODE"], s.cfg.Env["VCPU"], s.cfg.Env["RAM"])

	return nil
}

func (s *vrXRV9K) Config() *types.NodeConfig { return s.cfg }

func (s *vrXRV9K) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return nil
}

func (s *vrXRV9K) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, s.cfg)
}

func (s *vrXRV9K) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}

func (s *vrXRV9K) WithMgmtNet(mgmt *types.MgmtNet) { s.mgmt = mgmt }

func (s *vrXRV9K) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
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
