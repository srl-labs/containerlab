package vr_ros

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-ros"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrROS)
	})
}

type vrROS struct{}

func (s *vrROS) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrROS) GenerateConfig() error { return nil }

func (s *vrROS) Config() *types.NodeConfig { return nil }

func (s *vrROS) PreDeploy() error { return nil }

func (s *vrROS) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrROS) PostDeploy() error                                            { return nil }
func (s *vrROS) Destroy() error                                               { return nil }
