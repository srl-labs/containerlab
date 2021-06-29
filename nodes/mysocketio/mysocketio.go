package mysocketio

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

func init() {
	nodes.Register(nodes.NodeKindMySocketIO, func() nodes.Node {
		return new(mySocketIO)
	})
}

type mySocketIO struct{ cfg *types.NodeConfig }

func (s *mySocketIO) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	s.cfg.Entrypoint = "/bin/bash"
	return nil
}

func (s *mySocketIO) Config() *types.NodeConfig { return s.cfg }

func (s *mySocketIO) PreDeploy(configName, labCADir, labCARoot string) error {
	// utils.CreateDirectory(s.cfg.LabDir, 0777)
	return nil
}
func (s *mySocketIO) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }

func (s *mySocketIO) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for mysocketio '%s' node", s.cfg.ShortName)
	err := types.DisableTxOffload(s.cfg)
	if err != nil {
		return fmt.Errorf("failed to disable tx checksum offload for mysocketio kind: %v", err)
	}

	log.Infof("Creating mysocketio tunnels...")
	err = createMysocketTunnels(ctx, r, s.cfg, ns)
	return err
}

func (s *mySocketIO) WithMgmtNet(*types.MgmtNet) {}

func (s *mySocketIO) SaveConfig(ctx context.Context, r runtime.ContainerRuntime) error {
	return nil
}

///
