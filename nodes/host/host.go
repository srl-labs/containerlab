// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"host"}

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(host)
	})
}

type host struct {
	nodes.DefaultNode
}

func (s *host) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}

	return nil
}
func (*host) Deploy(_ context.Context) error                { return nil }
func (*host) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*host) PullImage(_ context.Context) error             { return nil }
func (*host) Delete(_ context.Context) error                { return nil }
func (*host) WithMgmtNet(*types.MgmtNet)                    {}

// UpdateConfigWithRuntimeInfo is a noop for hosts.
func (*host) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers returns a basic skeleton of a container to enable graphing of hosts kinds.
func (*host) GetContainers(_ context.Context) ([]types.GenericContainer, error) {
	return []types.GenericContainer{
		{
			Names:   []string{"Host"},
			State:   "running",
			ID:      "N/A",
			ShortID: "N/A",
			Image:   "-",
			Status:  "running",
			NetworkSettings: types.GenericMgmtIPs{
				IPv4addr: "N/A",
				IPv4pLen: 0,
				IPv4Gw:   "N/A",
				IPv6addr: "N/A",
				IPv6pLen: 0,
				IPv6Gw:   "N/A",
			},
		},
	}, nil
}

func (h *host) RunExecConfig(_ context.Context) ([]types.ExecResultHolder, error) {
	if h.Cfg.Exec != nil && len(h.Cfg.Exec) > 0 {
		log.Error("exec not supported on kind 'host' -> noop; continuing")
	}
	return []types.ExecResultHolder{}, nil
}

func (h *host) RunExecType(_ context.Context, _ types.ExecOperation) (types.ExecResultHolder, error) {
	log.Error("exec not supported on kind 'host' -> noop; continuing")
	return nil, types.ErrRunExecTypeNotSupported
}
