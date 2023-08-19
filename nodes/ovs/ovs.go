// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ovs

import (
	"context"
	"fmt"

	goOvs "github.com/digitalocean/go-openvswitch/ovs"
	log "github.com/sirupsen/logrus"
	cExec "github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"ovs-bridge"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(ovs)
	}, nil)
}

type ovs struct {
	nodes.DefaultNode
}

func (n *ovs) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.IsRootNamespaceBased = true

	return nil
}

func (n *ovs) CheckDeploymentConditions(_ context.Context) error {
	// check if ovs bridge exists
	c := goOvs.New(
		// Prepend "sudo" to all commands.
		goOvs.Sudo(),
	)

	if _, err := c.VSwitch.Get.Bridge(n.Cfg.ShortName); err != nil {
		return fmt.Errorf("could not find ovs bridge %q", n.Cfg.ShortName)
	}

	return nil
}

func (*ovs) Deploy(_ context.Context, _ *nodes.DeployParams) error { return nil }
func (*ovs) PullImage(_ context.Context) error                     { return nil }
func (*ovs) GetImages(_ context.Context) map[string]string         { return map[string]string{} }
func (*ovs) Delete(_ context.Context) error                        { return nil }
func (*ovs) DeleteNetnsSymlink() (err error)                       { return nil }

// UpdateConfigWithRuntimeInfo is a noop for bridges.
func (*ovs) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers is a noop for bridges.
func (*ovs) GetContainers(_ context.Context) ([]runtime.GenericContainer, error) { return nil, nil }

// RunExec is noop for ovs kind.
func (n *ovs) RunExec(_ context.Context, _ *cExec.ExecCmd) (*cExec.ExecResult, error) {
	log.Warnf("Exec operation is not implemented for kind %q", n.Config().Kind)

	return nil, cExec.ErrRunExecNotSupported
}
