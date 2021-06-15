package vr_vmx

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "vr-vmx"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(vrVMX)
	})
}

type vrVMX struct{}

func (s *vrVMX) Init(cfg *types.NodeConfig) error {
	return nil
}
func (s *vrVMX) GenerateConfig() error { return nil }

func (s *vrVMX) Config() *types.NodeConfig { return nil }

func (s *vrVMX) PreDeploy() error { return nil }

func (s *vrVMX) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *vrVMX) PostDeploy() error                                            { return nil }
func (s *vrVMX) Destroy() error                                               { return nil }
