// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func initVrCSRNode(c *CLab, nodeDef *types.NodeDefinition, nodeCfg *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	nodeCfg.Image = c.Config.Topology.GetNodeImage(nodeCfg.ShortName)
	nodeCfg.Group = c.Config.Topology.GetNodeGroup(nodeCfg.ShortName)
	nodeCfg.Position = c.Config.Topology.GetNodePosition(nodeCfg.ShortName)
	nodeCfg.User = user

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    vrDefConnMode,
		"USERNAME":           "admin",
		"PASSWORD":           "admin",
		"DOCKER_NET_V4_ADDR": c.Config.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": c.Config.Mgmt.IPv6Subnet,
	}
	nodeCfg.Env = utils.MergeStringMaps(defEnv, envs)

	if nodeCfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		nodeCfg.Binds = append(nodeCfg.Binds, "/dev:/dev")
	}

	nodeCfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace", nodeCfg.Env["USERNAME"], nodeCfg.Env["PASSWORD"], nodeCfg.ShortName, nodeCfg.Env["CONNECTION_MODE"])

	return err
}
