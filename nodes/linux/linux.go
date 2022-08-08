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
	"github.com/weaveworks/ignite/pkg/operations"
)

var kindnames = []string{"linux"}

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(linux)
	})
}

type linux struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
	vmChans *operations.VMChannels
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
	cID, err := l.runtime.CreateContainer(ctx, l.cfg)
	if err != nil {
		return err
	}
	intf, err := l.runtime.StartContainer(ctx, cID, l.cfg)

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		l.vmChans = vmChans
	}

	return err
}

func (l *linux) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", l.cfg.ShortName)
	if err := types.DisableTxOffload(l.cfg); err != nil {
		return err
	}

	// when ignite runtime is in use
	if l.vmChans != nil {
		return <-l.vmChans.SpawnFinished
	}

	return nil
}

func (l *linux) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image

	// ignite runtime additionally needs a kernel and sandbox image
	if l.runtime.GetName() != runtime.IgniteRuntime {
		return images
	}
	images[nodes.KernelKey] = l.cfg.Kernel
	images[nodes.SandboxKey] = l.cfg.Sandbox
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
