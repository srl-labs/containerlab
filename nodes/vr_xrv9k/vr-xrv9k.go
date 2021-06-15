package vr_xrv9k

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-xrv9k"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrXRV9k)
	})
}

type vrXRV9k struct{}

func (s *vrXRV9k) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrXRV9k) GenerateConfig() error { return nil }

func (s *vrXRV9k) Config() *types.NodeConfig { return nil }

func (s *vrXRV9k) PreDeploy() error { return nil }

func (s *vrXRV9k) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrXRV9k) PostDeploy() error                                            { return nil }
func (s *vrXRV9k) Destroy() error                                               { return nil }
