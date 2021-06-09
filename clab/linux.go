// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"strings"

	"github.com/srl-labs/containerlab/types"
)

func initLinuxNode(c *CLab, nodeCfg NodeConfig, node *types.Node, user string, envs map[string]string) error {
	var err error

	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.Cmd = c.cmdInit(&nodeCfg, node.Kind)
	node.User = user

	node.Sysctls = make(map[string]string)
	if strings.ToLower(node.NetworkMode) != "host" {
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	node.Env = envs

	return err
}
