// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sonic

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func init() {
	nodes.Register(nodes.NodeKindSonic, func() nodes.Node {
		return new(sonic)
	})
}

type sonic struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *sonic) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// the entrypoint is reset to prevent it from starting before all interfaces are connected
	// all main sonic agents are started in a post-deploy phase
	s.cfg.Entrypoint = "/bin/bash"
	return nil
}
func (s *sonic) Config() *types.NodeConfig { return s.cfg }

func (s *sonic) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)

	return nil
}
func (s *sonic) Deploy(ctx context.Context) error {
	_, err := s.runtime.CreateContainer(ctx, s.cfg)
	return err
}

func (s *sonic) PostDeploy(ctx context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for sonic-vs '%s' node", s.cfg.ShortName)

	err := s.runtime.ExecNotWait(ctx, s.cfg.ContainerID, []string{"supervisord"})
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.cfg.ShortName, err)
	}

	err = s.runtime.ExecNotWait(ctx, s.cfg.ContainerID, []string{"supervisorctl start bgpd"})
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.cfg.ShortName, err)
	}

	return nil
}

func (*sonic) WithMgmtNet(*types.MgmtNet)               {}
func (s *sonic) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *sonic) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (s *sonic) Delete(ctx context.Context) error {
	return s.runtime.DeleteContainer(ctx, s.Config().LongName)
}

func (s *sonic) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (*sonic) SaveConfig(_ context.Context) error {
	return nil
}
