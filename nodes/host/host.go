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

type host struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *host) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}

	return nil
}
func (s *host) Config() *types.NodeConfig { return s.cfg }
func (*host) PreDeploy(_, _, _ string) error {
	return nil
}
func (*host) Deploy(_ context.Context) error { return nil }
func (*host) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return nil
}

func (*host) GetImages() map[string]string { return map[string]string{} }

func (*host) WithMgmtNet(*types.MgmtNet)               {}
func (s *host) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *host) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (*host) GetContainer(_ context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (*host) Delete(_ context.Context) error {
	return nil
}

func (*host) SaveConfig(_ context.Context) error {
	return nil
}
