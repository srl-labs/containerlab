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
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *ovs) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}

func (s *ovs) Config() *types.NodeConfig { return s.cfg }

func (*ovs) PreDeploy(_, _, _ string) error { return nil }

func (*ovs) Deploy(_ context.Context) error { return nil }

func (*ovs) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return nil
}

func (*ovs) WithMgmtNet(*types.MgmtNet)               {}
func (s *ovs) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *ovs) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (*ovs) GetContainer(_ context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (*ovs) Delete(_ context.Context) error {
	return nil
}

func (*ovs) GetImages() map[string]string { return map[string]string{} }

func (*ovs) SaveConfig(_ context.Context) error {
	return nil
}
