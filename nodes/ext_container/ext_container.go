// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ext_container

import (
	"context"

	containerlablabels "github.com/srl-labs/containerlab/labels"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"ext-container"}

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	r.Register(kindnames, func() containerlabnodes.Node {
		return new(extcont)
	}, nil)
}

type extcont struct {
	containerlabnodes.DefaultNode
}

func (s *extcont) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	s.DefaultNode = *containerlabnodes.NewDefaultNode(s)
	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// Indicate that the pre-deployment UniquenessCheck is to be skipped.
	// Since we would stop deployment on pre-existing containers.
	s.Cfg.SkipUniquenessCheck = true
	s.Cfg.LongName = s.Cfg.ShortName
	return nil
}

func (e *extcont) Deploy(ctx context.Context, _ *containerlabnodes.DeployParams) error {
	// check for the external dependency to be running
	err := containerlabruntime.WaitForContainerRunning(ctx, e.Runtime, e.Cfg.ShortName, e.Cfg.ShortName)
	if err != nil {
		return err
	}

	e.SetState(containerlabnodesstate.Deployed)

	return nil
}

// Delete we will not mess with external containers on delete.
func (*extcont) Delete(_ context.Context) error { return nil }

// GetImages don't matter for external containers.
func (*extcont) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*extcont) PullImage(_ context.Context) error             { return nil }

// GetContainerName returns the short name for the ext-container node, since for these nodes
// container name is specified in the topology file entirely.
func (e *extcont) GetContainerName() string {
	return e.Cfg.ShortName
}

func (e *extcont) GetContainers(ctx context.Context) ([]containerlabruntime.GenericContainer, error) {
	containers, err := e.DefaultNode.GetContainers(ctx)
	if err != nil {
		return nil, err
	}

	// we need to artificially add the Node Kind Label
	// this label data is e.g. used in the table printed after deployment
	for idx := range containers {
		containers[idx].Labels[containerlablabels.NodeKind] = kindnames[0]
	}
	return containers, nil
}
