// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/weaveworks/ignite/pkg/operations"
)

var kindnames = []string{"linux"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(linux)
	}, nil)
}

type linux struct {
	nodes.DefaultNode
	vmChans *operations.VMChannels
}

func (n *linux) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// make ipv6 enabled on all linux node interfaces
	// but not for the nodes with host network mode, as this is not supported on gh action runners
	if n.Config().NetworkMode != "host" {
		cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	return nil
}

func (n *linux) Deploy(ctx context.Context, _ *nodes.DeployParams) error {
	cID, err := n.Runtime.CreateContainer(ctx, n.Cfg)
	if err != nil {
		return err
	}
	intf, err := n.Runtime.StartContainer(ctx, cID, n)

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		n.vmChans = vmChans
	}

	n.SetState(state.Deployed)

	return err
}

func (n *linux) PostDeploy(_ context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", n.Cfg.ShortName)
	if err := types.DisableTxOffload(n.Cfg); err != nil {
		return err
	}

	// when ignite runtime is in use
	if n.vmChans != nil {
		return <-n.vmChans.SpawnFinished
	}

	return nil
}

func (n *linux) GetImages(_ context.Context) map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = n.Cfg.Image

	// ignite runtime additionally needs a kernel and sandbox image
	if n.Runtime.GetName() != ignite.RuntimeName {
		return images
	}
	images[nodes.KernelKey] = n.Cfg.Kernel
	images[nodes.SandboxKey] = n.Cfg.Sandbox
	return images
}

// CheckInterfaceName allows any interface name for linux nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *linux) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node when network mode is not set to none", n.Cfg.ShortName)
		}
	}
	return nil
}
