// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ext_container

import (
	"context"

	"github.com/pkg/errors"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"ext-container"}

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(extcont)
	})
}

type extcont struct {
	nodes.DefaultNode
}

func (s *extcont) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}
func (e *extcont) Deploy(ctx context.Context) error {
	// check for the external dependency to be running
	err := runtime.TimeoutWaitForContainerRunning(ctx, e.Runtime, e.Cfg.ShortName, e.Cfg.ShortName)
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
