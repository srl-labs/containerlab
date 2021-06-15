package crpd

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "crpd"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(crpd)
	})
}

type crpd struct{}

func (s *crpd) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *crpd) GenerateConfig() error { return nil }

func (s *crpd) Config() *types.NodeConfig { return nil }

func (s *crpd) PreDeploy() error { return nil }

func (s *crpd) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *crpd) PostDeploy() error                                            { return nil }
func (s *crpd) Destroy() error                                               { return nil }
