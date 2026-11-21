// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "strconv"

// maxEdgeLabel caps the text shown along an edge. Rates and interface names fit;
// anything longer belongs in a detail view.
const maxEdgeLabel = 16

// run is a straight stretch of a route that shows nothing but its own edge.
type run struct {
	from, to pt // inclusive, from is the lower coordinate
}

func (r run) length() int { return r.to.x - r.from.x + r.to.y - r.from.y + 1 }

// labels writes the bundle badges and the caller's edge labels. The focused edge
// goes first so that it gets the best spot; an edge with no free spot stays bare.
func (p *painter) labels() {
	order := make([]*wire, 0, len(p.l.edges))

	for _, e := range p.l.edges {
		if e.ID == p.sel.Edge {
			order = append(order, e)
		}
	}

	for _, e := range p.l.edges {
		if e.ID != p.sel.Edge {
			order = append(order, e)
		}
	}

	for _, e := range order {
		var cells []Cell

		if len(e.Links) > 1 {
			cells = appendText(cells, strconv.Itoa(len(e.Links))+"x", ClassBadge)
		}

		if s := fit(clean(p.st.EdgeLabels[e.ID], p.l.opts.ASCII), maxEdgeLabel); s != "" {
			if len(cells) > 0 {
				cells = append(cells, Cell{Rune: ' '})
			}

			cells = appendText(cells, s, ClassEdgeLabel)
		}

		if len(cells) > 0 {
			p.label(e, cells)
		}
	}
}

func appendText(cells []Cell, s string, class Class) []Cell {
	for _, r := range s {
		cells = append(cells, Cell{Rune: r, Class: class})
	}

	return cells
}

// label puts text on an edge: in the middle of its longest horizontal stretch,
// else beside a vertical one or a corner, the last before the first because labels
// read best next to the lower node.
func (p *painter) label(e *wire, cells []Cell) {
	for i := range cells {
		cells[i].Selected = e.ID == p.sel.Edge
	}

	across, down, bends := p.runs(e)
	longest := run{to: pt{-1, 0}}

	for _, r := range across {
		if r.length() > longest.length() {
			longest = r
		}
	}

	// The text replaces a stretch of the line, with a blank at each end where
	// there is room for one.
	for _, blanks := range []int{1, 0} {
		room := len(cells) + 2*blanks //nolint:mnd // both ends
		if longest.length() < room {
			continue
		}

		x := longest.from.x + (longest.length()-room)/2 //nolint:mnd // center

		for i := range room {
			p.g.at(x+i, longest.from.y).Rune = ' '
		}

		p.write(x+blanks, longest.from.y, cells)

		return
	}

	for _, spots := range [][]run{down, bends} {
		if len(spots) == 0 {
			continue
		}

		first, last := spots[0], spots[len(spots)-1]

		for _, try := range []struct {
			r     run
			right bool
		}{{last, true}, {first, true}, {last, false}, {first, false}} {
			if p.beside(try.r, try.right, cells) {
				return
			}
		}
	}
}

// runs collects the stretches of a route where the cell shows this edge alone, not
// a line it shares and not a crossing: horizontal ones, vertical ones and corners.
func (p *painter) runs(e *wire) (across, down, bends []run) {
	var cur *run

	for i := 1; i+1 < len(e.path); i++ {
		at := e.path[i]
		ink := p.ink[at.y*p.l.w+at.x]

		if e.spanAt(i) != nil || ink.owner != int32(e.ID+1) || ink.bits != e.bits[i] {
			cur = nil
			continue
		}

		list := &bends

		switch e.bits[i] {
		case horizontal:
			list = &across
		case vertical:
			list = &down
		}

		if cur != nil && list != &bends && e.bits[i] == e.bits[i-1] {
			cur.from = pt{min(cur.from.x, at.x), min(cur.from.y, at.y)}
			cur.to = pt{max(cur.to.x, at.x), max(cur.to.y, at.y)}

			continue
		}

		*list = append(*list, run{at, at})
		cur = &(*list)[len(*list)-1]
	}

	return across, down, bends
}

// beside writes the text next to a vertical stretch, on the first row where the
// cells are free and the text would not touch whatever follows.
func (p *painter) beside(r run, right bool, cells []Cell) bool {
	mid := (r.from.y + r.to.y) / 2 //nolint:mnd // start at the middle row

	for d := 0; d <= r.to.y-r.from.y; d++ {
		for _, y := range []int{mid - d, mid + d} {
			if y < r.from.y || y > r.to.y {
				continue
			}

			x := r.from.x + 2 //nolint:mnd // one blank between line and text
			if !right {
				x = r.from.x - 1 - len(cells)
			}

			if p.free(x-1, y, len(cells)+2) { //nolint:mnd // a blank on both sides
				p.write(x, y, cells)
				return true
			}
		}
	}

	return false
}

// free reports whether n cells from x are blank. Cells past the edge of the grid
// count as blank only for the margin, never for the text itself.
func (p *painter) free(x, y, n int) bool {
	for i := range n {
		inside := p.g.has(x+i, y)
		margin := i == 0 || i == n-1

		if (!inside && !margin) || (inside && !p.g.blank(x+i, y)) {
			return false
		}
	}

	return true
}

func (p *painter) write(x, y int, cells []Cell) {
	for i, c := range cells {
		*p.g.at(x+i, y) = c
	}
}
