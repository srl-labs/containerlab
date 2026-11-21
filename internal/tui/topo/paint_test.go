// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// edgeCells returns the painted cells an edge has to itself.
func edgeCells(l *Layout, g *Grid, id int) []Cell {
	var out []Cell

	e := l.edges[id]

	for i := 1; i+1 < len(e.path); i++ {
		p := e.path[i]
		if e.spanAt(i) == nil && l.Hit(p.x, p.y).Edge == id {
			out = append(out, g.Cell(p.x, p.y))
		}
	}

	return out
}

func edgeID(t *testing.T, l *Layout, a, b string) int {
	t.Helper()

	for _, e := range l.Edges() {
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			return e.ID
		}
	}

	t.Fatalf("no edge %s-%s", a, b)

	return -1
}

func TestLinkStatesShowInClassAndGlyph(t *testing.T) {
	g := threeTier()
	l := Compute(g, Options{Zoom: ZoomNormal})
	st := allUp(g)
	st.Links[4] = LinkDown    // spine2-leaf1
	st.Links[7] = LinkUnknown // spine2-leaf4
	st.Links[12] = LinkDown   // second link of the leaf2-client2 bundle
	grid := l.Paint(st, Selection{Edge: -1})

	for _, tc := range []struct {
		a, b  string
		class Class
		runes string
	}{
		{"spine1", "leaf1", ClassLinkUp, "│─┌┐└┘"},
		{"spine2", "leaf1", ClassLinkDown, "╎╌┌┐└┘"},
		{"spine2", "leaf4", ClassLinkUnknown, "┊┈┌┐└┘"},
		{"leaf2", "client2", ClassLinkMixed, "║═"},
	} {
		cells := edgeCells(l, grid, edgeID(t, l, tc.a, tc.b))
		require.NotEmpty(t, cells)

		for _, c := range cells {
			if c.Class == ClassBadge || c.Class == ClassBlank {
				continue
			}

			assert.Equal(t, tc.class, c.Class, "%s-%s", tc.a, tc.b)
			assert.Contains(t, tc.runes, string(c.Rune), "%s-%s", tc.a, tc.b)
			assert.False(t, c.Selected)
		}
	}
}

func TestSelectionIsMarkedAndDrawnHeavy(t *testing.T) {
	g := threeTier()
	l := Compute(g, Options{Zoom: ZoomNormal})
	edge := edgeID(t, l, "leaf1", "spine1")
	st := allUp(g)
	st.Nodes = map[string]NodeState{"leaf1": NodeReady, "leaf2": NodeDown}
	grid := l.Paint(st, Selection{Node: "leaf1", Edge: edge})

	for _, c := range edgeCells(l, grid, edge) {
		assert.True(t, c.Selected)
		assert.Contains(t, "┃━┏┓┗┛", string(c.Rune))
	}

	for _, b := range l.nodes {
		for y := b.rect.Y; y < b.rect.Y+b.rect.H; y++ {
			for x := b.rect.X; x < b.rect.X+b.rect.W; x++ {
				c := grid.Cell(x, y)
				assert.Equal(t, st.Nodes[b.ID], c.State, "state of %s", b.ID)

				// The port of the focused edge on another node is lit too.
				if b.ID != "spine1" {
					assert.Equal(t, b.ID == "leaf1", c.Selected, "%s at %d,%d", b.ID, x, y)
				}
			}
		}
	}

	r, _ := l.NodeRect("leaf1")
	assert.Equal(t, '┏', grid.Cell(r.X, r.Y).Rune)
	assert.Equal(t, ClassNodeBorder, grid.Cell(r.X, r.Y).Class)
	assert.Equal(t, ClassNodeLabel, grid.Cell(r.X+r.W/2, r.Y+1).Class)

	// No focus at all leaves nothing marked.
	plain := l.Paint(st, Selection{Edge: -1})
	w, h := plain.Size()

	for y := range h {
		for x := range w {
			assert.False(t, plain.Cell(x, y).Selected)
		}
	}

	assert.Equal(t, Cell{Rune: ' '}, plain.Cell(-1, -1))
	assert.Equal(t, Cell{Rune: ' '}, plain.Cell(w, h))
}

func TestEdgeLabels(t *testing.T) {
	g := threeTier()
	l := Compute(g, Options{Zoom: ZoomNormal})
	st := allUp(g)
	st.EdgeLabels = map[int]string{
		edgeID(t, l, "leaf1", "client1"): "1.2M",
		edgeID(t, l, "leaf3", "spine1"):  "880k",
		edgeID(t, l, "leaf2", "client2"): "7k",
	}
	grid := l.Paint(st, Selection{Edge: -1})
	text := strings.Join(grid.Lines(), "\n")

	assert.Contains(t, text, "│ 1.2M", "a label sits beside a vertical link")
	assert.Contains(t, text, " 880k ", "or on a horizontal stretch")
	assert.Contains(t, text, "║ 2x 7k", "after the badge of a bundle")

	w, h := grid.Size()
	classes := map[Class]int{}

	for y := range h {
		for x := range w {
			classes[grid.Cell(x, y).Class]++
		}
	}

	assert.Equal(t, len("1.2M")+len("880k")+len("7k"), classes[ClassEdgeLabel])
	assert.Equal(t, len("2x"), classes[ClassBadge])
}

func TestDetailZoomShowsNotes(t *testing.T) {
	g := serviceProvider()
	for i := range g.Nodes {
		g.Nodes[i].Kind = "nokia_srlinux"
	}

	l := Compute(g, Options{Zoom: ZoomDetail})
	long := "a note far too long for any box, whatever its width may be"
	st := State{NodeNotes: map[string]string{"ce1": "1% 24M", "pe1": "100% 1024M", "p1": long}}
	text := strings.Join(l.Paint(st, Selection{Edge: -1}).Lines(), "\n")

	assert.Contains(t, text, " 1% 24M ", "a note is shown whole")
	assert.Contains(t, text, " 100% 1024M ")
	assert.Contains(t, text, " nokia_srlinux ", "the kind stands in where there is no note")

	r, _ := l.NodeRect("p1")
	assert.Equal(t, 4, r.H)
	assert.Contains(t, text, "│ "+long[:r.W-5]+"~ │", "a note that cannot fit loses its end")

	// The same boxes whatever the notes say: they arrive after the layout.
	for _, id := range []string{"ce1", "ce2"} {
		r, _ := l.NodeRect(id)
		assert.GreaterOrEqual(t, r.W, noteCells+4)
	}
}

func TestLabelsAreNotCutWhileTheyFit(t *testing.T) {
	g := leafSpine(4, 12)
	compact := strings.Join(
		Compute(g, Options{Zoom: ZoomCompact}).Paint(State{}, Selection{Edge: -1}).Lines(),
		"\n",
	)
	normal := strings.Join(
		Compute(g, Options{Zoom: ZoomNormal}).Paint(State{}, Selection{Edge: -1}).Lines(),
		"\n",
	)

	for _, label := range []string{"leaf1 +4", "leaf10 +4", "leaf12 +4"} {
		assert.Contains(t, compact, "["+label+"]")
		assert.Contains(t, normal, " "+label+" ")
	}

	// A box widened for its ports shows more of a long label than the limit.
	long := Graph{Nodes: []Node{{ID: "hub", Label: "aggregation-router-1"}}}
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		long.Nodes = append(long.Nodes, Node{ID: n, Level: 2})
		long.Links = append(long.Links, Link{A: "hub", B: n})
	}

	long.Nodes[0].Level = 1
	text := strings.Join(
		Compute(long, Options{Zoom: ZoomNormal}).Paint(State{}, Selection{Edge: -1}).Lines(),
		"\n",
	)
	assert.Contains(t, text, " aggregation-router-1 ")
}

func TestFit(t *testing.T) {
	for _, tc := range []struct {
		in    string
		limit int
		want  string
	}{
		{"leaf1", 8, "leaf1"},
		{"leaf10 +4", 9, "leaf10 +4"},
		{"leaf10 +4", 8, "le~10 +4"},
		{"aggregation-switch-12", 14, "aggrega~tch-12"},
		{"aggregation 100 links", 12, "aggreg~links"},
		{"ab", 1, "a"},
		{"héllo wörld", 5, "hé~ld"},
	} {
		got := fit(tc.in, tc.limit)
		assert.Equal(t, tc.want, got)
		assert.LessOrEqual(t, utf8.RuneCountInString(got), tc.limit)
	}

	assert.Equal(t, "12% 7~", clip("12% 712M", 6))
	assert.Equal(t, "12% 712M", clip("12% 712M", 8))
	assert.Empty(t, clip("abc", 0))
}

func TestClean(t *testing.T) {
	assert.Equal(t, "leaf1", clean(" leaf1\t", false))
	assert.Equal(t, "ab", clean("a\x00\u200b\u0301b", false), "invisible runes are dropped")
	assert.Equal(t, "n-??", clean("n-\uff21\uff22", false), "wide runes would break the grid")
	assert.Equal(t, "café ↓", clean("café ↓", false))
	assert.Equal(t, "caf? ?", clean("café ↓", true))
}

func TestSharedPrefix(t *testing.T) {
	assert.Equal(t, 4, sharedPrefix([]string{"dc1-leaf1", "dc1-leaf2", "dc1-spine1"}))
	assert.Zero(t, sharedPrefix([]string{"leaf1", "leaf2"}), "only whole words are cut")
	assert.Zero(t, sharedPrefix([]string{"dc1-", "dc1-leaf2"}), "nothing may be left empty")
	assert.Zero(t, sharedPrefix([]string{"dc1-leaf1"}))
}

func TestNaturalOrder(t *testing.T) {
	names := []string{"leaf10", "leaf2", "leaf1", "leaf02", "spine1", "leaf"}
	want := []string{"leaf", "leaf1", "leaf02", "leaf2", "leaf10", "spine1"}

	l := Compute(Graph{Nodes: []Node{{ID: "leaf10"}, {ID: "leaf2"}, {ID: "leaf1"}}}, Options{})
	assert.Equal(t, [][]string{{"leaf1", "leaf2", "leaf10"}}, rowsOf(l))

	for i, a := range want {
		for j, b := range want {
			assert.Equal(t, i < j, natLess(a, b), "%s < %s", a, b)
		}
	}

	assert.Len(t, names, len(want))
}

// TestASCII checks that the ASCII mode keeps to printable ASCII whatever the input.
func TestASCII(t *testing.T) {
	r := rand.New(rand.NewSource(3))

	for _, tc := range propCases() {
		g := Graph{Nodes: append([]Node(nil), tc.graph.Nodes...), Links: tc.graph.Links}
		st := State{
			Links:      map[int]LinkState{},
			EdgeLabels: map[int]string{},
			NodeNotes:  map[string]string{},
		}

		for i := range g.Nodes {
			if i%3 == 0 {
				g.Nodes[i].Label = "n\u0153ud-\uff21\uff22-" + g.Nodes[i].ID
			}

			st.NodeNotes[g.Nodes[i].ID] = "↓1.2M ↑880k"
		}

		for i := range g.Links {
			st.Links[i] = LinkState(r.Intn(4))
		}

		for _, zoom := range []Zoom{ZoomCompact, ZoomNormal, ZoomDetail} {
			l := Compute(g, Options{Zoom: zoom, ASCII: true})

			for _, e := range l.Edges() {
				st.EdgeLabels[e.ID] = "↓2.6M"
			}

			sel := Selection{Edge: 0}
			if len(g.Nodes) > 0 {
				sel.Node = g.Nodes[0].ID
			}

			for y, line := range l.Paint(st, sel).Lines() {
				for _, c := range line {
					require.True(
						t,
						c >= ' ' && c <= '~',
						"%s zoom %d line %d: %q",
						tc.name,
						zoom,
						y,
						c,
					)
				}
			}
		}
	}
}

// TestCellsAreOneColumnWide checks that the drawing itself only uses ASCII and box
// drawing characters, which every terminal draws one column wide.
func TestCellsAreOneColumnWide(t *testing.T) {
	r := rand.New(rand.NewSource(5))

	for _, lc := range layouts() {
		st := State{Links: map[int]LinkState{}}
		for i := range lc.graph.Links {
			st.Links[i] = LinkState(r.Intn(4))
		}

		sel := Selection{Edge: 0}
		if len(lc.layout.nodes) > 0 {
			sel.Node = lc.layout.nodes[0].ID
		}

		for _, line := range lc.layout.Paint(st, sel).Lines() {
			for _, c := range line {
				boxDrawing := c >= 0x2500 && c <= 0x257F
				require.True(t, boxDrawing || (c >= ' ' && c <= '~'), "%s: %q", lc.name, c)
			}
		}
	}
}

func BenchmarkCompute(b *testing.B) {
	for _, tc := range []struct {
		name  string
		graph Graph
	}{
		{"fabric-4x36-196-nodes", leafHosts(4, 36, 156)},
		{"fat-tree-100-nodes", fatTree()},
		{"random-200-nodes", random(rand.New(rand.NewSource(11)), 200, 300)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				Compute(tc.graph, Options{Zoom: ZoomNormal})
			}
		})
	}
}

func BenchmarkPaint(b *testing.B) {
	g := leafHosts(4, 36, 156)
	l := Compute(g, Options{Zoom: ZoomNormal})
	st := allUp(g)

	b.ReportAllocs()

	for b.Loop() {
		l.Paint(st, Selection{Node: "leaf1", Edge: 0})
	}
}
