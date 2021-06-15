package vr_sros

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-sros"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrSROS)
	})
}

type vrSROS struct{}

func (s *vrSROS) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrSROS) GenerateConfig() error { return nil }

func (s *vrSROS) Config() *types.NodeConfig { return nil }

func (s *vrSROS) PreDeploy() error { return nil }

func (s *vrSROS) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrSROS) PostDeploy() error                                            { return nil }
func (s *vrSROS) Destroy() error                                               { return nil }
