// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "sort"

// rowWeight is how much a row of distance counts against a column when looking for
// the nearest node in another drawing: a cell is about twice as tall as wide, and
// staying level matters more when moving sideways.
const (
	rowWeightAcross = 4
	rowWeightAlong  = 2
)

// tabOrders ranks the borders of a box by the direction an edge leaves in: top,
// left, right, bottom.
var tabOrders = map[uint8]int{dirN: 0, dirW: 1, dirE: 2, dirS: 3} //nolint:mnd // ranks

// Hit returns what is drawn at a cell. A cell inside a box belongs to the node, a
// cell on a route to its edge; where two routes cross it is the vertical one.
func (l *Layout) Hit(x, y int) Hit {
	h := Hit{Edge: -1}

	if x < 0 || y < 0 || x >= l.w || y >= l.h {
		return h
	}

	switch o := l.owner[y*l.w+x]; {
	case o > 0:
		h.Node = l.nodes[o-1].ID
	case o < 0:
		h.Edge = int(-o - 1)
	}

	return h
}

// EdgesOf returns the edges of a node in tab order: ports on the top border from
// left to right, then the side links, then the bottom border.
func (l *Layout) EdgesOf(node string) []int {
	i, ok := l.byID[node]
	if !ok {
		return nil
	}

	return append([]int(nil), l.nodes[i].tab...)
}

func (l *Layout) tabOrder() {
	type stop struct {
		side, x, far, edge int
	}

	stops := make([][]stop, len(l.nodes))

	add := func(e *wire, n, peer int, port, next pt) {
		side := tabOrders[towards(port, next)]

		far := l.nodes[peer].rect
		stops[n] = append(stops[n], stop{side, port.x, 2*far.X + far.W, e.ID})
	}

	for _, e := range l.edges {
		if len(e.path) < 2 { //nolint:mnd // unrouted
			continue
		}

		head, tail := e.a, e.b
		if !l.nodes[head].contains(e.path[0]) {
			head, tail = tail, head
		}

		add(e, head, tail, e.path[0], e.path[1])
		add(e, tail, head, e.path[len(e.path)-1], e.path[len(e.path)-2])
	}

	for n, list := range stops {
		sort.Slice(list, func(i, j int) bool {
			a, b := list[i], list[j]

			switch {
			case a.side != b.side:
				return a.side < b.side
			case a.x != b.x:
				return a.x < b.x
			case a.far != b.far:
				return a.far < b.far
			default:
				return a.edge < b.edge
			}
		})

		for _, s := range list {
			l.nodes[n].tab = append(l.nodes[n].tab, s.edge)
		}
	}
}

func (b *box) contains(p pt) bool {
	return p.x >= b.rect.X && p.x < b.rect.X+b.rect.W &&
		p.y >= b.rect.Y && p.y < b.rect.Y+b.rect.H
}

// Neighbor returns the node the focus moves to from node in direction d.
//
// Left and right step along the row. Up and down go to the adjacent row, to the
// nearest node linked to the current one if there is any, else to the nearest by
// column. At the border of a drawing the focus jumps to the closest node of another
// one in that direction, so every node can be reached with the arrow keys.
func (l *Layout) Neighbor(node string, d Dir) (string, bool) {
	i, ok := l.byID[node]
	if !ok {
		return "", false
	}

	b := l.nodes[i]
	rows := l.nav[b.unit]
	next := -1

	switch d {
	case DirLeft, DirRight:
		row := rows[b.row]
		at := sort.Search(len(row), func(k int) bool {
			return l.nodes[row[k]].rect.X >= b.rect.X
		})

		if d == DirLeft && at > 0 {
			next = row[at-1]
		}

		if d == DirRight && at+1 < len(row) {
			next = row[at+1]
		}
	case DirUp:
		if b.row > 0 {
			next = l.nearest(b, rows[b.row-1])
		}
	case DirDown:
		if b.row+1 < len(rows) {
			next = l.nearest(b, rows[b.row+1])
		}
	}

	if next < 0 {
		next = l.jump(b, d)
	}

	if next < 0 {
		return "", false
	}

	return l.nodes[next].ID, true
}

// nearest picks the node of a row closest in column to b, among its neighbors if
// the row holds any.
func (l *Layout) nearest(b *box, row []int) int {
	best, bestDist, bestLinked := -1, 0, false

	for _, n := range row {
		at := sort.SearchInts(b.adj, n)
		linked := at < len(b.adj) && b.adj[at] == n
		dist := abs(l.nodes[n].center2() - b.center2())

		if best < 0 || (linked && !bestLinked) || (linked == bestLinked && dist < bestDist) {
			best, bestDist, bestLinked = n, dist, linked
		}
	}

	return best
}

// center2 is twice the center column, which keeps half cells exact.
func (b *box) center2() int { return 2*b.rect.X + b.rect.W - 1 }

// jump finds the closest node of another drawing that lies entirely beyond b in
// direction d.
func (l *Layout) jump(b *box, d Dir) int {
	best, bestCost := -1, 0

	for _, c := range l.nodes {
		if c.unit == b.unit {
			continue
		}

		dx := abs(c.center2()-b.center2()) / 2 //nolint:mnd // back to cells
		dy := abs(c.rect.Y - b.rect.Y)

		var (
			beyond bool
			cost   int
		)

		switch d {
		case DirLeft:
			beyond, cost = c.rect.X+c.rect.W <= b.rect.X, dx+rowWeightAcross*dy
		case DirRight:
			beyond, cost = c.rect.X >= b.rect.X+b.rect.W, dx+rowWeightAcross*dy
		case DirUp:
			beyond, cost = c.rect.Y+c.rect.H <= b.rect.Y, dx+rowWeightAlong*dy
		case DirDown:
			beyond, cost = c.rect.Y >= b.rect.Y+b.rect.H, dx+rowWeightAlong*dy
		}

		if beyond && (best < 0 || cost < bestCost) {
			best, bestCost = c.idx, cost
		}
	}

	return best
}
