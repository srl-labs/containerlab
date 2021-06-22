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

const (
	nodeKind = "sonic-vs"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(sonic)
	})
}

type sonic struct{ cfg *types.NodeConfig }

func (s *sonic) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	s.cfg.Entrypoint = "/bin/bash"
	return nil
}
func (s *sonic) Config() *types.NodeConfig { return s.cfg }
func (s *sonic) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)

	return nil
}
func (s *sonic) Deploy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *sonic) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for sonic-vs '%s' node", s.cfg.ShortName)
	// TODO: change this calls to c.ExecNotWait
	// exec `supervisord` to start sonic services
	_, stderr, err := r.Exec(ctx, s.cfg.ContainerID, []string{"supervisord"})
	if err != nil {
		return err
	}
	if len(stderr) > 0 {
		return fmt.Errorf("failed post-deploy node %q: %s", s.cfg.ShortName, string(stderr))
	}
	_, stderr, err = r.Exec(ctx, s.cfg.ContainerID, []string{"/usr/lib/frr/bgpd"})
	if err != nil {
		return err
	}
	if len(stderr) > 0 {
		return fmt.Errorf("failed post-deploy node %q: %s", s.cfg.ShortName, string(stderr))
	}
	return nil
}
func (s *sonic) Destroy(ctx context.Context, r runtime.ContainerRuntime) error { return nil }
func (s *sonic) WithMgmtNet(*types.MgmtNet)                                    {}
