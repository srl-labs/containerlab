// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabtypes "github.com/srl-labs/containerlab/types"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// dotEdgeRe matches an undirected edge line in graphviz dot output, e.g.
// `node1--node2;` (gographviz may quote the node names).
var dotEdgeRe = regexp.MustCompile(`(?m)^\s*"?[^"\s]+"?\s*--\s*"?[^"\s;]+"?`)

// mermaidEdgeRe matches an edge line in mermaid flowchart output, e.g.
// `  node1---node2`.
var mermaidEdgeRe = regexp.MustCompile(`(?m)^\s+\S+---\S+\s*$`)

// graphTestNode is a minimal clablinks.Node implementation used to construct
// links in graph generation tests without standing up a real runtime.
type graphTestNode struct {
	name string
}

func (n *graphTestNode) GetShortName() string { return n.name }

func (*graphTestNode) AddLinkToContainer(
	_ context.Context,
	_ netlink.Link,
	_ func(ns.NetNS) error,
) error {
	return nil
}

func (*graphTestNode) AddEndpoint(_ clablinks.Endpoint) error { return nil }

func (*graphTestNode) AdoptEndpoint(_ clablinks.Endpoint) error { return nil }

func (*graphTestNode) ReleaseEndpoint(_ clablinks.Endpoint) error { return nil }

func (*graphTestNode) GetLinkEndpointType() clablinks.LinkEndpointType {
	return clablinks.LinkEndpointTypeVeth
}

func (*graphTestNode) GetEndpoints() []clablinks.Endpoint { return nil }

func (*graphTestNode) ExecFunction(_ context.Context, _ func(ns.NetNS) error) error {
	return nil
}

func (*graphTestNode) GetState() clabnodesstate.NodeState {
	return clabnodesstate.NodeState(0)
}

func (*graphTestNode) Delete(_ context.Context) error { return nil }

// newVethLink builds a real two-endpoint veth link between the named nodes.
func newVethLink(aNode, bNode string) clablinks.Link {
	l := clablinks.NewLinkVEth()
	l.Endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointVeth(
			clablinks.NewEndpointGeneric(&graphTestNode{name: aNode}, "eth1", l),
		),
		clablinks.NewEndpointVeth(
			clablinks.NewEndpointGeneric(&graphTestNode{name: bNode}, "eth1", l),
		),
	}

	return l
}

// newDummyLink builds a real single-endpoint dummy link, mirroring the topology
// in https://github.com/srl-labs/containerlab/issues/3226.
func newDummyLink(node string) clablinks.Link {
	l := clablinks.NewLinkDummy()
	l.Endpoints = []clablinks.Endpoint{
		clablinks.NewEndpointDummy(
			clablinks.NewEndpointGeneric(&graphTestNode{name: node}, "eth1", l),
		),
	}

	return l
}

// newZeroEndpointLink builds a dummy link with no endpoints to exercise the
// defensive guard against links that report zero endpoints.
func newZeroEndpointLink() clablinks.Link {
	l := clablinks.NewLinkDummy()
	l.Endpoints = nil

	return l
}

// newGraphTestCLab returns a CLab wired with the provided links and a TopoPaths
// rooted in a temporary lab directory so the generators can write output files.
func newGraphTestCLab(t *testing.T, links map[int]clablinks.Link) *CLab {
	t.Helper()

	labDir := t.TempDir()
	topoFile := filepath.Join(labDir, "graphtest.clab.yml")
	if err := os.WriteFile(topoFile, []byte("name: graphtest\n"), 0o644); err != nil {
		t.Fatalf("failed to write temp topology file: %v", err)
	}

	tp, err := clabtypes.NewTopoPaths(topoFile, nil)
	if err != nil {
		t.Fatalf("failed to create TopoPaths: %v", err)
	}

	if err := tp.SetLabDir(labDir); err != nil {
		t.Fatalf("failed to set lab dir: %v", err)
	}

	return &CLab{
		Config:    &Config{Name: "graphtest"},
		TopoPaths: tp,
		Nodes:     map[string]clabnodes.Node{},
		Links:     links,
	}
}

// readGraphFile reads the single graph output file written under the lab's graph
// directory.
func readGraphFile(t *testing.T, c *CLab, ext string) string {
	t.Helper()

	data, err := os.ReadFile(c.TopoPaths.GraphFilename(ext))
	if err != nil {
		t.Fatalf("failed to read graph file: %v", err)
	}

	return string(data)
}

// TestGenerateDotGraphSkipsNonPointToPointLinks verifies that single- and
// zero-endpoint links no longer panic the dot generator (issue #3226) and emit
// no edge. The dot backend requires edge endpoints to be registered as graph
// nodes first, so node-to-node edges are covered by the mermaid test instead.
func TestGenerateDotGraphSkipsNonPointToPointLinks(t *testing.T) {
	tests := []struct {
		name  string
		links map[int]clablinks.Link
	}{
		{
			name:  "single dummy link",
			links: map[int]clablinks.Link{0: newDummyLink("node1")},
		},
		{
			name:  "zero endpoint link",
			links: map[int]clablinks.Link{0: newZeroEndpointLink()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newGraphTestCLab(t, tt.links)

			if err := c.GenerateDotGraph(context.Background()); err != nil {
				t.Fatalf("GenerateDotGraph returned error: %v", err)
			}

			out := readGraphFile(t, c, ".dot")
			if got := len(dotEdgeRe.FindAllString(out, -1)); got != 0 {
				t.Fatalf("dot graph edge count = %d, want 0\noutput:\n%s", got, out)
			}
		})
	}
}

func TestGenerateMermaidGraphSkipsNonPointToPointLinks(t *testing.T) {
	tests := []struct {
		name      string
		links     map[int]clablinks.Link
		wantEdges int
	}{
		{
			name:      "single veth link renders one edge",
			links:     map[int]clablinks.Link{0: newVethLink("node1", "node2")},
			wantEdges: 1,
		},
		{
			name:      "single dummy link renders no edge and does not panic",
			links:     map[int]clablinks.Link{0: newDummyLink("node1")},
			wantEdges: 0,
		},
		{
			name:      "zero endpoint link renders no edge and does not panic",
			links:     map[int]clablinks.Link{0: newZeroEndpointLink()},
			wantEdges: 0,
		},
		{
			name: "mixed veth and dummy links render only the veth edge",
			links: map[int]clablinks.Link{
				0: newVethLink("node1", "node2"),
				1: newDummyLink("node1"),
			},
			wantEdges: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newGraphTestCLab(t, tt.links)

			if err := c.GenerateMermaidGraph("TB"); err != nil {
				t.Fatalf("GenerateMermaidGraph returned error: %v", err)
			}

			out := readGraphFile(t, c, ".mermaid")
			got := len(mermaidEdgeRe.FindAllString(out, -1))
			if got != tt.wantEdges {
				t.Fatalf("mermaid graph edge count = %d, want %d\noutput:\n%s", got, tt.wantEdges, out)
			}
		})
	}
}
