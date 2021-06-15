package vr_csr

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-csr"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrCSR)
	})
}

type vrCSR struct{}

func (s *vrCSR) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrCSR) GenerateConfig() error { return nil }

func (s *vrCSR) Config() *types.NodeConfig { return nil }

func (s *vrCSR) PreDeploy() error { return nil }

func (s *vrCSR) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrCSR) PostDeploy() error                                            { return nil }
func (s *vrCSR) Destroy() error                                               { return nil }
