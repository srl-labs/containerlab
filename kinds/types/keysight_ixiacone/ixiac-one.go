// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package keysight_ixiacone

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"keysight_ixia-c-one"}

var ixiacStatusConfig = struct {
	statusSleepDuration time.Duration
	readyFileName       string
}{
	statusSleepDuration: time.Duration(time.Second * 5),
	readyFileName:       "/home/keysight/ixia-c-one/init-done",
}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(ixiacOne)
	}, nil)
}

type ixiacOne struct {
	nodes.DefaultNode
}

func (l *ixiacOne) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	l.DefaultNode = *nodes.NewDefaultNode(l)

	l.Cfg = cfg
	for _, o := range opts {
		o(l)
	}

	return nil
}

func (l *ixiacOne) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for keysight_ixia-c-one '%s' node", l.Cfg.ShortName)
	return l.ixiacPostDeploy(ctx)
}

// ixiacPostDeploy runs postdeploy actions which are required for keysight_ixia-c-one node.
func (l *ixiacOne) ixiacPostDeploy(ctx context.Context) error {
	ixiacOneCmd := fmt.Sprintf("bash -c 'ls %s'", ixiacStatusConfig.readyFileName)
	statusInProgressMsg := fmt.Sprintf("ls: %s: No such file or directory", ixiacStatusConfig.readyFileName)
	for {
		cmd, _ := exec.NewExecCmdFromString(ixiacOneCmd)
		execResult, err := l.RunExec(ctx, cmd)
		if err != nil {
			return err
		}
		if len(execResult.GetStdErrString()) > 0 {
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
