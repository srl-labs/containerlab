// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func (c *CLab) initVrXRV9kNode(nodeCfg *types.NodeConfig) error {
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           "clab",
		"PASSWORD":           "clab@123",
		"CONNECTION_MODE":    vrDefConnMode,
		"VCPU":               "2",
		"RAM":                "12288",
		"DOCKER_NET_V4_ADDR": c.Config.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": c.Config.Mgmt.IPv6Subnet,
	}
	nodeCfg.Env = utils.MergeStringMaps(defEnv, nodeCfg.Env)

	if nodeCfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		nodeCfg.Binds = append(nodeCfg.Binds, "/dev:/dev")
	}

	nodeCfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --vcpu %s --ram %s --trace",
		nodeCfg.Env["USERNAME"], nodeCfg.Env["PASSWORD"], nodeCfg.ShortName, nodeCfg.Env["CONNECTION_MODE"], nodeCfg.Env["VCPU"], nodeCfg.Env["RAM"])

	return nil
}
