// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package rare

import (
	"context"
	"fmt"
	"path/filepath"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var kindNames = []string{"rare"}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	generateNodeAttributes := containerlabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() containerlabnodes.Node {
		return new(rare)
	}, nrea)
}

type rare struct {
	containerlabnodes.DefaultNode
}

func (n *rare) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *containerlabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// make ipv6 disabled on all rare node interfaces unconditionally
	// as ipv6 will be handled by rare/freertr
	// The setting 'net.ipv6.conf.all.disable_ipv6' 1 - interferes with IPv6 out-of-band management. Commenting it out for now as a workaround.
	// cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "1"

	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "run"), ":/rtr/run"),
	)

	return nil
}

func (n *rare) PreDeploy(_ context.Context, params *containerlabnodes.PreDeployParams) error {
	containerlabutils.CreateDirectory(n.Cfg.LabDir, 0o777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.createRAREFiles()
}

func (n *rare) createRAREFiles() error {
	nodeCfg := n.Config()
	// create "run" directory that will be bind mounted to rare node
	containerlabutils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "run"), 0o777)

	return nil
}
