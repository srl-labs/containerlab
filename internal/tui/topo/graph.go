// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"sort"
	"strings"
)

// pt is a cell position.
type pt struct{ x, y int }

// box is a node together with everything the layout derives for it.
type box struct {
	Node
	idx   int
	label string // cleaned label, before it is fitted to the box
	text  string // label as drawn
	loops int    // links from the node to itself
	adj   []int  // neighbor node indexes, ascending
	unit  int    // index into Layout.nav
	row   int    // row within the unit
	rect  Rect
	tab   []int // edge IDs in tab order
}

// wire is a bundled edge with its route.
type wire struct {
	Edge
	a, b  int     // node indexes, a < b
	path  []pt    // every cell from one box border to the other
	bits  []uint8 // direction bits of each path cell
	spans []span  // stretches of the path shared with other edges
}

// span is a stretch of a path, from inclusive and to exclusive, that the edge
// shares with others: the trunk of a net, or the stem between a port and the trunk.
type span struct {
	from, to int
	net      int   // net whose line this is, numbered from 1; 0 if the edge's own
	port     *slot // for a stem, the port all its edges attach to
	stem     int   // key of the border column when several ports sit on it, else 0
}

// spanAt returns the span covering a path index, or nil for a cell of its own.
func (w *wire) spanAt(i int) *span {
	for k := range w.spans {
		if i >= w.spans[k].from && i < w.spans[k].to {
			return &w.spans[k]
		}
	}

	return nil
}

func newLayout(g Graph, o Options) *Layout {
	l := &Layout{opts: o, spec: specFor(o.Zoom), mode: o.Mode, byID: map[string]int{}}

	nodes := make([]Node, 0, len(g.Nodes))

	for i := range g.Nodes {
		if g.Nodes[i].ID != "" {
			nodes = append(nodes, g.Nodes[i])
		}
	}

	sort.SliceStable(nodes, func(i, j int) bool { return nodeLess(&nodes[i], &nodes[j]) })

	for i := range nodes {
		if _, dup := l.byID[nodes[i].ID]; dup {
			continue
		}

		l.byID[nodes[i].ID] = len(l.nodes)
		l.nodes = append(l.nodes, &box{Node: nodes[i], idx: len(l.nodes)})
	}

	l.rankNodes()
	l.bundle(g.Links)
	l.labelNodes()

	return l
}

// nodeLess orders nodes by ID. The remaining fields only make the choice between
// duplicate IDs independent of the input order.
func nodeLess(a, b *Node) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}

	ka := strings.Join([]string{a.Label, a.Kind, a.Role, a.Group}, "\x00")
	kb := strings.Join([]string{b.Label, b.Kind, b.Role, b.Group}, "\x00")

	switch {
	case ka != kb:
		return ka < kb
	case a.Level != b.Level:
		return a.Level < b.Level
	case a.HasPos != b.HasPos:
		return a.HasPos
	case a.X != b.X:
		return a.X < b.X
	default:
		return a.Y < b.Y
	}
}

// rankNodes records the natural name order, the tie-breaker used throughout.
func (l *Layout) rankNodes() {
	order := make([]int, len(l.nodes))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(i, j int) bool {
		return natLess(l.nodes[order[i]].ID, l.nodes[order[j]].ID)
	})

	l.rank = make([]int, len(l.nodes))
	for r, n := range order {
		l.rank[n] = r
	}
}

// bundle folds parallel links into one edge per node pair and counts self-loops.
func (l *Layout) bundle(links []Link) {
	type pair struct{ a, b int }

	byPair := map[pair]*wire{}

	for i := range links {
		a, okA := l.byID[links[i].A]
		b, okB := l.byID[links[i].B]

		if !okA || !okB {
			continue
		}

		if a == b {
			l.nodes[a].loops++
			continue
		}

		if a > b {
			a, b = b, a
		}

		w := byPair[pair{a, b}]
		if w == nil {
			w = &wire{a: a, b: b}
			w.A, w.B = l.nodes[a].ID, l.nodes[b].ID
			byPair[pair{a, b}] = w
			l.edges = append(l.edges, w)
		}

		w.Links = append(w.Links, i)
	}

	sort.Slice(l.edges, func(i, j int) bool {
		if l.edges[i].a != l.edges[j].a {
			return l.edges[i].a < l.edges[j].a
		}

		return l.edges[i].b < l.edges[j].b
	})

	for i, w := range l.edges {
		w.ID = i
		l.nodes[w.a].adj = append(l.nodes[w.a].adj, w.b)
		l.nodes[w.b].adj = append(l.nodes[w.b].adj, w.a)
	}

	for _, b := range l.nodes {
		sort.Ints(b.adj)
	}
}

// components returns the connected components, each in natural name order, largest
// first.
func (l *Layout) components() [][]int {
	seen := make([]bool, len(l.nodes))

	var comps [][]int

	for start := range l.nodes {
		if seen[start] {
			continue
		}

		comp := []int{start}
		seen[start] = true

		for head := 0; head < len(comp); head++ {
			for _, nb := range l.nodes[comp[head]].adj {
				if !seen[nb] {
					seen[nb] = true
					comp = append(comp, nb)
				}
			}
		}

		l.sortByName(comp)
		comps = append(comps, comp)
	}

	sort.SliceStable(comps, func(i, j int) bool {
		if len(comps[i]) != len(comps[j]) {
			return len(comps[i]) > len(comps[j])
		}

		return l.rank[comps[i][0]] < l.rank[comps[j][0]]
	})

	return comps
}

func (l *Layout) sortByName(nodes []int) {
	sort.Slice(nodes, func(i, j int) bool { return l.rank[nodes[i]] < l.rank[nodes[j]] })
}

// byGroup orders nodes by name while keeping the members of a group together, each
// group sitting where its first member would.
func (l *Layout) byGroup(nodes []int) []int {
	out := append([]int(nil), nodes...)
	l.sortByName(out)

	first := map[string]int{}

	for i, n := range out {
		if g := l.nodes[n].Group; g != "" {
			if _, ok := first[g]; !ok {
				first[g] = i
			}
		}
	}

	key := make(map[int]int, len(out))

	for i, n := range out {
		key[n] = i
		if g := l.nodes[n].Group; g != "" {
			key[n] = first[g]
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return key[out[i]] < key[out[j]] })

	return out
}

// natLess compares names so that digit runs order numerically: leaf2 before leaf10.
func natLess(a, b string) bool {
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			si, sj := i, j

			for i < len(a) && isDigit(a[i]) {
				i++
			}

			for j < len(b) && isDigit(b[j]) {
				j++
			}

			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")

			if len(na) != len(nb) {
				return len(na) < len(nb)
			}

			if na != nb {
				return na < nb
			}

			continue
		}

		if a[i] != b[j] {
			return a[i] < b[j]
		}

		i++
		j++
	}

	if len(a)-i != len(b)-j {
		return len(a)-i < len(b)-j
	}

	// Equal up to leading zeros: fall back to the raw strings to stay a strict order.
	return a < b
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
