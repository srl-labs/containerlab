// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime/ignite"
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
	nodes.DefaultNode
	vmChans *operations.VMChannels
}

func (l *linux) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	l.DefaultNode = *nodes.NewDefaultNode(l)

	l.Cfg = cfg
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

func (l *linux) Deploy(ctx context.Context) error {
	cID, err := l.Runtime.CreateContainer(ctx, l.Cfg)
	if err != nil {
		return err
	}
	intf, err := l.Runtime.StartContainer(ctx, cID, l.Cfg)

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		l.vmChans = vmChans
	}

	return err
}

func (l *linux) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", l.Cfg.ShortName)
	if err := types.DisableTxOffload(l.Cfg); err != nil {
		return err
	}

	// when ignite runtime is in use
	if l.vmChans != nil {
		return <-l.vmChans.SpawnFinished
	}

	return nil
}

func (l *linux) GetImages(_ context.Context) map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.Cfg.Image

	// ignite runtime additionally needs a kernel and sandbox image
	if l.Runtime.GetName() != ignite.RuntimeName {
		return images
	}
	images[nodes.KernelKey] = l.Cfg.Kernel
	images[nodes.SandboxKey] = l.Cfg.Sandbox
	return images
}
