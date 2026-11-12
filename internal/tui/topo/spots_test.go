// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"reflect"
	"testing"
)

func spotGraph() Graph {
	return Graph{
		Nodes: []Node{
			{ID: "spine1", Role: "spine"},
			{ID: "spine2", Role: "spine"},
			{ID: "leaf1", Role: "leaf"},
			{ID: "leaf2", Role: "leaf"},
			{ID: "alone"},
		},
		Links: []Link{
			{A: "spine1", B: "leaf1"},
			{A: "spine1", B: "leaf2"},
			{A: "spine2", B: "leaf1"},
			{A: "spine2", B: "leaf2"},
		},
	}
}

func TestSeedFollowsTheLayoutOfAGraphWithoutPositions(t *testing.T) {
	const aspect = 2.25

	g := spotGraph()
	spots := Seed(g, nil, aspect)
	layout := Compute(g, Options{Zoom: ZoomNormal})

	if len(spots) != len(g.Nodes) {
		t.Fatalf("%d of %d nodes have a spot", len(spots), len(g.Nodes))
	}

	for id, s := range spots {
		r, _ := layout.NodeRect(id)
		wantX := (float64(r.X) + float64(r.W)/2) * SpotCol
		wantY := (float64(r.Y) + float64(r.H)/2) * SpotCol * aspect

		if math.Abs(s.X-wantX) > 1e-9 || math.Abs(s.Y-wantY) > 1e-9 {
			t.Errorf("%s is at %v, want %v,%v", id, s, wantX, wantY)
		}
	}

	if spots["spine1"].Y >= spots["leaf1"].Y {
		t.Error("the spines are not above the leaves")
	}

	// The order of the nodes and the links does not matter.
	g.Nodes[0], g.Nodes[3] = g.Nodes[3], g.Nodes[0]
	g.Links[0], g.Links[2] = g.Links[2], g.Links[0]

	if again := Seed(g, nil, aspect); !reflect.DeepEqual(again, spots) {
		t.Errorf("the spots depend on the order of the graph: %v and %v", spots, again)
	}
}

func TestSeedPlacesWhatIsMissingNextToItsNeighbors(t *testing.T) {
	known := map[string]Spot{"spine1": {X: 100, Y: 100}, "spine2": {X: 300, Y: 100}}
	spots := Seed(spotGraph(), known, 2)

	for id, want := range known {
		if spots[id] != want {
			t.Errorf("%s moved from %v to %v", id, want, spots[id])
		}
	}

	for _, id := range []string{"leaf1", "leaf2"} {
		if s := spots[id]; s.Y <= 100 || s.X < 0 || s.X > 400 {
			t.Errorf("%s is at %v, want it under the spines", id, s)
		}
	}

	// No two nodes on top of each other.
	for a, sa := range spots {
		for b, sb := range spots {
			if a < b && math.Abs(sa.X-sb.X) < spotClear && math.Abs(sa.Y-sb.Y) < spotClear {
				t.Errorf("%s and %s are both at about %v", a, b, sa)
			}
		}
	}

	// A node without a placed neighbor goes below everything else.
	lowest := math.Inf(-1)

	for id, s := range spots {
		if id != "alone" {
			lowest = math.Max(lowest, s.Y)
		}
	}

	if spots["alone"].Y <= lowest {
		t.Errorf("the node without links is at %v, want it below %v", spots["alone"], lowest)
	}
}

func TestDrawn(t *testing.T) {
	g := spotGraph()

	if Drawn(g, ModeAuto) || Drawn(g, ModeSaved) {
		t.Error("a graph without positions is drawn by them")
	}

	g.Nodes[0].HasPos = true

	if Drawn(g, ModeAuto) {
		t.Error("one position in five makes a drawing")
	}

	if !Drawn(g, ModeSaved) {
		t.Error("the saved positions are not used when asked for")
	}

	for i := range g.Nodes[:4] {
		g.Nodes[i].HasPos = true
	}

	if !Drawn(g, ModeAuto) || Drawn(g, ModeTiers) || Drawn(g, ModeGraph) {
		t.Error("four positions in five: want a drawing unless another layout is asked for")
	}
}
