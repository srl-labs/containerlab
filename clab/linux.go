// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"strings"

	"github.com/srl-labs/containerlab/types"
)

func (c *CLab) initLinuxNode(nodeCfg *types.NodeConfig) error {
	var err error

	nodeCfg.Config, err = c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}

	nodeCfg.Sysctls = make(map[string]string)
	if strings.ToLower(nodeCfg.NetworkMode) != "host" {
		nodeCfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	return nil
}
