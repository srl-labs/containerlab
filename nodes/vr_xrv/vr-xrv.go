package vr_xrv

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-xrv"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrXRV)
	})
}

type vrXRV struct{}

func (s *vrXRV) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrXRV) GenerateConfig() error { return nil }

func (s *vrXRV) Config() *types.NodeConfig { return nil }

func (s *vrXRV) PreDeploy() error { return nil }

func (s *vrXRV) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrXRV) PostDeploy() error                                            { return nil }
func (s *vrXRV) Destroy() error                                               { return nil }
