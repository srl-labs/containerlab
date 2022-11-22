// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_pan

import (
	"fmt"

	"github.com/cloudflare/cfssl/log"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vr-pan", "vr-paloalto_panos"}

const (
	defaultUser     = "admin"
	defaultPassword = "Admin@123"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(vrPan)
	})
	err := nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
	if err != nil {
		log.Error(err)
	}
}

type vrPan struct {
	nodes.DefaultNode
}

func (s *vrPan) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultUser,
		"PASSWORD":           defaultPassword,
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"VCPU":               "2",
		"RAM":                "6144",
		"DOCKER_NET_V4_ADDR": s.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.Mgmt.IPv6Subnet,
	}
	s.Cfg.Env = utils.MergeStringMaps(defEnv, s.Cfg.Env)

	if s.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.Cfg.Binds = append(s.Cfg.Binds, "/dev:/dev")
	}

	s.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		s.Cfg.Env["USERNAME"], s.Cfg.Env["PASSWORD"], s.Cfg.ShortName, s.Cfg.Env["CONNECTION_MODE"])

	// set virtualization requirement
	s.Cfg.HostRequirements.VirtRequired = true

	return nil
}

func (s *vrPan) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return nil
}
