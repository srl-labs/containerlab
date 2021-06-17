// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"strings"

	"github.com/srl-labs/containerlab/types"
)

func initLinuxNode(c *CLab, nodeDef *types.NodeDefinition, nodeCfg *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	// nodeCfg.Config, err = c.configInit(nodeDef, nodeCfg.Kind)
	c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	// nodeCfg.Image = c.imageInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Image = c.Config.Topology.GetNodeImage(nodeCfg.ShortName)
	// nodeCfg.Group = c.groupInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Group = c.Config.Topology.GetNodeGroup(nodeCfg.ShortName)
	// nodeCfg.Position = c.positionInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Position = c.Config.Topology.GetNodePosition(nodeCfg.ShortName)
	// nodeCfg.Cmd = c.cmdInit(nodeDef, nodeCfg.Kind)
	nodeCfg.Cmd = c.Config.Topology.GetNodeCmd(nodeCfg.ShortName)
	nodeCfg.User = user

	nodeCfg.Sysctls = make(map[string]string)
	if strings.ToLower(nodeCfg.NetworkMode) != "host" {
		nodeCfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	nodeCfg.Env = envs

	return err
}
