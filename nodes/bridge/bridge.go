package bridge

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "bridge"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(bridge)
	})
}

type bridge struct{}

func (s *bridge) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *bridge) GenerateConfig() error { return nil }

func (s *bridge) Config() *types.NodeConfig { return nil }

func (s *bridge) PreDeploy() error { return nil }

func (s *bridge) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *bridge) PostDeploy() error                                            { return nil }
func (s *bridge) Destroy() error                                               { return nil }
