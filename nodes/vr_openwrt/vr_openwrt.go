// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause
package vr_openwrt

import (
	"context"
	"fmt"
	"path/filepath"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"openwrt"}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	generateNodeAttributes := containerlabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() containerlabnodes.Node {
		return new(vrOpenWrt)
	}, nrea)
}

type vrOpenWrt struct {
	containerlabnodes.DefaultNode
}

func (n *vrOpenWrt) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *containerlabnodes.NewDefaultNode(n)

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

func (n *vrOpenWrt) PreDeploy(_ context.Context, params *containerlabnodes.PreDeployParams) error {
	// Ensure the overlay directory exists
	containerlabutils.CreateDirectory(filepath.Join(n.Cfg.LabDir, "overlay"), 0o777)
	return nil
}
