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

func init() {
	nodes.Register(nodes.NodeKindLinux, func() nodes.Node {
		return new(linux)
	})
}

type linux struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (l *linux) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}

	// make ipv6 enabled on all linux node interfaces
	// but not for the nodes with host network mode, as this is not supported on gh action runners
	if l.Config().NetworkMode != "host" {
		cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	return nil
}

func (l *linux) Config() *types.NodeConfig { return l.cfg }

func (*linux) PreDeploy(_, _, _ string) error { return nil }

func (l *linux) Deploy(ctx context.Context) error {
	_, err := l.runtime.CreateContainer(ctx, l.cfg)
	return err
}

func (l *linux) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", l.cfg.ShortName)
	return types.DisableTxOffload(l.cfg)
}

func (l *linux) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image
	return images
}

func (*linux) WithMgmtNet(*types.MgmtNet)               {}
func (l *linux) WithRuntime(r runtime.ContainerRuntime) { l.runtime = r }
func (l *linux) GetRuntime() runtime.ContainerRuntime   { return l.runtime }

func (l *linux) Delete(ctx context.Context) error {
	return l.runtime.DeleteContainer(ctx, l.Config().LongName)
}

func (*linux) SaveConfig(_ context.Context) error {
	return nil
}
