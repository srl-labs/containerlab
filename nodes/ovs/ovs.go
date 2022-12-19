// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ovs

import (
	"context"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"ovs-bridge"}

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(ovs)
	})
}

type ovs struct {
	nodes.DefaultNode
}

func (s *ovs) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	return nil
}

func (s *ovs) CheckDeploymentConditions(_ context.Context) error {
	err := s.VerifyHostRequirements()
	if err != nil {
		return err
	}
	// check bridge exists
	_, err = utils.BridgeByName(s.Cfg.ShortName)
	if err != nil {
		return err
	}
	return nil
}

func (*ovs) Deploy(_ context.Context) error                { return nil }
func (*ovs) PullImage(_ context.Context) error             { return nil }
func (*ovs) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*ovs) Delete(_ context.Context) error                { return nil }
