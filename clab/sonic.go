// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import "github.com/srl-labs/containerlab/types"

func initSonicNode(c *CLab, nodeCfg NodeDefinition, node *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)
	node.User = user

	// rewrite entrypoint so sonic won't start supervisord before we attach veth interfaces
	node.Entrypoint = "/bin/bash"

	return err
}
