package ovs

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "ovs"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(ovs)
	})
}

type ovs struct{}

func (s *ovs) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *ovs) GenerateConfig() error { return nil }

func (s *ovs) Config() *types.NodeConfig { return nil }

func (s *ovs) PreDeploy() error { return nil }

func (s *ovs) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *ovs) PostDeploy() error                                            { return nil }
func (s *ovs) Destroy() error                                               { return nil }
