// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

// Direction bits of a path cell: which neighbors the line continues to.
const (
	dirN uint8 = 1 << iota
	dirE
	dirS
	dirW
)

const (
	vertical   = dirN | dirS
	horizontal = dirE | dirW
)

// route assigns ports and tracks channel by channel and derives the row positions
// from the height each channel ends up with.
func (u *unit) route() {
	rows := len(u.rows)
	u.tracks = make([]int, rows)
	u.rowY = make([]int, rows)
	u.chanY = make([]int, rows)

	y := 0
	u.left = u.rows[0][0].x

	for ch := range u.rows {
		u.assignPorts(ch)

		for _, n := range u.nets[ch] {
			n.measure()
		}

		u.tracks[ch] = assignTracks(u.nets[ch])

		// Rows are kept one line apart even when nothing runs between them.
		// Only the channel under the last row may be empty.
		height := u.tracks[ch]
		if ch+1 < rows {
			height = max(height, 1)
		}

		u.rowY[ch] = y
		u.chanY[ch] = y + u.l.spec.boxH
		y = u.chanY[ch] + height

		for _, it := range u.rows[ch] {
			u.w = max(u.w, it.x+it.w)
		}

		u.left = min(u.left, u.rows[ch][0].x)
	}

	u.h = y
}

// paths writes the route of every edge of the unit and the rectangle of every box,
// in drawing coordinates.
func (u *unit) paths() {
	l := u.l

	for _, row := range u.rows {
		for _, it := range row {
			if !it.lane() {
				l.nodes[it.node].rect = Rect{
					X: u.ox + it.x, Y: u.oy + u.rowY[it.row], W: it.w, H: l.spec.boxH,
				}
			}
		}
	}

	for _, e := range l.edges {
		switch {
		case u.direct[e.ID]:
			u.sidePath(e)
		case len(u.edgeHops[e.ID]) > 0:
			u.channelPath(e)
		}
	}
}

// sidePath joins two neighboring boxes on the row of their labels.
func (u *unit) sidePath(e *wire) {
	a, b := u.l.nodes[e.a].rect, u.l.nodes[e.b].rect
	if a.X > b.X {
		a, b = b, a
	}

	y := a.Y + (a.H-1)/2 //nolint:mnd // label row of a 1, 3 or 4 row box
	e.path = line(nil, pt{a.X + a.W - 1, y}, pt{b.X, y})
}

// leg is the part of a path inside one channel: where it starts, and where it
// meets and leaves the track.
type leg struct {
	h             *hop
	from, on, off int // on is -1 for a leg that runs straight down
}

// channelPath walks an edge through its channels: down to the track, across, and
// on down to the next row. A hop between boxes of one row comes back up instead.
func (u *unit) channelPath(e *wire) {
	hops := u.edgeHops[e.ID]
	first, last := hops[0], hops[len(hops)-1]
	cur := pt{u.ox + first.slots[0].col, u.borderY(first.ends[0], first.at[0])}
	path := []pt{cur}

	legs := make([]leg, 0, len(hops))

	for _, h := range hops {
		lg := leg{h: h, from: len(path), on: -1}

		if h.net.track >= 0 {
			y := u.oy + u.chanY[h.ch] + h.net.track
			path = line(path, cur, pt{cur.x, y})
			lg.on = len(path) - 1
			cur = pt{u.ox + h.slots[1].col, y}
			path = line(path, path[len(path)-1], cur)
			lg.off = len(path) - 1
		}

		legs = append(legs, lg)
	}

	e.path = line(path, cur, pt{cur.x, u.borderY(last.ends[1], last.at[1])})

	for i, lg := range legs {
		end := len(e.path) - 1
		if i+1 < len(legs) {
			end = legs[i+1].from
		}

		e.spans = append(e.spans, u.share(lg, end)...)
	}
}

// share lists the cells of a leg that the edge does not have to itself: the trunk
// when its net carries other edges, and the stems of ports it shares. The first
// span that covers a cell counts, so the more specific ones come first.
func (u *unit) share(lg leg, end int) []span {
	var out []span

	net := 0
	if len(lg.h.net.hops) > 1 {
		net = u.l.netID(lg.h.net)
	}

	a, b := lg.h.slots[0], lg.h.slots[1]
	keyA, keyB := u.l.stemKey(a), u.l.stemKey(b)

	// A leg that runs straight down never reaches a track. It can only be the
	// lone edge of one port moved over a column that ports share at the other
	// end, so it is part of that one stem all the way.
	if lg.on < 0 {
		if key := max(keyA, keyB); key != 0 {
			out = append(out, span{from: lg.from, to: end, stem: key})
		}

		return out
	}

	// Where ports double up on a column, the cell at which one of their lines
	// turns off is also part of the stem the others continue on.
	if keyA != 0 {
		out = append(out, span{from: lg.on, to: lg.on + 1, net: net, stem: keyA})
	}

	if keyB != 0 {
		out = append(out, span{from: lg.off, to: lg.off + 1, net: net, stem: keyB})
	}

	stem := func(sl *slot, key, from, to int) {
		switch {
		case len(sl.hops) > 1:
			out = append(out, span{from: from, to: to, net: net, port: sl, stem: key})
		case key != 0:
			out = append(out, span{from: from, to: to, stem: key})
		}
	}

	stem(a, keyA, lg.from, lg.on)
	stem(b, keyB, lg.off+1, end)

	if net != 0 {
		out = append(out, span{from: lg.on, to: lg.off + 1, net: net})
	}

	return out
}

// netID numbers the nets that carry more than one edge, from 1.
func (l *Layout) netID(n *net) int {
	if l.netIDs == nil {
		l.netIDs = map[*net]int{}
	}

	if _, ok := l.netIDs[n]; !ok {
		l.netIDs[n] = len(l.netIDs) + 1
	}

	return l.netIDs[n]
}

// stemKey identifies the border a port sits on when ports double up there, and is
// zero otherwise.
func (l *Layout) stemKey(sl *slot) int {
	if !sl.side.shared {
		return 0
	}

	if l.stemIDs == nil {
		l.stemIDs = map[*side]int{}
	}

	if _, ok := l.stemIDs[sl.side]; !ok {
		l.stemIDs[sl.side] = len(l.stemIDs) + 1
	}

	return l.stemIDs[sl.side]
}

// borderY is the row of the border an edge attaches to.
func (u *unit) borderY(it *item, at int) int {
	y := u.oy + u.rowY[it.row]
	if at == sideBottom {
		y += u.l.spec.boxH - 1
	}

	return y
}

// line appends the cells from a to b, which share a row or a column. The start cell
// is left out when the path already ends there.
func line(path []pt, a, b pt) []pt {
	if len(path) == 0 {
		path = append(path, a)
	}

	for a != b {
		switch {
		case a.x < b.x:
			a.x++
		case a.x > b.x:
			a.x--
		case a.y < b.y:
			a.y++
		default:
			a.y--
		}

		path = append(path, a)
	}

	return path
}

// towards returns the direction from one cell to an adjacent one.
// towards returns the direction from one cell to an adjacent one.
func towards(from, to pt) uint8 {
	switch {
	case to.y < from.y:
		return dirN
	case to.y > from.y:
		return dirS
	case to.x > from.x:
		return dirE
	default:
		return dirW
	}
}

// finish turns the units into the final drawing: routes, the cell ownership map and
// the tab order of each node.
func (l *Layout) finish() {
	for _, u := range l.units {
		u.paths()
	}

	for _, e := range l.edges {
		e.bits = make([]uint8, len(e.path))

		for i, p := range e.path {
			if i > 0 {
				e.bits[i] |= towards(p, e.path[i-1])
			}

			if i+1 < len(e.path) {
				e.bits[i] |= towards(p, e.path[i+1])
			}
		}
	}

	l.fillOwners()
	l.tabOrder()
}

// fillOwners records what each cell shows. Where a vertical line crosses a
// horizontal one the vertical owns the cell, as it does when painted.
func (l *Layout) fillOwners() {
	l.owner = make([]int32, l.w*l.h)

	for _, e := range l.edges {
		for i := 1; i+1 < len(e.path); i++ {
			at := e.path[i].y*l.w + e.path[i].x
			if l.owner[at] == 0 || e.bits[i] == vertical {
				l.owner[at] = -int32(e.ID + 1)
			}
		}
	}

	for _, b := range l.nodes {
		for y := b.rect.Y; y < b.rect.Y+b.rect.H; y++ {
			for x := b.rect.X; x < b.rect.X+b.rect.W; x++ {
				l.owner[y*l.w+x] = int32(b.idx + 1)
			}
		}
	}
}
