// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rowsOf returns the node IDs of a drawing row by row, each from left to right.
func rowsOf(l *Layout) [][]string {
	byY := map[int][]*box{}

	for _, b := range l.nodes {
		byY[b.rect.Y] = append(byY[b.rect.Y], b)
	}

	ys := make([]int, 0, len(byY))
	for y := range byY {
		ys = append(ys, y)
	}

	sort.Ints(ys)

	rows := make([][]string, 0, len(ys))

	for _, y := range ys {
		row := byY[y]
		sort.Slice(row, func(i, j int) bool { return row[i].rect.X < row[j].rect.X })

		ids := make([]string, len(row))
		for i, b := range row {
			ids[i] = b.ID
		}

		rows = append(rows, ids)
	}

	return rows
}

func words(rows ...string) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = strings.Fields(r)
	}

	return out
}

// crossingCells counts the cells where two routes cross.
func crossingCells(l *Layout) int {
	seen := map[pt]int{}
	count := 0

	for _, e := range l.edges {
		for _, p := range e.path[1 : len(e.path)-1] {
			if owner, ok := seen[p]; ok && owner != e.ID {
				count++
			}

			seen[p] = e.ID
		}
	}

	return count
}

func TestEmptyGraph(t *testing.T) {
	l := Compute(Graph{}, Options{})
	w, h := l.Size()

	assert.Zero(t, w)
	assert.Zero(t, h)
	assert.Empty(t, l.Edges())
	assert.Empty(t, l.Paint(State{}, Selection{Edge: -1}).Lines())
	assert.Equal(t, Hit{Edge: -1}, l.Hit(0, 0))

	_, ok := l.Neighbor("a", DirDown)
	assert.False(t, ok)
	assert.Nil(t, l.EdgesOf("a"))
}

func TestModeSelection(t *testing.T) {
	placed := func(g Graph, share int) Graph {
		for i := range g.Nodes {
			if i*100 < share*len(g.Nodes) {
				g.Nodes[i].HasPos, g.Nodes[i].X, g.Nodes[i].Y = true, float64(
					100*i,
				), float64(
					100*(i%3),
				)
			}
		}

		return g
	}

	for _, tc := range []struct {
		name  string
		graph Graph
		ask   Mode
		want  Mode
	}{
		{"names give tiers", clos(2, 4), ModeAuto, ModeTiers},
		{"opaque names give the graph layout", ring(6), ModeAuto, ModeGraph},
		{"saved positions win when most nodes have one", placed(clos(2, 8), 80), ModeAuto, ModeSaved},
		{"a few saved positions are ignored", placed(clos(2, 8), 50), ModeAuto, ModeTiers},
		{"a few saved positions are used on request", placed(clos(2, 8), 50), ModeSaved, ModeSaved},
		{"saved falls back without positions", clos(2, 4), ModeSaved, ModeTiers},
		{"tiers fall back without names or levels", ring(6), ModeTiers, ModeGraph},
		{"graph is always possible", clos(2, 4), ModeGraph, ModeGraph},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Compute(tc.graph, Options{Mode: tc.ask}).Mode())
		})
	}
}

func TestRowsFromNames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		graph Graph
		want  [][]string
	}{
		{
			"service provider", serviceProvider(),
			words("rr1 rr2", "p1 p2 p3 p4", "pe1 pe2 pe3 pe4", "ce1 ce2 ce3 ce4"),
		},
		{
			"campus", campus(),
			words("internet", "fw1", "core1 core2", "dist1 dist2 dist3 dist4",
				"acc1 acc2 acc3 acc4 acc5 acc6"),
		},
		{
			"metro ring", metroRing(true),
			words("cpe1 cpe2", "r1 r2 r3 r4", "r8 r7 r6 r5", "cpe4 cpe3"),
		},
		{"ring", metroRing(false), words("r1 r2 r3 r4", "r8 r7 r6 r5")},
		{"three tier", threeTier(), words("spine1 spine2", "leaf1 leaf2 leaf3 leaf4",
			"client1 client2 client3 client4")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := Compute(tc.graph, Options{Zoom: ZoomNormal})
			assert.Equal(t, tc.want, rowsOf(l))
		})
	}
}

func TestRingHasNoCrossings(t *testing.T) {
	for _, g := range []Graph{ring(6), metroRing(false), metroRing(true)} {
		l := Compute(g, Options{Zoom: ZoomNormal})
		assert.Equal(t, ModeGraph, l.Mode())
		assert.Zero(t, crossingCells(l))
	}
}

// TestNodesNamedAlikeShareARow covers names that mean nothing to the layout: the
// row of such nodes follows from what they connect to, and all of them get it.
func TestNodesNamedAlikeShareARow(t *testing.T) {
	g := serviceProvider()
	rename := strings.NewReplacer("p1", "xr1", "p2", "xr2", "p3", "xr3", "p4", "xr4")

	for i := range g.Nodes {
		if strings.HasPrefix(g.Nodes[i].ID, "p") && !strings.HasPrefix(g.Nodes[i].ID, "pe") {
			g.Nodes[i].ID = rename.Replace(g.Nodes[i].ID)
		}
	}

	for i := range g.Links {
		for _, end := range []*string{&g.Links[i].A, &g.Links[i].B} {
			if !strings.HasPrefix(*end, "pe") {
				*end = rename.Replace(*end)
			}
		}
	}

	l := Compute(g, Options{Zoom: ZoomNormal})

	assert.Equal(t, ModeTiers, l.Mode())
	assert.Equal(
		t,
		words("rr1 rr2", "xr1 xr2 xr3 xr4", "pe1 pe2 pe3 pe4", "ce1 ce2 ce3 ce4"),
		rowsOf(l),
	)
}

func TestUnknownNodeGoesBetweenItsNeighbors(t *testing.T) {
	var b builder

	b.nodes("spine", 2)
	b.nodes("leaf", 2)
	b.nodes("client", 2)
	b.g.Nodes = append(b.g.Nodes, Node{ID: "tap"}, Node{ID: "probe"})
	b.links("spine1-leaf1", "spine1-leaf2", "spine2-leaf1", "spine2-leaf2",
		"leaf1-client1", "leaf2-client2", "spine1-tap", "tap-client1", "leaf2-probe")

	l := Compute(b.g, Options{Zoom: ZoomNormal})

	// The tap sits between a spine and a client, so it joins the leaves; the
	// probe hangs off a leaf like a client does.
	assert.Equal(
		t,
		words("spine1 spine2", "leaf1 tap leaf2", "client1 client2 probe"),
		sorted(rowsOf(l)),
	)
}

func sorted(rows [][]string) [][]string {
	for _, r := range rows {
		sort.Slice(r, func(i, j int) bool {
			return natLess(roleKey(r[i]), roleKey(r[j]))
		})
	}

	return rows
}

// roleKey orders the names of a row for comparison without fixing where the
// layout puts an inferred node among them.
func roleKey(id string) string {
	switch id {
	case "tap":
		return "leaf1z"
	case "probe":
		return "client9"
	}

	return id
}

func TestTokenTier(t *testing.T) {
	for name, want := range map[string]int{
		"p1": tierCore, "P2": tierCore, "rr1": tierReflector, "reflector-east": tierReflector,
		"pe1": tierSpine, "ce12": tierLeaf, "cpe1": tierLeaf, "acc3": tierLeaf, "access1": tierLeaf,
		"fw1": tierFirewall, "firewall": tierFirewall, "internet": tierInternet,
		"dc1-super-spine2": tierSuperSpine, "superspine1": tierSuperSpine, "spine01": tierSpine,
		"leafs": tierLeaf, "client1": tierHost, "core1": tierCore, "dist2": tierSpine,
	} {
		got, ok := tokenTier(name)
		assert.True(t, ok, name)
		assert.Equal(t, want, got, name)
	}

	for _, name := range []string{"peer1", "r1", "srl1", "ps", "space", "x-1", ""} {
		_, ok := tokenTier(name)
		assert.False(t, ok, name)
	}
}

func TestStem(t *testing.T) {
	assert.Equal(t, stem("p1"), stem("P12"))
	assert.Equal(t, stem("dc1-leaf3"), stem("dc2-leaf10"))
	assert.NotEqual(t, stem("p1"), stem("pe1"))
}

func TestSavedPositionsGiveRowsAndOrder(t *testing.T) {
	g := threeTier()
	pos := map[string][2]float64{
		"spine1": {400, 100}, "spine2": {200, 110},
		"leaf1": {100, 300}, "leaf2": {250, 310}, "leaf3": {400, 290}, "leaf4": {550, 300},
		"client1": {100, 500}, "client2": {250, 500}, "client4": {560, 500},
	}

	for i := range g.Nodes {
		if p, ok := pos[g.Nodes[i].ID]; ok {
			g.Nodes[i].X, g.Nodes[i].Y, g.Nodes[i].HasPos = p[0], p[1], true
		}
	}

	l := Compute(g, Options{Zoom: ZoomNormal})

	// spine2 was drawn left of spine1, and client3 has no position: it goes
	// below its only neighbor.
	assert.Equal(t, ModeSaved, l.Mode())
	assert.Equal(
		t,
		words("spine2 spine1", "leaf1 leaf2 leaf3 leaf4", "client1 client2 client3 client4"),
		rowsOf(l),
	)
}

func TestEdges(t *testing.T) {
	g := Graph{
		Nodes: []Node{{ID: "b"}, {ID: "a"}, {ID: "c"}, {ID: "a"}},
		Links: []Link{
			{A: "b", B: "a"},
			{A: "a", B: "b"},
			{A: "c", B: "a"},
			{A: "a", B: "ghost"},
			{A: "c", B: "c"},
			{A: "a", B: "b"},
		},
	}
	l := Compute(g, Options{Zoom: ZoomNormal})

	assert.Equal(t, []Edge{
		{ID: 0, A: "a", B: "b", Links: []int{0, 1, 5}},
		{ID: 1, A: "a", B: "c", Links: []int{2}},
	}, l.Edges())

	// Callers may keep the slice.
	l.Edges()[0].Links[0] = 99
	assert.Equal(t, []int{0, 1, 5}, l.Edges()[0].Links)

	lines := strings.Join(l.Paint(allUp(g), Selection{Edge: -1}).Lines(), "\n")
	assert.Contains(t, lines, "3x", "a bundle shows how many links it holds")
	assert.Contains(t, lines, string(loopMark), "a link from a node to itself marks the node")

	_, ok := l.NodeRect("ghost")
	assert.False(t, ok)
}

func TestEdgesOfIsInTabOrder(t *testing.T) {
	l := Compute(parallel(), Options{Zoom: ZoomNormal})
	id := func(a, b string) int {
		for _, e := range l.Edges() {
			if e.A == a && e.B == b {
				return e.ID
			}
		}

		t.Fatalf("no edge %s-%s", a, b)

		return -1
	}

	// Top border first, then the sides, then the bottom border.
	assert.Equal(t, []int{id("leaf1", "spine1"), id("leaf1", "leaf2")}, l.EdgesOf("leaf1"))
	assert.Equal(t, []int{id("leaf1", "spine1"), id("leaf2", "spine1")}, l.EdgesOf("spine1"))
	assert.Nil(t, l.EdgesOf("nobody"))

	l = Compute(threeTier(), Options{Zoom: ZoomNormal})
	edges := l.Edges()
	tab := l.EdgesOf("leaf2")

	require.Len(t, tab, 3)
	assert.Equal(t, "spine1", edges[tab[0]].B, "top ports go left to right")
	assert.Equal(t, "spine2", edges[tab[1]].B)
	assert.Equal(t, "client2", edges[tab[2]].A, "the bottom border comes last")
}
