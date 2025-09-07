// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntimeignite "github.com/srl-labs/containerlab/runtime/ignite"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/weaveworks/ignite/pkg/operations"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

var kindnames = []string{"linux"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindnames, func() clabnodes.Node {
		return new(linux)
	}, nrea)
}

type linux struct {
	clabnodes.DefaultNode
	vmChans *operations.VMChannels
}

func (n *linux) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// linux kind uses `always` as a default restart policy
	// since often they run auxiliary services that might fail because
	// of the wrong configuration or other reasons.
	// Usually we want those services to automatically restart.
	if n.Cfg.RestartPolicy == "" {
		n.Cfg.RestartPolicy = "always"
	}

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

func (n *linux) Deploy(ctx context.Context, _ *clabnodes.DeployParams) error {
	// Set the "CLAB_INTFS" variable to the number of interfaces
	// Which is required by vrnetlab to determine if all configured interfaces are present
	// such that the internal VM can be started with these interfaces assigned.
	n.Config().Env[clabconstants.ClabEnvIntfs] = strconv.Itoa(len(n.GetEndpoints()))

	cID, err := n.Runtime.CreateContainer(ctx, n.Cfg)
	if err != nil {
		return err
	}
	intf, err := n.Runtime.StartContainer(ctx, cID, n)

	if vmChans, ok := intf.(*operations.VMChannels); ok {
		n.vmChans = vmChans
	}

	n.SetState(clabnodesstate.Deployed)

	return err
}

func (n *linux) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for Linux '%s' node", n.Cfg.ShortName)

	err := n.ExecFunction(ctx, clabutils.NSEthtoolTXOff(n.GetShortName(), "eth0"))
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
	images[clabnodes.ImageKey] = n.Cfg.Image

	// ignite runtime additionally needs a kernel and sandbox image
	if n.Runtime.GetName() != clabruntimeignite.RuntimeName {
		return images
	}
	images[clabnodes.KernelKey] = n.Cfg.Kernel
	images[clabnodes.SandboxKey] = n.Cfg.Sandbox
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
