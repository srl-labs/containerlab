// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type propCase struct {
	name  string
	graph Graph
}

// propCases returns the golden graphs and a spread of random ones: sparse and
// dense, with and without levels, role names and saved positions.
func propCases() []propCase {
	golden := goldenCases()
	cases := make([]propCase, 0, len(golden))

	for _, tc := range golden {
		cases = append(cases, propCase{tc.name, tc.graph})
	}

	r := rand.New(rand.NewSource(1))

	for i := range 120 {
		nodes := 2 + r.Intn(30)
		g := random(r, nodes, r.Intn(2*nodes+1))

		switch i % 4 {
		case 1:
			for n := range g.Nodes {
				g.Nodes[n].Level = r.Intn(4)
			}
		case 2:
			for n := range g.Nodes {
				if r.Intn(10) > 0 {
					g.Nodes[n].HasPos = true
					g.Nodes[n].X = float64(20 * r.Intn(40))
					g.Nodes[n].Y = float64(100*r.Intn(4) + r.Intn(30))
				}
			}
		case 3:
			roles := []string{"spine", "leaf", "client", "border", "x"}
			for n := range g.Nodes {
				g.Nodes[n].Role = roles[r.Intn(len(roles))]
			}
		}

		cases = append(cases, propCase{fmt.Sprintf("random-%d", i), g})
	}

	return append(cases,
		propCase{"triangle", ring(3)},
		propCase{"dense", random(r, 12, 60)},
		propCase{"wide", clos(4, 24)},
		propCase{"tall", clos(12, 3)},
		propCase{"fabric", leafSpine(4, 32)},
		propCase{"loners", Graph{Nodes: []Node{{ID: "a"}, {ID: "b"}, {ID: "c"}}}},
		propCase{"empty", Graph{}},
	)
}

type layoutCase struct {
	name   string
	graph  Graph
	layout *Layout
}

var (
	layoutsOnce sync.Once
	layoutCases []layoutCase
)

// layouts computes every property case at every zoom, once for all tests. Random
// graphs have no names to go by, so forcing the graph mode adds little there; the
// other graphs are laid out both ways.
func layouts() []layoutCase {
	layoutsOnce.Do(func() {
		for _, tc := range propCases() {
			modes := []Mode{ModeAuto, ModeGraph}
			if strings.HasPrefix(tc.name, "random-") {
				modes = modes[:1]
			}

			for _, zoom := range []Zoom{ZoomCompact, ZoomNormal, ZoomDetail} {
				for _, mode := range modes {
					layoutCases = append(layoutCases, layoutCase{
						name:   fmt.Sprintf("%s/zoom%d/mode%d", tc.name, zoom, mode),
						graph:  tc.graph,
						layout: Compute(tc.graph, Options{Zoom: zoom, Mode: mode}),
					})
				}
			}
		}
	})

	return layoutCases
}

func inside(r Rect, p pt) bool {
	return p.x >= r.X && p.x < r.X+r.W && p.y >= r.Y && p.y < r.Y+r.H
}

func TestBoxesDoNotOverlap(t *testing.T) {
	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			l := lc.layout
			w, h := l.Size()

			for i, a := range l.nodes {
				r := a.rect
				require.Positive(t, r.W*r.H, "node %s has no box", a.ID)
				require.True(t, r.X >= 0 && r.Y >= 0 && r.X+r.W <= w && r.Y+r.H <= h,
					"node %s lies outside the drawing: %v", a.ID, r)

				for _, b := range l.nodes[i+1:] {
					apart := r.X+r.W <= b.rect.X || b.rect.X+b.rect.W <= r.X ||
						r.Y+r.H <= b.rect.Y || b.rect.Y+b.rect.H <= r.Y
					require.True(t, apart, "%s %v overlaps %s %v", a.ID, r, b.ID, b.rect)
				}
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			l := lc.layout
			boxes := map[pt]string{}

			for _, b := range l.nodes {
				for y := b.rect.Y; y < b.rect.Y+b.rect.H; y++ {
					for x := b.rect.X; x < b.rect.X+b.rect.W; x++ {
						boxes[pt{x, y}] = b.ID
					}
				}
			}

			for _, e := range l.edges {
				n := len(e.path)
				require.GreaterOrEqual(t, n, 2, "edge %s-%s is not routed", e.A, e.B)

				for i := 1; i < n; i++ {
					dx, dy := abs(e.path[i].x-e.path[i-1].x), abs(e.path[i].y-e.path[i-1].y)
					require.Equal(t, 1, dx+dy, "edge %s-%s jumps at step %d", e.A, e.B, i)
				}

				// Both ends sit on the border of the two nodes, one each.
				a, b := l.nodes[e.a].rect, l.nodes[e.b].rect
				head, tail := e.path[0], e.path[n-1]
				joined := (inside(a, head) && inside(b, tail)) ||
					(inside(b, head) && inside(a, tail))
				require.True(t, joined, "edge %s-%s does not join its nodes", e.A, e.B)

				for _, end := range []pt{head, tail} {
					for _, r := range []Rect{a, b} {
						if inside(r, end) && r.H > 2 {
							border := end.x == r.X || end.x == r.X+r.W-1 ||
								end.y == r.Y || end.y == r.Y+r.H-1
							require.True(t, border, "edge %s-%s ends inside a box", e.A, e.B)
						}
					}
				}

				// In between, a route stays clear of every box.
				for _, p := range e.path[1 : n-1] {
					if id, hit := boxes[p]; hit {
						t.Fatalf("edge %s-%s runs through %s at %v", e.A, e.B, id, p)
					}
				}
			}
		})
	}
}

// TestRoutesDoNotMerge checks that two edges share a cell only where they cross at
// a right angle, run along the line of one net, or leave a box by the same stem.
func TestRoutesDoNotMerge(t *testing.T) {
	type use struct {
		edge *wire
		at   int
	}

	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			cells := map[pt][]use{}

			for _, e := range lc.layout.edges {
				for i := 1; i+1 < len(e.path); i++ {
					cells[e.path[i]] = append(cells[e.path[i]], use{e, i})
				}
			}

			for p, uses := range cells {
				for i, a := range uses {
					for _, b := range uses[i+1:] {
						ab, bb := a.edge.bits[a.at], b.edge.bits[b.at]
						crossing := (ab == vertical && bb == horizontal) ||
							(ab == horizontal && bb == vertical)
						sa, sb := a.edge.spanAt(a.at), b.edge.spanAt(b.at)
						shared := sa != nil && sb != nil &&
							((sa.net > 0 && sa.net == sb.net) || (sa.stem > 0 && sa.stem == sb.stem))

						require.True(t, crossing || shared, "edges %s-%s and %s-%s merge at %v",
							a.edge.A, a.edge.B, b.edge.A, b.edge.B, p)
					}
				}
			}
		})
	}
}

// TestPortColumns checks the rule that keeps verticals apart: unless an edge runs
// straight through a channel, it leaves a box downwards on a multiple of the port
// step and arrives upwards half a step off, and lanes keep to the columns between.
func TestPortColumns(t *testing.T) {
	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			spec := lc.layout.spec

			for _, u := range lc.layout.units {
				for _, hops := range u.hops {
					for _, h := range hops {
						for i, sl := range h.slots {
							col := sl.col

							switch {
							case h.ends[i].lane():
								require.True(t, spec.laneColumn(col),
									"lane of edge %d on column %d", h.edge, col)
							case h.slots[0].col != h.slots[1].col:
								require.True(t, spec.portColumn(col, h.at[i] == sideTop),
									"edge %d uses column %d on side %d", h.edge, col, h.at[i])
							}
						}
					}
				}
			}
		})
	}
}

// TestCompactIsCompact checks what the compact zoom is for: a box is its label in
// brackets, and a drawing is no wider than its widest row needs with one blank
// between boxes. A lane, which a row may have to make room for, counts as a box
// with a label of the port step.
func TestCompactIsCompact(t *testing.T) {
	for _, lc := range layouts() {
		l := lc.layout
		if l.opts.Zoom != ZoomCompact {
			continue
		}

		t.Run(lc.name, func(t *testing.T) {
			longest := 0

			for _, b := range l.nodes {
				label := utf8.RuneCountInString(b.label)
				longest = max(longest, label)

				require.Equal(
					t,
					max(min(label, l.spec.maxLabel)+2, l.spec.minW),
					b.rect.W,
					"box of %s",
					b.ID,
				)
				require.Equal(t, 1, b.rect.H)
				require.LessOrEqual(t, b.rect.W, longest+2)

				if label <= l.spec.maxLabel {
					require.Equal(t, b.label, b.text, "label of %s was shortened", b.ID)
				}
			}

			for _, u := range l.units {
				if u.keyed {
					// Saved positions keep their proportions, gaps included.
					continue
				}

				bound := 0

				for _, row := range u.rows {
					need := 0

					for _, it := range row {
						need += 3
						if it.lane() {
							need += l.spec.step
						} else {
							need += utf8.RuneCountInString(l.nodes[it.node].text)
						}
					}

					bound = max(bound, need)
				}

				require.LessOrEqual(t, u.w-u.left, bound, "unit is wider than its widest row")
			}
		})
	}
}

func TestShuffledInputGivesSameDrawing(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	sel := Selection{Edge: -1}

	for _, tc := range propCases() {
		for _, zoom := range []Zoom{ZoomCompact, ZoomNormal} {
			t.Run(fmt.Sprintf("%s/zoom%d", tc.name, zoom), func(t *testing.T) {
				o := Options{Zoom: zoom}
				want := Compute(tc.graph, o)

				for range 3 {
					got := Compute(shuffled(tc.graph, r), o)
					require.Equal(t, want.Mode(), got.Mode())
					require.Equal(
						t,
						want.Paint(State{}, sel).Lines(),
						got.Paint(State{}, sel).Lines(),
					)

					for _, n := range want.nodes {
						rect, ok := got.NodeRect(n.ID)
						require.True(t, ok)
						require.Equal(t, n.rect, rect)
						require.Len(t, got.EdgesOf(n.ID), len(want.EdgesOf(n.ID)))
					}
				}
			})
		}
	}
}

func TestEveryNodeIsReachableByKeyboard(t *testing.T) {
	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			l := lc.layout

			for _, start := range l.nodes {
				seen := map[string]bool{start.ID: true}
				queue := []string{start.ID}

				for len(queue) > 0 {
					cur := queue[0]
					queue = queue[1:]

					for _, d := range []Dir{DirLeft, DirRight, DirUp, DirDown} {
						next, ok := l.Neighbor(cur, d)
						if !ok {
							continue
						}

						_, known := l.NodeRect(next)
						require.True(t, known, "Neighbor returned unknown node %q", next)
						require.NotEqual(t, cur, next)

						if !seen[next] {
							seen[next] = true
							queue = append(queue, next)
						}
					}
				}

				require.Len(t, seen, len(l.nodes), "not every node is reachable from %s", start.ID)
			}
		})
	}
}

// TestNeighborFollowsRows checks the documented moves inside one drawing: left and
// right walk a row in order, up and down prefer a linked node.
func TestNeighborFollowsRows(t *testing.T) {
	l := Compute(threeTier(), Options{Zoom: ZoomNormal})

	for _, step := range []struct {
		from string
		dir  Dir
		to   string
	}{
		{"leaf1", DirRight, "leaf2"},
		{"leaf3", DirLeft, "leaf2"},
		{"leaf2", DirDown, "client2"},
		{"client4", DirUp, "leaf4"},
		{"leaf1", DirUp, "spine1"},
		{"spine2", DirLeft, "spine1"},
	} {
		got, ok := l.Neighbor(step.from, step.dir)
		assert.True(t, ok)
		assert.Equal(t, step.to, got, "from %s in direction %d", step.from, step.dir)
	}

	_, ok := l.Neighbor("spine1", DirUp)
	assert.False(t, ok, "nothing lies above the top row")

	_, ok = l.Neighbor("nobody", DirUp)
	assert.False(t, ok)
}

func TestHit(t *testing.T) {
	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			l := lc.layout

			for _, b := range l.nodes {
				for y := b.rect.Y; y < b.rect.Y+b.rect.H; y++ {
					for x := b.rect.X; x < b.rect.X+b.rect.W; x++ {
						require.Equal(t, Hit{Node: b.ID, Edge: -1}, l.Hit(x, y))
					}
				}
			}

			// A route cell reports an edge that runs there, which leaves no
			// choice where only one does.
			covers := map[pt]map[int]bool{}

			for _, e := range l.edges {
				for _, p := range e.path[1 : len(e.path)-1] {
					if covers[p] == nil {
						covers[p] = map[int]bool{}
					}

					covers[p][e.ID] = true
				}
			}

			for p, edges := range covers {
				h := l.Hit(p.x, p.y)
				require.Empty(t, h.Node)
				require.True(t, edges[h.Edge],
					"cell %v reports edge %d, which does not run there", p, h.Edge)
			}

			w, h := l.Size()
			assert.Equal(t, Hit{Edge: -1}, l.Hit(-1, 0))
			assert.Equal(t, Hit{Edge: -1}, l.Hit(w, h))
		})
	}
}

// TestPaintMatchesHit checks that what is painted agrees with what Hit reports:
// node cells carry a node class, edge cells a link class, and nothing is drawn on
// the cells that belong to neither except the badge of a bundle.
func TestPaintMatchesHit(t *testing.T) {
	nodeClasses := []Class{ClassNodeBorder, ClassNodeLabel, ClassNodeNote, ClassBadge}
	edgeClasses := []Class{ClassLinkUp, ClassLinkUnknown, ClassBadge, ClassBlank}

	for _, lc := range layouts() {
		t.Run(lc.name, func(t *testing.T) {
			l := lc.layout
			grid := l.Paint(allUp(lc.graph), Selection{Edge: -1})
			w, h := l.Size()
			gw, gh := grid.Size()

			require.Equal(t, w, gw)
			require.Equal(t, h, gh)

			for y := range h {
				for x := range w {
					hit, cell := l.Hit(x, y), grid.Cell(x, y)

					switch {
					case hit.Node != "":
						require.Contains(t, nodeClasses, cell.Class)
					case hit.Edge >= 0:
						require.Contains(t, edgeClasses, cell.Class, "cell %d,%d", x, y)
					case cell.Class != ClassBadge:
						require.Equal(t, Cell{Rune: ' '}, cell, "stray glyph at %d,%d", x, y)
					}
				}
			}
		})
	}
}
