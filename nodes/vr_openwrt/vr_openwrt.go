// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause
package vr_openwrt

import (
	"context"
	"fmt"
	"path/filepath"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"openwrt"}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrOpenWrt)
	}, nrea)
}

type vrOpenWrt struct {
	clabnodes.DefaultNode
}

func (n *vrOpenWrt) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

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

func (n *vrOpenWrt) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	// Ensure the overlay directory exists
	clabutils.CreateDirectory(filepath.Join(n.Cfg.LabDir, "overlay"),
		clabconstants.PermissionsOpen)
	return nil
}
