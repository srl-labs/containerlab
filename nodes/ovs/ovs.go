// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ovs

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

func init() {
	nodes.Register(nodes.NodeKindOVS, func() nodes.Node {
		return new(ovs)
	})
}

type ovs struct {
	cfg *types.NodeConfig
}

func (l *ovs) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}
	return nil
}

func (l *ovs) Config() *types.NodeConfig { return l.cfg }

func (l *ovs) PreDeploy(configName, labCADir, labCARoot string) error { return nil }

func (l *ovs) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }

func (l *ovs) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}

func (l *ovs) WithMgmtNet(*types.MgmtNet) {}

func (s *ovs) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
	return nil
}
