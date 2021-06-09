// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/types"
)

func initVrXRVNode(c *CLab, nodeCfg NodeConfig, node *types.Node, user string, envs map[string]string) error {
	var err error

	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.User = user

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           "clab",
		"PASSWORD":           "clab@123",
		"CONNECTION_MODE":    vrDefConnMode,
		"DOCKER_NET_V4_ADDR": c.Config.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": c.Config.Mgmt.IPv6Subnet,
	}
	node.Env = mergeStringMaps(defEnv, envs)

	if node.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		node.Binds = append(node.Binds, "/dev:/dev")
	}

	node.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace", node.Env["USERNAME"], node.Env["PASSWORD"], node.ShortName, node.Env["CONNECTION_MODE"])

	return err
}
