// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package checkpoint_cloudguard

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"checkpoint_cloudguard"}

const (
	scrapliPlatformName = "checkpoint_cloudguard"
	defaultUser         = "admin"
	defaultPassword     = "admin"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(CheckpointCloudguard)
	})
	err := nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
	if err != nil {
		log.Error(err)
	}
}

type CheckpointCloudguard struct {
	nodes.DefaultNode
}

func (n *CheckpointCloudguard) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set startup arguments in boxen container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultUser,
		"PASSWORD":           defaultPassword,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	// set virtualization requirement
	n.Cfg.HostRequirements.VirtRequired = true

	return nil
}

func (n *CheckpointCloudguard) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	return nil
}
