// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ext_container

import (
	"context"

	"github.com/pkg/errors"

	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"ext-container"}

// Register registers the node in the global Node map.
func Register() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(extcont)
	})
}

type extcont struct {
	nodes.DefaultNode
}

func (s *extcont) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.DefaultNode = *nodes.NewDefaultNode(s)
	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}

func (e *extcont) Deploy(ctx context.Context) error {
	// check for the external dependency to be running
	err := runtime.WaitForContainerRunning(ctx, e.Runtime, e.Cfg.ShortName, e.Cfg.ShortName)
	if err != nil {
		return err
	}

	// request nspath from runtime
	nspath, err := e.Runtime.GetNSPath(ctx, e.Cfg.ShortName)
	if err != nil {
		return errors.Wrap(err, "reading external container namespace path")
	}
	// set nspath in node config
	e.Cfg.NSPath = nspath
	// reflect ste nodes status as created
	e.Cfg.DeploymentStatus = "created"
	return nil
}

// Delete we will not mess with external containers on delete.
func (*extcont) Delete(_ context.Context) error { return nil }

// GetImages don't matter for external containers.
func (*extcont) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*extcont) PullImage(_ context.Context) error             { return nil }

// GetRuntimeContainerName will return the name used by the runtime to identify the container
// e.g. ext-cotnainer will use Cfg.ShortName while most other containers will use
// Cfg.LongName
func (e *extcont) GetRuntimeContainerName() string {
	return e.Cfg.ShortName
}

func (e *extcont) GetContainers(ctx context.Context) ([]types.GenericContainer, error) {
	cnts, err := e.DefaultNode.GetContainers(ctx)
	if err != nil {
		return nil, err
	}

	// we need to artifically add the Node Kind Label
	// this label data is e.g. used in the table printed after deployment
	for _, c := range cnts {
		c.Labels[labels.NodeKindLabel] = kindnames[0]
	}
	return cnts, nil
}
