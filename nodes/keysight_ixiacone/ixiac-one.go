// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package keysight_ixiacone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"keysight_ixia-c-one"}

var ixiacStatusConfig = struct {
	statusSleepDuration time.Duration
	readyFileName       string
}{
	statusSleepDuration: time.Second * 5,
	readyFileName:       "/home/keysight/ixia-c-one/init-done",
}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	r.Register(kindnames, func() clabnodes.Node {
		return new(ixiacOne)
	}, nil)
}

type ixiacOne struct {
	clabnodes.DefaultNode
}

func (l *ixiacOne) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	l.DefaultNode = *clabnodes.NewDefaultNode(l)

	l.Cfg = cfg
	for _, o := range opts {
		o(l)
	}

	return nil
}

func (l *ixiacOne) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for keysight_ixia-c-one '%s' node", l.Cfg.ShortName)
	return l.ixiacPostDeploy(ctx)
}

// ixiacPostDeploy runs postdeploy actions which are required for keysight_ixia-c-one node.
func (l *ixiacOne) ixiacPostDeploy(ctx context.Context) error {
	ixiacOneCmd := fmt.Sprintf("bash -c 'ls %s'", ixiacStatusConfig.readyFileName)
	statusInProgressMsg := fmt.Sprintf("ls: %s: No such file or directory", ixiacStatusConfig.readyFileName)
	for {
		cmd, _ := clabexec.NewExecCmdFromString(ixiacOneCmd)
		execResult, err := l.RunExec(ctx, cmd)
		if err != nil {
			return err
		}
		if execResult.GetStdErrString() != "" {
			msg := strings.TrimSuffix(execResult.GetStdErrString(), "\n")
			if msg != statusInProgressMsg {
				return err
			}
			time.Sleep(ixiacStatusConfig.statusSleepDuration)
		} else {
			break
		}
	}

	return nil
}
