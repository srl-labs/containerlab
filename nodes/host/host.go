// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

func init() {
	nodes.Register(nodes.NodeKindHOST, func() nodes.Node {
		return new(host)
	})
}

type host struct{ cfg *types.NodeConfig }

func (s *host) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}
func (s *host) Config() *types.NodeConfig { return s.cfg }
func (s *host) PreDeploy(configName, labCADir, labCARoot string) error {
	return nil
}
func (s *host) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *host) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}

func (s *host) WithMgmtNet(*types.MgmtNet) {}
func (s *host) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
	return nil
}
