// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes_test

import (
	"testing"

	"github.com/srl-labs/containerlab/nodes"
	ceosnodes "github.com/srl-labs/containerlab/nodes/ceos"
	linuxnodes "github.com/srl-labs/containerlab/nodes/linux"
	srlnodes "github.com/srl-labs/containerlab/nodes/srl"
	srosnodes "github.com/srl-labs/containerlab/nodes/sros"
	vrsrosnodes "github.com/srl-labs/containerlab/nodes/vr_sros"
)

// TestNodeRegistryLinkApplyModeForKind checks that the kind default a node kind declares
// resolves from the registry without a configured node, matching what a plain deploy
// would use.
func TestNodeRegistryLinkApplyModeForKind(t *testing.T) {
	t.Parallel()

	registry := nodes.NewNodeRegistry()
	for _, register := range []func(*nodes.NodeRegistry){
		linuxnodes.Register,
		srlnodes.Register,
		ceosnodes.Register,
		vrsrosnodes.Register,
		srosnodes.Register,
	} {
		register(registry)
	}

	tests := []struct {
		kind string
		want nodes.LinkApplyMode
	}{
		{kind: "linux", want: nodes.LinkApplyModeLive},
		{kind: "nokia_srlinux", want: nodes.LinkApplyModeLive},
		{kind: "arista_ceos", want: nodes.LinkApplyModeRestart},
		{kind: "nokia_srsim", want: nodes.LinkApplyModeLive},
		{kind: "nokia_sros", want: nodes.LinkApplyModeRecreate},
		{kind: "cisco_c8000v", want: nodes.LinkApplyModeRecreate},
		{kind: "alpine", want: nodes.LinkApplyModeRecreate},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			got := registry.LinkApplyModeForKind(tc.kind)
			if got != tc.want {
				t.Fatalf("LinkApplyModeForKind(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}
