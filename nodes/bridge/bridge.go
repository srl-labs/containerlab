// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package bridge

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

func init() {
	nodes.Register(nodes.NodeKindBridge, func() nodes.Node {
		return new(bridge)
	})
}

type bridge struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *bridge) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	s.cfg.DeploymentStatus = "created" // since we do not create bridges with clab, the status is implied here
	return nil
}
func (s *bridge) Config() *types.NodeConfig    { return s.cfg }
func (*bridge) PreDeploy(_, _, _ string) error { return nil }
func (*bridge) Deploy(_ context.Context) error { return nil }
func (*bridge) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return nil
}
func (*bridge) WithMgmtNet(*types.MgmtNet)               {}
func (s *bridge) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *bridge) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (*bridge) GetContainer(_ context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (*bridge) SaveConfig(_ context.Context) error { return nil }

func (*bridge) GetImages() map[string]string { return map[string]string{} }

func (*bridge) Delete(_ context.Context) error {
	return nil
}
