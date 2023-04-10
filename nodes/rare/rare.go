// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package rare

import (
	"context"
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/weaveworks/ignite/pkg/operations"
)

var kindnames = []string{"rare"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(rare)
	}, nil)
}

type rare struct {
	nodes.DefaultNode
	vmChans *operations.VMChannels
}

func (n *rare) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// make ipv6 disabled on all rare node interfaces unconditionally
	// as ipv6 will be handled by rare/freertr
	cfg.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "1"

	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(filepath.Join(n.Cfg.LabDir, "run"), ":/rtr/run"),
	)

	return nil
}

func (n *rare) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return createRAREFiles(n)
}

func (n *rare) Deploy(ctx context.Context, _ *nodes.DeployParams) error {
	cID, err := n.Runtime.CreateContainer(ctx, n.Cfg)
	if err != nil {
		return err
	}
	intf, err := n.Runtime.StartContainer(ctx, cID, n.Cfg)

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		n.vmChans = vmChans
	}

	return err
}

func (n *rare) PostDeploy(_ context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for RARE/freeRtr '%s' node", n.Cfg.ShortName)

	if err := types.DisableTxOffload(n.Cfg); err != nil {
		return err
	}

	// when ignite runtime is in use
	if n.vmChans != nil {
		return <-n.vmChans.SpawnFinished
	}

	return nil
}

func (n *rare) GetImages(_ context.Context) map[string]string {
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

func createRAREFiles(node nodes.Node) error {
	nodeCfg := node.Config()
	// create "run" directory that will be bind mounted to rare node
	utils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "run"), 0777)
	return nil
}

// CheckInterfaceName is a noop for rare containers as they can have any names.
func (n *rare) CheckInterfaceName() error {
	return nil
}
