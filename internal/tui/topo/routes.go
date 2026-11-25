// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

// Pos is a cell of a drawing.
type Pos struct{ X, Y int }

// Route returns the corners of the line of an edge, from the border of the box of Edge.A
// or Edge.B to the border of the other one. It is what a caller that draws the links
// itself, rather than with the characters of Paint, needs to know. An edge that could
// not be routed has no corners.
func (l *Layout) Route(id int) []Pos {
	e := l.wire(id)
	if e == nil || len(e.path) < 2 { // unrouted
		return nil
	}

	route := []Pos{{X: e.path[0].x, Y: e.path[0].y}}

	for i := 1; i+1 < len(e.path); i++ {
		prev, at, next := e.path[i-1], e.path[i], e.path[i+1]

		// Only the cells where the line turns are corners.
		if (at.x-prev.x) != (next.x-at.x) || (at.y-prev.y) != (next.y-at.y) {
			route = append(route, Pos{X: at.x, Y: at.y})
		}
	}

	last := e.path[len(e.path)-1]

	return append(route, Pos{X: last.x, Y: last.y})
}

// EdgeState returns the state the links of an edge have in common.
func (l *Layout) EdgeState(st State, id int) LinkState {
	e := l.wire(id)
	if e == nil {
		return LinkUnknown
	}

	p := painter{l: l, st: st}

	return p.state(e)
}

// Bundled reports whether an edge stands for more than one link.
func (l *Layout) Bundled(id int) bool {
	e := l.wire(id)

	return e != nil && len(e.Links) > 1
}

func (l *Layout) wire(id int) *wire {
	if id >= 0 && id < len(l.edges) && l.edges[id].ID == id {
		return l.edges[id]
	}

	for _, e := range l.edges {
		if e.ID == id {
			return e
		}
	}

	return nil
}
