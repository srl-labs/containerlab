// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import "github.com/srl-labs/containerlab/types"

func (c *CLab) initSonicNode(nodeCfg *types.NodeConfig) error {
	var err error

	nodeCfg.Config, err = c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}

	// rewrite entrypoint so sonic won't start supervisord before we attach veth interfaces
	nodeCfg.Entrypoint = "/bin/bash"

	return nil
}
