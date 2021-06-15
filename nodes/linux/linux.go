package linux

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "linux"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(linux)
	})
}

type linux struct{}

func (s *linux) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *linux) GenerateConfig() error { return nil }

func (s *linux) Config() *types.NodeConfig { return nil }

func (s *linux) PreDeploy() error { return nil }

func (s *linux) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *linux) PostDeploy() error                                            { return nil }
func (s *linux) Destroy() error                                               { return nil }
