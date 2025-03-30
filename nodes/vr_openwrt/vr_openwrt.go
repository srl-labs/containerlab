// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause
package vr_openwrt

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"openwrt"}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() nodes.Node {
		return new(vrOpenWrt)
	}, nrea)
}

type vrOpenWrt struct {
	nodes.DefaultNode
}

func (n *vrOpenWrt) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// Add a simple bind-mount for the 'overlay' directory
	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "overlay"), ":/overlay"),
	)

	return nil
}

func (n *vrOpenWrt) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	// Ensure the overlay directory exists
	utils.CreateDirectory(filepath.Join(n.Cfg.LabDir, "overlay"), 0777)
	return nil
}
