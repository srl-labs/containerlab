package vr_veos

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-veos"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrVEOS)
	})
}

type vrVEOS struct{}

func (s *vrVEOS) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrVEOS) GenerateConfig() error { return nil }

func (s *vrVEOS) Config() *types.NodeConfig { return nil }

func (s *vrVEOS) PreDeploy() error { return nil }

func (s *vrVEOS) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrVEOS) PostDeploy() error                                            { return nil }
func (s *vrVEOS) Destroy() error                                               { return nil }
