// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ext_container

import (
	"context"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
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
func (e *extcont) Delete(_ context.Context) error { return nil }

// GetImages don't matter for external containers.
func (e *extcont) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (e *extcont) PullImage(_ context.Context) error             { return nil }

// RunExecType is the final function that calls the runtime to execute a type.Exec on a container
// This is to be overriden if the nodes implementation differs
func (e *extcont) RunExecType(ctx context.Context, exec types.ExecCmd) (types.ExecResultHolder, error) {
	execResult, err := e.GetRuntime().Exec(ctx, e.Cfg.ShortName, exec)
	if err != nil {
		// On Ext-container we have to use the shortname, whilst default is to use longname
		log.Errorf("%s: failed to execute cmd: %q with error %v", e.Cfg.ShortName, exec.GetCmdString(), err)
		return nil, err
	}
	return execResult, nil
}

// RunExecType is the final function that calls the runtime to execute a type.Exec on a container
// This is to be overriden if the nodes implementation differs
func (e *extcont) RunExecTypeWoWait(ctx context.Context, exec types.ExecCmd) error {
	err := e.GetRuntime().ExecNotWait(ctx, e.Cfg.ShortName, exec)
	if err != nil {
		// On Ext-container we have to use the shortname, whilst default is to use longname
		log.Errorf("%s: failed to execute cmd: %q with error %v", e.Cfg.ShortName, exec.GetCmdString(), err)
		return err
	}
	return nil
}
