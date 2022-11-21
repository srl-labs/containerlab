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
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *extcont) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}
func (e *extcont) Deploy(ctx context.Context) error {
	// check for the external dependency to be running
	err := runtime.TimeoutWaitForContainerRunning(ctx, e.runtime, e.cfg.ShortName, e.cfg.ShortName)
	if err != nil {
		return err
	}

	// request nspath from runtime
	nspath, err := e.runtime.GetNSPath(ctx, e.cfg.ShortName)
	if err != nil {
		return errors.Wrap(err, "reading external container namespace path")
	}
	// set nspath in node config
	e.cfg.NSPath = nspath
	// reflect ste nodes status as created
	e.cfg.DeploymentStatus = "created"
	return nil
}
func (e *extcont) Config() *types.NodeConfig                                   { return e.cfg }
func (e *extcont) PreDeploy(_, _, _ string) error                              { return nil }
func (e *extcont) PostDeploy(_ context.Context, _ map[string]nodes.Node) error { return nil }
func (e *extcont) WithMgmtNet(*types.MgmtNet)                                  {}
func (e *extcont) WithRuntime(r runtime.ContainerRuntime)                      { e.runtime = r }
func (e *extcont) GetRuntime() runtime.ContainerRuntime                        { return e.runtime }
func (e *extcont) SaveConfig(_ context.Context) error                          { return nil }
func (e *extcont) GetImages() map[string]string                                { return map[string]string{} }
func (e *extcont) Delete(_ context.Context) error                              { return nil }
