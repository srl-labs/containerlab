package ceos

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "ceos"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(ceos)
	})
}

type ceos struct{}

func (s *ceos) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *ceos) GenerateConfig() error { return nil }

func (s *ceos) Config() *types.NodeConfig { return nil }

func (s *ceos) PreDeploy() error { return nil }

func (s *ceos) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *ceos) PostDeploy() error                                            { return nil }
func (s *ceos) Destroy() error                                               { return nil }
