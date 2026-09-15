// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestSkipMgmtNetwork(t *testing.T) {
	withNodes := func(nodes map[string]*clabtypes.NodeDefinition) *clabtypes.Topology {
		topo := clabtypes.NewTopology()
		for n, d := range nodes {
			topo.Nodes[n] = d
		}

		return topo
	}

	tests := []struct {
		name string
		topo *clabtypes.Topology
		mgmt clabtypes.MgmtNet
		want bool
	}{
		{
			name: "every node explicitly none",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "none"},
			}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "none inherited from defaults",
			topo: func() *clabtypes.Topology {
				topo := withNodes(map[string]*clabtypes.NodeDefinition{"n1": {}, "n2": {}})
				topo.Defaults.NetworkMode = "none"
				return topo
			}(),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "none inherited from kind",
			topo: func() *clabtypes.Topology {
				topo := withNodes(map[string]*clabtypes.NodeDefinition{
					"n1": {Kind: "linux"},
					"n2": {Kind: "linux"},
				})
				topo.Kinds["linux"] = &clabtypes.NodeDefinition{NetworkMode: "none"}
				return topo
			}(),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "flag unset preserves old behavior even when all-none",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "none"},
			}),
			want: false,
		},
		{
			name: "mixed: one node uses mgmt",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "container:foo"},
			}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
		{
			name: "no NetworkMode anywhere (default mgmt attachment)",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{"n1": {}}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
		{
			name: "empty topology is not 'unused'",
			topo: withNodes(nil),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &CLab{
				Config: &Config{
					Mgmt:     &tc.mgmt,
					Topology: tc.topo,
				},
			}

			if got := c.skipMgmtNetwork(); got != tc.want {
				t.Errorf("skipMgmtNetwork() = %v, want %v", got, tc.want)
			}
		})
	}
}
