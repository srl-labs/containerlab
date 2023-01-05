// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var kindnames = []string{"host"}

// Register registers the node in the global Node map.
func Register() {
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
func (*host) GetContainers(_ context.Context) ([]runtime.GenericContainer, error) {
	return []runtime.GenericContainer{
		{
			Names:   []string{"Host"},
			State:   "running",
			ID:      "N/A",
			ShortID: "N/A",
			Image:   "-",
			Status:  "running",
			NetworkSettings: runtime.GenericMgmtIPs{
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

func (h *host) RunExecs(_ context.Context, _ []string) ([]exec.ExecResultHolder, error) {
	log.Warnf("Exec operation is not implemented for kind %q", h.Config().Kind)

	return nil, exec.ErrRunExecNotSupported
}
