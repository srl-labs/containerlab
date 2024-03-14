// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sonic

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"sonic-vs"}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(sonic)
	}, nil)
}

type sonic struct {
	nodes.DefaultNode
}

func (s *sonic) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// the entrypoint is reset to prevent it from starting before all interfaces are connected
	// all main sonic agents are started in a post-deploy phase
	s.Cfg.Entrypoint = "/bin/bash"
	return nil
}

func (s *sonic) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return nil
}

func (s *sonic) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for sonic-vs '%s' node", s.Cfg.ShortName)

	cmd, _ := exec.NewExecCmdFromString("supervisord")
	err := s.RunExecNotWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.Cfg.ShortName, err)
	}

	cmd, _ = exec.NewExecCmdFromString("supervisorctl start bgpd")
	err = s.RunExecNotWait(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed post-deploy node %q: %w", s.Cfg.ShortName, err)
	}

	return nil
}
