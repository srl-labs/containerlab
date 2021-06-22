// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	nodeKind = "linux"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(linux)
	})
}

type linux struct{ cfg *types.NodeConfig }

func (l *linux) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}
	return nil
}

func (l *linux) Config() *types.NodeConfig { return l.cfg }

func (l *linux) PreDeploy(configName, labCADir, labCARoot string) error { return nil }

func (l *linux) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, l.cfg)
}

func (l *linux) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", l.cfg.ShortName)
	return nil
}

func (l *linux) WithMgmtNet(*types.MgmtNet) {}
