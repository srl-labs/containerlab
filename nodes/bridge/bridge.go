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
	return nil
}
func (s *bridge) Config() *types.NodeConfig                              { return s.cfg }
func (s *bridge) PreDeploy(configName, labCADir, labCARoot string) error { return nil }
func (s *bridge) Deploy(ctx context.Context) error                       { return nil }
func (s *bridge) PostDeploy(ctx context.Context, ns map[string]nodes.Node) error {
	return nil
}
func (s *bridge) WithMgmtNet(*types.MgmtNet) {}
func (s *bridge) WithRuntime(globalRuntime string, allRuntimes map[string]runtime.ContainerRuntime) {
	s.runtime = allRuntimes[globalRuntime]
}
func (s *bridge) GetRuntime() runtime.ContainerRuntime { return s.runtime }

func (s *bridge) GetContainer(ctx context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (s *bridge) SaveConfig(ctx context.Context) error { return nil }

func (s *bridge) GetImages() map[string]string { return map[string]string{} }

func (s *bridge) Delete(ctx context.Context) error {
	return nil
}
