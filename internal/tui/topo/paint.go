// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"slices"
	"sort"
)

const (
	// loopMark flags a node that has a link to itself. Such a link has no route,
	// so the mark takes a corner of the box, where no port can be.
	loopMark = '@'
	// noteRow is the row of a detail box that holds the note, under the label.
	noteRow = 2
)

// stroke is one cell of a line while painting. Strokes of one owner merge into
// junctions; strokes of different owners cross.
type stroke struct {
	owner int32 // 0 none, e+1 for edge e, -n for the shared line of net n
	stem  int32 // key of the border column when ports double up on it, else 0
	bits  uint8
	w     weight
}

type painter struct {
	l   *Layout
	g   *Grid
	st  State
	sel Selection
	ink []stroke
	// joined lists, for the few cells on a stem that several nets run down,
	// the owners merged there besides the first.
	joined map[int][]int32
}

// Paint draws the layout with the given state and focus.
func (l *Layout) Paint(st State, sel Selection) *Grid {
	p := &painter{
		l: l, g: newGrid(l.w, l.h), st: st, sel: sel,
		ink: make([]stroke, l.w*l.h), joined: map[int][]int32{},
	}

	for _, b := range l.nodes {
		p.box(b)
	}

	// Later strokes win where routes had to overlap, so the order is by how much
	// the reader needs to see them: unknown, up, mixed, down, then the focus.
	order := append([]*wire(nil), l.edges...)
	sort.SliceStable(
		order,
		func(i, j int) bool { return p.urgency(order[i]) < p.urgency(order[j]) },
	)

	for _, e := range order {
		p.edge(e)
	}

	for _, e := range order {
		p.ports(e)
	}

	p.labels()
	p.endLabels()

	return p.g
}

func (p *painter) urgency(e *wire) int {
	if e.ID == p.sel.Edge {
		return int(LinkMixed) + 2 //nolint:mnd // above every state
	}

	switch p.state(e) {
	case LinkUp:
		return 1
	case LinkMixed:
		return 2 //nolint:mnd // between up and down
	case LinkDown:
		return 3 //nolint:mnd // on top of the healthy links
	default:
		return 0
	}
}

// state aggregates the member links of an edge.
func (p *painter) state(e *wire) LinkState { return p.states(e) }

// states aggregates the links of several edges: one state if they all agree, mixed
// otherwise.
func (p *painter) states(edges ...*wire) LinkState {
	var seen [LinkMixed + 1]bool

	for _, e := range edges {
		for _, i := range e.Links {
			s := p.st.Links[i]
			if s < LinkUnknown || s > LinkMixed {
				s = LinkUnknown
			}

			seen[s] = true
		}
	}

	switch {
	case seen[LinkMixed], seen[LinkUp] && seen[LinkDown],
		seen[LinkUnknown] && (seen[LinkUp] || seen[LinkDown]):
		return LinkMixed
	case seen[LinkUp]:
		return LinkUp
	case seen[LinkDown]:
		return LinkDown
	default:
		return LinkUnknown
	}
}

// lineStyle picks the line for a link state. State is never shown by color alone:
// a down link is dashed and an unknown one dotted.
func lineStyle(s LinkState) (weight, Class) {
	switch s {
	case LinkUp:
		return weightLight, ClassLinkUp
	case LinkDown:
		return weightDashed, ClassLinkDown
	case LinkMixed:
		return weightLight, ClassLinkMixed
	default:
		return weightDotted, ClassLinkUnknown
	}
}

// style picks the line for an edge. A bundle is drawn double unless that would
// hide a dashed or dotted line; the badge still tells it is one.
func (p *painter) style(e *wire) (weight, Class) {
	w, class := lineStyle(p.state(e))

	if w == weightLight && len(e.Links) > 1 {
		w = weightDouble
	}

	if e.ID == p.sel.Edge {
		w = weightHeavy
	}

	return w, class
}

func (p *painter) box(b *box) {
	r := b.rect
	right, bottom := r.X+r.W-1, r.Y+r.H-1
	inner := r.W - 2 //nolint:mnd // without the frame
	fill := Cell{
		Rune: ' ', Class: ClassNodeLabel, State: p.st.Nodes[b.ID], Selected: p.sel.Node == b.ID,
	}

	for y := r.Y; y <= bottom; y++ {
		for x := r.X; x <= right; x++ {
			*p.g.at(x, y) = fill
		}
	}

	w := weightLight
	if fill.Selected {
		w = weightHeavy
	}

	border := func(x, y int, glyph rune) {
		c := p.g.at(x, y)
		c.Rune, c.Class = glyph, ClassNodeBorder
	}
	draw := func(x, y int, bits uint8) { border(x, y, lineRune(w, bits, p.l.opts.ASCII)) }

	if r.H == 1 {
		// A one-row node has no room for a frame; brackets mark its extent.
		border(r.X, r.Y, '[')
		border(right, r.Y, ']')
		p.text(r.X+1, r.Y, inner, 0, b.text, ClassNodeLabel)
	} else {
		for x := r.X + 1; x < right; x++ {
			draw(x, r.Y, horizontal)
			draw(x, bottom, horizontal)
		}

		for y := r.Y + 1; y < bottom; y++ {
			draw(r.X, y, vertical)
			draw(right, y, vertical)
		}

		draw(r.X, r.Y, dirE|dirS)
		draw(right, r.Y, dirW|dirS)
		draw(r.X, bottom, dirN|dirE)
		draw(right, bottom, dirN|dirW)
		p.text(r.X+1, r.Y+1, inner, 1, b.text, ClassNodeLabel)
	}

	if r.H > noteRow+1 {
		note := p.st.NodeNotes[b.ID]
		if note == "" {
			note = b.Kind
		}

		note = clip(clean(note, p.l.opts.ASCII), inner-2) //nolint:mnd // a blank each side
		p.text(r.X+1, r.Y+noteRow, inner, 1, note, ClassNodeNote)
	}

	if b.loops > 0 {
		border(right, r.Y, loopMark)
		p.g.at(right, r.Y).Class = ClassBadge
	}
}

// text writes s centered in a field, shortened to leave margin blanks at both ends.
func (p *painter) text(x, y, width, margin int, s string, class Class) {
	r := []rune(fit(s, width-2*margin))
	x += (width - len(r)) / 2 //nolint:mnd // center

	for i, c := range r {
		cell := p.g.at(x+i, y)
		cell.Rune, cell.Class = c, class
	}
}

func (p *painter) edge(e *wire) {
	w, class := p.style(e)
	sel := e.ID == p.sel.Edge

	for i := 1; i+1 < len(e.path); i++ {
		s, cc := stroke{owner: int32(e.ID + 1), bits: e.bits[i], w: w}, class

		if sp := e.spanAt(i); sp != nil {
			s.stem = int32(sp.stem)

			if sp.net > 0 {
				s.owner = -int32(sp.net)
				s.w, cc = p.shared(sp)
			}

			if sel {
				s.w = weightHeavy
			}
		}

		p.stroke(e.path[i], s, cc, sel)
	}
}

// shared picks the line for a cell several edges run through. A trunk carries
// links that may differ in state, so it shows none; the stem of a port shows the
// state its edges have in common.
func (p *painter) shared(sp *span) (weight, Class) {
	if sp.port == nil {
		return weightLight, ClassLinkUnknown
	}

	edges := make([]*wire, len(sp.port.hops))
	for i, h := range sp.port.hops {
		edges[i] = p.l.edges[h.edge]
	}

	return lineStyle(p.states(edges...))
}

// stroke draws one cell of a line. Where a vertical and a horizontal line of
// different owners meet, the vertical one stays: a crossing then reads as a bridge
// and cannot be mistaken for a junction.
func (p *painter) stroke(at pt, s stroke, class Class, sel bool) {
	cell := at.y*p.l.w + at.x
	ink := &p.ink[cell]
	c := p.g.at(at.x, at.y)

	switch {
	case p.same(cell, ink, s):
		s.owner = ink.owner
		s.stem = max(s.stem, ink.stem)
		s.bits |= ink.bits
		s.w = max(s.w, ink.w)
		sel = sel || c.Selected
	case ink.owner != 0 && ink.bits == vertical && s.bits == horizontal:
		return
	}

	*ink = s
	*c = Cell{Rune: lineRune(s.w, s.bits, p.l.opts.ASCII), Class: class, Selected: sel}
}

// same reports whether a stroke belongs to the line already in a cell: it has the
// same owner, or both are stems of ports doubled up on one column.
func (p *painter) same(cell int, ink *stroke, s stroke) bool {
	if ink.owner == 0 {
		return false
	}

	if ink.owner == s.owner || slices.Contains(p.joined[cell], s.owner) {
		return true
	}

	if s.stem == 0 || s.stem != ink.stem {
		return false
	}

	p.joined[cell] = append(p.joined[cell], s.owner)

	return true
}

func (p *painter) ports(e *wire) {
	n := len(e.path)
	if n < 2 { //nolint:mnd // unrouted
		return
	}

	w, _ := p.style(e)
	p.port(e.path[0], e.path[1], w, e.ID == p.sel.Edge)
	p.port(e.path[n-1], e.path[n-2], w, e.ID == p.sel.Edge)
}

// port turns the border cell an edge attaches to into a tee.
func (p *painter) port(at, next pt, w weight, sel bool) {
	owner := p.l.owner[at.y*p.l.w+at.x]
	if owner <= 0 {
		return
	}

	b := p.l.nodes[owner-1]
	c := p.g.at(at.x, at.y)
	c.Selected = c.Selected || sel

	if b.rect.H == 1 {
		return
	}

	// Several edges may share a port; the strongest stem decides.
	ink := &p.ink[at.y*p.l.w+at.x]
	ink.w = max(ink.w, w)

	frame := weightLight
	if p.sel.Node == b.ID {
		frame = weightHeavy
	}

	c.Rune = portRune(towards(at, next), frame, ink.w, p.l.opts.ASCII)
}
