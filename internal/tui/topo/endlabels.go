// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "sort"

const (
	// maxEndLabel caps the text written at one end of an edge. Interface names fit.
	maxEndLabel = 12
	// markAbove and markBelow follow a name that sits level with both of its nodes,
	// in a gap of a single row, and point at the one it belongs to.
	markAbove, markBelow           = '▴', '▾'
	markAboveASCII, markBelowASCII = '^', 'v'
)

// stretch is a straight run of a route that shows nothing but its own edge, with its
// place on the path.
type stretch struct {
	run
	first, last int  // path indexes of its first and last cell
	across      bool // horizontal
}

// endLabels writes the caller's texts for the two ends of every edge, each as close
// to its own node as there is room for. Room is scarce, so the ends go in the order
// of their use to the reader: those of the focused edge, those at the focused node,
// then the rest. An end with no free spot stays bare, and so does one that could be
// taken for the end of another edge.
func (p *painter) endLabels() {
	if len(p.st.EndLabels) == 0 {
		return
	}

	type job struct {
		e     *wire
		text  string
		start bool
		rank  int
	}

	var jobs []job

	for _, e := range p.l.edges {
		names, ok := p.st.EndLabels[e.ID]
		if !ok || len(e.path) < 2 { // unrouted
			continue
		}

		// The path runs from either node to the other; the names follow it.
		nodes := [2]int{e.a, e.b}
		if !p.startsAt(e, e.a) {
			names[0], names[1] = names[1], names[0]
			nodes[0], nodes[1] = nodes[1], nodes[0]
		}

		for i, name := range names {
			rank := 2 // after the focus

			switch {
			case e.ID == p.sel.Edge:
				rank = 0
			case p.l.nodes[nodes[i]].ID == p.sel.Node:
				rank = 1
			}

			jobs = append(jobs, job{e: e, text: name, start: i == 0, rank: rank})
		}
	}

	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].rank < jobs[j].rank })

	stretches := map[int][]stretch{}

	for _, it := range jobs {
		list, ok := stretches[it.e.ID]
		if !ok {
			list = p.stretches(it.e)
			stretches[it.e.ID] = list
		}

		p.endLabel(it.e, list, it.text, it.start)
	}
}

// startsAt reports whether the path of an edge leaves from the given node.
func (p *painter) startsAt(e *wire, node int) bool {
	at := e.path[0]

	return int(p.l.owner[at.y*p.l.w+at.x]) == node+1
}

// stretches collects the straight runs of a route where the cells show this edge
// alone, not a line it shares and not a crossing, in the order of the path.
func (p *painter) stretches(e *wire) []stretch {
	var out []stretch

	open := false

	for i := 1; i+1 < len(e.path); i++ {
		at := e.path[i]
		ink := p.ink[at.y*p.l.w+at.x]
		straight := e.bits[i] == horizontal || e.bits[i] == vertical

		if !straight || e.spanAt(i) != nil || ink.owner != int32(e.ID+1) || ink.bits != e.bits[i] {
			open = false
			continue
		}

		if open && e.bits[i] == e.bits[i-1] {
			s := &out[len(out)-1]
			s.from = pt{min(s.from.x, at.x), min(s.from.y, at.y)}
			s.to = pt{max(s.to.x, at.x), max(s.to.y, at.y)}
			s.last = i

			continue
		}

		out = append(out, stretch{
			run: run{at, at}, first: i, last: i, across: e.bits[i] == horizontal,
		})
		open = true
	}

	return out
}

// shared reports whether any path cell in [from, to) is a line the edge shares.
func (w *wire) shared(from, to int) bool {
	for i := from; i < to; i++ {
		if w.spanAt(i) != nil {
			return true
		}
	}

	return false
}

// endLabel writes the text for the end of an edge where its path starts, or for the
// other one. It walks the stretches from that end and takes the first with room.
func (p *painter) endLabel(e *wire, stretches []stretch, text string, start bool) {
	text = fit(clean(text, p.l.opts.ASCII), maxEndLabel)
	if text == "" {
		return
	}

	cells := appendText(nil, text, ClassEndLabel)
	for i := range cells {
		cells[i].Selected = e.ID == p.sel.Edge
	}

	last := len(e.path) - 1

	for k := range stretches {
		s := stretches[k]
		if !start {
			s = stretches[len(stretches)-1-k]
		}

		// A name is only as good as the line that leads from it to its node: past a
		// line shared with other edges it would read as theirs.
		if (start && e.shared(1, s.first)) || (!start && e.shared(s.last+1, last)) {
			return
		}

		at := end{e: e, s: s, start: start}

		if s.across && p.endAcross(at, cells) {
			return
		}

		if !s.across && p.endBeside(at, p.marked(cells, at)) {
			return
		}
	}
}

// end is a stretch of an edge seen from one of its two ends.
type end struct {
	e     *wire
	s     stretch
	start bool // the end where the path starts
}

// near returns the cell of the stretch closest to the end, and node the cell of the
// border the edge attaches to there.
func (a end) near() pt {
	if a.start {
		return a.e.path[a.s.first]
	}

	return a.e.path[a.s.last]
}

func (a end) node() pt {
	if a.start {
		return a.e.path[0]
	}

	return a.e.path[len(a.e.path)-1]
}

// closer reports whether a text over the cells from one position of the stretch to
// another is closer to this end than to the other one, counted along the route. A
// text further on would read as the name of the far end. With level set, a text
// that is as far from both will do: it carries a mark.
func (a end) closer(from, to pt, level bool) bool {
	first := a.e.path[a.s.first]
	i := a.s.first + abs(from.x-first.x) + abs(from.y-first.y)
	j := a.s.first + abs(to.x-first.x) + abs(to.y-first.y)
	own, other := min(i, j), len(a.e.path)-1-max(i, j)

	if !a.start {
		own, other = other, own
	}

	return own < other || (level && own == other)
}

// marked returns the text for a vertical stretch. On a stretch of a single row the
// place of a name cannot tell which node it belongs to, so a mark pointing at the
// node is added.
func (p *painter) marked(cells []Cell, a end) []Cell {
	near, node := a.near(), a.node()
	if a.s.length() > 1 || node.y == near.y {
		return cells
	}

	mark := markAbove
	if node.y > near.y {
		mark = markBelow
	}

	if p.l.opts.ASCII {
		mark = markAboveASCII
		if node.y > near.y {
			mark = markBelowASCII
		}
	}

	out := append(make([]Cell, 0, len(cells)+1), cells...)

	return append(out, Cell{Rune: mark, Class: ClassEndLabel, Selected: cells[0].Selected})
}

// endAcross writes the text into a horizontal stretch, at the end of it that is near.
// A piece of the line is kept between that end and the text where there is room.
func (p *painter) endAcross(a end, cells []Cell) bool {
	s, near := a.s, a.near()
	room := len(cells) + 2 //nolint:mnd // a blank on both sides
	y := s.from.y

	for _, lead := range []int{1, 0} {
		// Something of the line has to remain, or the text would cut it in two.
		if s.length() < room+lead+1 {
			continue
		}

		x := s.from.x + lead
		if near.x != s.from.x {
			x = s.to.x - lead - room + 1
		}

		// The line has to go on behind the text, or two texts back to back would
		// leave nothing of it between them.
		behind := x + room
		if near.x != s.from.x {
			behind = x - 1
		}

		if !a.closer(pt{x, y}, pt{x + room - 1, y}, false) ||
			!p.line(x, y, room) || !p.line(behind, y, 1) {
			continue
		}

		p.g.at(x, y).Rune = ' '
		p.g.at(x+room-1, y).Rune = ' '
		p.write(x+1, y, cells)

		return true
	}

	return false
}

// line reports whether n cells from x still show the line of an edge: no text has
// been written over them.
func (p *painter) line(x, y, n int) bool {
	for i := range n {
		if !p.g.has(x+i, y) {
			return false
		}

		c := p.g.at(x+i, y)

		switch c.Class {
		case ClassLinkUp, ClassLinkDown, ClassLinkMixed, ClassLinkUnknown:
			if c.Rune == ' ' {
				return false
			}
		default:
			return false
		}
	}

	return true
}

// endBeside writes the text next to the vertical stretch an edge leaves its node on,
// on the free row closest to the node. Further along the route a text beside the line
// is too far from its node to be read as its name, so other stretches are left alone.
func (p *painter) endBeside(a end, cells []Cell) bool {
	s, near := a.s, a.near()

	if (a.start && s.first != 1) ||
		(!a.start && s.last != len(a.e.path)-2) { // next to the border
		return false
	}

	for d := range s.length() {
		y := s.from.y + d
		if near.y != s.from.y {
			y = s.to.y - d
		}

		if !a.closer(pt{s.from.x, y}, pt{s.from.x, y}, s.length() == 1) {
			return false
		}

		for _, right := range []bool{true, false} {
			if x, ok := p.spotBeside(s.from.x, y, len(cells), right); ok {
				p.write(x, y, cells)

				return true
			}
		}
	}

	return false
}

// spotBeside returns where a text of n cells goes next to the line at a position: one
// blank away from it, and two blanks away from whatever follows on the other side, so
// that it is seen to belong to this line and not to the next one.
func (p *painter) spotBeside(lineX, y, n int, right bool) (int, bool) {
	x := lineX + 2 //nolint:mnd // a blank between line and text
	if !right {
		x = lineX - 1 - n
	}

	if !p.g.has(x, y) || !p.g.has(x+n-1, y) {
		return 0, false
	}

	for i := -1; i <= n; i++ {
		if !p.vacant(x+i, y) {
			return 0, false
		}
	}

	// The second blank on the side away from the line.
	far := x + n + 1
	if !right {
		far = x - 2 //nolint:mnd // see above
	}

	return x, p.vacant(far, y)
}

// vacant reports whether nothing is drawn at a position; past the edge of the grid
// nothing is.
func (p *painter) vacant(x, y int) bool {
	return !p.g.has(x, y) || p.g.blank(x, y)
}
