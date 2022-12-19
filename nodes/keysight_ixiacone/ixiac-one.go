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
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
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

// Register registers the node in the global Node map.
func Register() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(ixiacOne)
	})
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

func (l *ixiacOne) PostDeploy(ctx context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for keysight_ixia-c-one '%s' node", l.Cfg.ShortName)
	return ixiacPostDeploy(ctx, l.Runtime, l.Cfg)
}

// ixiacPostDeploy runs postdeploy actions which are required for keysight_ixia-c-one node.
func ixiacPostDeploy(ctx context.Context, r runtime.ContainerRuntime, cfg *types.NodeConfig) error {
	ixiacOneCmd := fmt.Sprintf("ls %s", ixiacStatusConfig.readyFileName)
	statusInProgressMsg := fmt.Sprintf("ls: %s: No such file or directory", ixiacStatusConfig.readyFileName)
	for {
		_, stderr, err := r.Exec(ctx, cfg.LongName, []string{"bash", "-c", ixiacOneCmd})
		if err != nil {
			return err
		}
		if stderr != nil {
			msg := strings.TrimSuffix(string(stderr), "\n")
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
