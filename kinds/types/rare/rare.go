// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package rare

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"rare"}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(rare)
	}, nil)
}

type rare struct {
	nodes.DefaultNode
}

func (n *rare) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// make ipv6 disabled on all rare node interfaces unconditionally
	// as ipv6 will be handled by rare/freertr
	cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "1"

	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "run"), ":/rtr/run"),
	)

	return nil
}

func (n *rare) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	return n.createRAREFiles()
}

func (n *rare) createRAREFiles() error {
	nodeCfg := n.Config()
	// create "run" directory that will be bind mounted to rare node
	utils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "run"), 0777)

	return nil
}
