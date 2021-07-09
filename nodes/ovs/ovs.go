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

func (l *ovs) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}
	return nil
}

func (l *ovs) Config() *types.NodeConfig { return l.cfg }

func (l *ovs) PreDeploy(configName, labCADir, labCARoot string) error { return nil }

func (l *ovs) Deploy(ctx context.Context) error { return nil }

func (l *ovs) PostDeploy(ctx context.Context, ns map[string]nodes.Node) error {
	return nil
}

func (l *ovs) WithMgmtNet(*types.MgmtNet)             {}
func (s *ovs) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *ovs) GetRuntime() runtime.ContainerRuntime   { return s.runtime }
func (s *ovs) MaxWorkers() uint                       { return s.cfg.MaxWorkers }

func (s *ovs) GetContainer(ctx context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (s *ovs) Delete(ctx context.Context) error {
	return nil
}

func (s *ovs) GetImages() map[string]string { return map[string]string{} }

func (s *ovs) SaveConfig(ctx context.Context) error {
	return nil
}
