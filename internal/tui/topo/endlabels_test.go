// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// endTags gives every end of every edge a text of its own: a or b for the node it is
// at, followed by the ID of the edge.
func endTags(g Graph) map[int][2]string {
	tags := map[int][2]string{}
	for _, e := range Compute(g, Options{}).Edges() {
		tags[e.ID] = [2]string{fmt.Sprintf("a%03d", e.ID), fmt.Sprintf("b%03d", e.ID)}
	}

	return tags
}

// ifaceNames names the ends of every edge after the interfaces of its first link, the
// way the map does.
func ifaceNames(g Graph) State {
	st := allUp(g)
	st.EndLabels = map[int][2]string{}

	for _, e := range Compute(g, Options{}).Edges() {
		l := g.Links[e.Links[0]]
		if l.A != e.A {
			l.AIface, l.BIface = l.BIface, l.AIface
		}

		st.EndLabels[e.ID] = [2]string{l.AIface, l.BIface}
	}

	return st
}

// placedEnd is an end label found in a painted grid.
type placedEnd struct {
	text string
	mark rune
	x, y int
}

// placedEnds reads the end labels back from a grid.
func placedEnds(g *Grid) []placedEnd {
	var out []placedEnd

	w, h := g.Size()

	for y := range h {
		for x := 0; x < w; x++ {
			if g.Cell(x, y).Class != ClassEndLabel {
				continue
			}

			end := placedEnd{x: x, y: y}

			var text strings.Builder

			for ; x < w && g.Cell(x, y).Class == ClassEndLabel; x++ {
				switch r := g.Cell(x, y).Rune; r {
				case markAbove, markAboveASCII:
					end.mark = markAbove
				case markBelow, markBelowASCII:
					end.mark = markBelow
				default:
					text.WriteRune(r)
				}
			}

			end.text = text.String()
			out = append(out, end)
		}
	}

	return out
}

// gap returns how far a text is from a box, counted in cells along both axes.
func gap(end placedEnd, r Rect) int {
	dx := max(r.X-(end.x+len(end.text)-1), end.x-(r.X+r.W-1), 0)
	dy := max(r.Y-end.y, end.y-(r.Y+r.H-1), 0)

	return dx + dy
}

// reach returns how far along the route of an edge a text is from the node at the
// start of the path and from the one at its end. The text is on the line or beside it.
func reach(e *wire, end placedEnd) (fromStart, fromEnd int) {
	width := len(end.text)
	if end.mark != 0 {
		width++
	}

	first, last := -1, -1

	for i, at := range e.path {
		// a text beside a line is a blank away from it
		if at.y != end.y || at.x < end.x-2 || at.x > end.x+width+1 {
			continue
		}

		if first < 0 {
			first = i
		}

		last = i
	}

	if first < 0 {
		return -1, -1
	}

	return first, len(e.path) - 1 - last
}

// TestEndLabelsBelongToTheirNode checks, for every drawing of the golden cases, that a
// text for an end of an edge is written on the route of that edge and closer to its
// own node than to the other one, or carries the mark that points at its node where
// it is level with both.
func TestEndLabelsBelongToTheirNode(t *testing.T) {
	placed := 0

	for _, tc := range goldenCases() {
		for _, zoom := range []Zoom{ZoomCompact, ZoomNormal, ZoomDetail} {
			opts := tc.opts
			opts.Zoom = zoom
			l := Compute(tc.graph, opts)

			st := allUp(tc.graph)
			st.EndLabels = endTags(tc.graph)
			grid := l.Paint(st, Selection{Edge: -1})

			seen := map[string]bool{}

			for _, end := range placedEnds(grid) {
				name := fmt.Sprintf(
					"%s zoom %d: %q at %d,%d",
					tc.name,
					zoom,
					end.text,
					end.x,
					end.y,
				)

				require.Len(t, end.text, 4, "%s is not a whole text", name)
				require.False(t, seen[end.text], "%s is written twice", name)

				seen[end.text] = true
				placed++

				var id int

				_, err := fmt.Sscanf(end.text[1:], "%d", &id)
				require.NoError(t, err, name)

				e := l.edges[id]
				own, other := reach(e, end)
				require.GreaterOrEqual(t, own, 0, "%s is not on the route of its edge", name)

				atStart := e.a
				if int(l.owner[e.path[0].y*l.w+e.path[0].x]) != e.a+1 {
					atStart = e.b
				}

				node := e.a
				if end.text[0] == 'b' {
					node = e.b
				}

				if node != atStart {
					own, other = other, own
				}

				rect := l.nodes[node].rect

				switch end.mark {
				case markAbove:
					assert.Less(t, rect.Y, end.y, "%s points up, its node is not there", name)
					assert.LessOrEqual(t, own, other, "%s is closer to the other node", name)
				case markBelow:
					assert.Greater(t, rect.Y, end.y, "%s points down, its node is not there", name)
					assert.LessOrEqual(t, own, other, "%s is closer to the other node", name)
				default:
					assert.Less(t, own, other, "%s is not closer to its own node", name)
				}
			}
		}
	}

	assert.Greater(t, placed, 100, "too few texts were written for the test to tell much")
}

// TestEndLabelsLeaveTheRestAlone checks that the texts at the ends of edges only take
// free cells and cells of the line of their own edge: boxes, other texts and the lines
// of other edges look the same with and without them.
func TestEndLabelsLeaveTheRestAlone(t *testing.T) {
	for _, tc := range goldenCases() {
		l := Compute(tc.graph, tc.opts)

		plain := tc.state(tc.graph)
		plain.EndLabels = nil
		named := tc.state(tc.graph)
		named.EndLabels = endTags(tc.graph)

		before, after := l.Paint(plain, tc.sel), l.Paint(named, tc.sel)
		w, h := before.Size()

		for y := range h {
			for x := range w {
				was, is := before.Cell(x, y), after.Cell(x, y)
				if was == is {
					continue
				}

				switch was.Class {
				case ClassBlank:
					assert.Equal(t, ClassEndLabel, is.Class, "%s: %d,%d", tc.name, x, y)
				case ClassLinkUp, ClassLinkDown, ClassLinkMixed, ClassLinkUnknown:
					// A text written into a line, or the blank on either side of it.
					if is.Class != ClassEndLabel {
						assert.Equal(t, ' ', is.Rune, "%s: %d,%d", tc.name, x, y)
						assert.Equal(t, was.Class, is.Class, "%s: %d,%d", tc.name, x, y)
					}
				default:
					t.Errorf(
						"%s: the cell at %d,%d changed from %+v to %+v",
						tc.name,
						x,
						y,
						was,
						is,
					)
				}
			}
		}
	}
}

// TestEndLabelsSkipSharedLines checks the case that matters most in a fabric: the one
// line that leaves a hub stands for many links, so no name is written next to it.
func TestEndLabelsSkipSharedLines(t *testing.T) {
	g := fanOut(12)
	l := Compute(g, Options{Zoom: ZoomNormal})

	st := allUp(g)
	st.EndLabels = endTags(g)

	shared := 0

	for _, e := range l.edges {
		if len(e.spans) > 0 {
			shared++
		}
	}

	require.Positive(t, shared, "the fixture has no shared lines, the test tells nothing")

	hub, ok := l.NodeRect(g.Nodes[0].ID)
	require.True(t, ok)

	for _, end := range placedEnds(l.Paint(st, Selection{Edge: -1})) {
		assert.Greater(t, gap(end, hub), 1, "%q is written next to the hub", end.text)
	}
}

// TestEndLabelsPreferTheFocus checks that the ends at the focused node are served
// before the others where there is not room for all.
func TestEndLabelsPreferTheFocus(t *testing.T) {
	g := threeTier()
	l := Compute(g, Options{Zoom: ZoomNormal})

	st := ifaceNames(g)

	count := func(sel Selection, node string) int {
		rect, ok := l.NodeRect(node)
		require.True(t, ok)

		n := 0

		for _, end := range placedEnds(l.Paint(st, sel)) {
			if gap(end, rect) <= 2 { // right next to the box
				n++
			}
		}

		return n
	}

	without := count(Selection{Edge: -1}, "leaf2")
	with := count(Selection{Node: "leaf2", Edge: -1}, "leaf2")

	assert.GreaterOrEqual(t, with, without, "the focus cost leaf2 names")
	assert.Positive(t, with, "no name is written next to the focused node")
}
