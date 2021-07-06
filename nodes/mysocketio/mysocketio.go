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

type mySocketIO struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

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
func (s *mySocketIO) Deploy(ctx context.Context) error {
	_, err := s.runtime.CreateContainer(ctx, s.cfg)
	return err
}
func (s *mySocketIO) PostDeploy(ctx context.Context, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for mysocketio '%s' node", s.cfg.ShortName)
	err := types.DisableTxOffload(s.cfg)
	if err != nil {
		return fmt.Errorf("failed to disable tx checksum offload for mysocketio kind: %v", err)
	}

	log.Infof("Creating mysocketio tunnels...")
	err = createMysocketTunnels(ctx, s.runtime, s.cfg, ns)
	return err
}

func (s *mySocketIO) WithMgmtNet(*types.MgmtNet) {}
func (s *mySocketIO) WithRuntime(globalRuntime string, allRuntimes map[string]runtime.ContainerRuntime) {
	s.runtime = allRuntimes[globalRuntime]
}
func (s *mySocketIO) GetRuntime() runtime.ContainerRuntime { return s.runtime }

func (s *mySocketIO) GetContainer(ctx context.Context) (*types.GenericContainer, error) {
	return nil, nil
}

func (s *mySocketIO) Delete(ctx context.Context) error {
	return nil
}

func (s *mySocketIO) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (s *mySocketIO) SaveConfig(ctx context.Context) error {
	return nil
}

///
