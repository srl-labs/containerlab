// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/weaveworks/ignite/pkg/operations"
)

var kindnames = []string{"linux"}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
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
	if cfg.Sysctls != nil && n.Config().NetworkMode != "host" {
		cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}

	return nil
}

func (n *linux) Deploy(ctx context.Context, _ *nodes.DeployParams) error {
	// Set the "CLAB_INTFS" variable to the number of interfaces
	// Which is required by vrnetlab to determine if all configured interfaces are present
	// such that the internal VM can be started with these interfaces assigned.
	n.Config().Env[types.CLAB_ENV_INTFS] = strconv.Itoa(len(n.GetEndpoints()))

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

func (n *linux) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", n.Cfg.ShortName)

	err := n.ExecFunction(ctx, utils.NSEthtoolTXOff(n.GetShortName(), "eth0"))
	if err != nil {
		log.Error(err)
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
