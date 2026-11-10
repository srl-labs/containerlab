// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// zoomSpec holds the dimensions of one zoom level.
type zoomSpec struct {
	boxH     int  // rows of a node box
	pad      int  // columns around the label inside a box
	inset    int  // columns at each end of a border that take no port
	step     int  // distance between the columns open to ports of one border
	half     int  // how far the ports of a top border sit from those of a bottom one
	widen    bool // boxes grow so that every edge gets a port of its own
	minW     int
	maxW     int // widest box a label may ask for
	maxLabel int
	gap      int // blank columns between two boxes of a row
	laneGap  int // blank columns between a lane and a box
}

const (
	// noteCells is the room every box of the detail zoom keeps for its note line.
	// Notes are live data that arrive with each paint, long after the boxes were
	// sized, and a box that changed width with its note would reshuffle the whole
	// drawing; so the room is fixed, and enough for a load and a memory figure.
	noteCells = 12
	// maxPorts is the number of separate ports a side may have where boxes grow
	// with their ports. A node with more links than that in one direction draws
	// them from one port, on a shared trunk.
	maxPorts = 10
)

func specFor(z Zoom) zoomSpec {
	spec := zoomSpec{
		boxH: 3, pad: 4, inset: 2, step: 4, half: 2, widen: true, //nolint:mnd // table
		minW: 8, maxLabel: 14, gap: 3, laneGap: 2, //nolint:mnd // table
	}

	switch z {
	case ZoomCompact:
		// The compact zoom is what a lab is shown at when it fits no other way,
		// so a box is its label in brackets and nothing more. Ports may sit
		// under the brackets, and edges share ports rather than widen a box.
		spec = zoomSpec{
			boxH: 1, pad: 2, step: 3, half: 1, //nolint:mnd // table
			minW: 3, maxLabel: 12, gap: 1, laneGap: 1, //nolint:mnd // table
		}
	case ZoomDetail:
		spec.boxH, spec.maxLabel, spec.gap = 4, 32, 4
	}

	spec.maxW = max(spec.portWidth(maxPorts), spec.maxLabel+spec.pad)

	return spec
}

// portColumn reports whether a column is open to ports of a top or bottom border.
//
// Ports leaving a box downwards use the columns that are a multiple of step, ports
// leaving upwards those half a step off. A vertical coming down into a channel then
// never shares a column with one going up out of it, so no two edges can be forced
// to overlap however their horizontal parts are stacked. The columns left over
// belong to lanes, which enter a row from above and leave it below on one column
// and so fit neither kind. With a step of four the verticals of ports keep a blank
// column between them; the compact zoom gives that up for narrower boxes.
func (z *zoomSpec) portColumn(c int, top bool) bool {
	if top {
		return floorMod(c, z.step) == z.half
	}

	return floorMod(c, z.step) == 0
}

// laneColumn reports whether a column is open to lanes.
func (z *zoomSpec) laneColumn(c int) bool {
	return !z.portColumn(c, true) && !z.portColumn(c, false)
}

// portWidth is the least width of a box with n ports on a side: n steps hold n
// columns of either kind wherever the box starts, and the insets keep ports off the
// corners and the column next to them.
func (z *zoomSpec) portWidth(n int) int { return z.step*n + 2*z.inset }

// capacity is the number of separate ports a side of a box of the given width is
// sure to have room for.
func (z *zoomSpec) capacity(width int) int {
	if z.widen {
		return maxPorts
	}

	return max(1, (width-2*z.inset)/z.step)
}

const (
	// truncMark replaces the cut-out part of a long label. It is plain ASCII
	// because the ellipsis is drawn two cells wide by terminals set up for East
	// Asian text.
	truncMark = '~'
	// maxSuffix is the longest last word, with its blank, that fit keeps whole.
	maxSuffix = 5
	// minStem is the shortest text that can be cut in the middle: one character
	// at each end of the mark.
	minStem = 3
)

// labelNodes cleans the node labels up. It is part of the layout because box widths
// follow from them.
func (l *Layout) labelNodes() {
	for _, b := range l.nodes {
		b.label = clean(b.Label, l.opts.ASCII)
		if b.label == "" {
			b.label = clean(b.ID, l.opts.ASCII)
		}
	}

	if l.opts.Zoom != ZoomCompact {
		return
	}

	labels := make([]string, len(l.nodes))
	for i, b := range l.nodes {
		labels[i] = b.label
	}

	cut := sharedPrefix(labels)
	for _, b := range l.nodes {
		b.label = b.label[cut:]
	}
}

// labelWidth is the width of a box before it is widened for ports.
func (l *Layout) labelWidth(b *box) int {
	w := min(utf8.RuneCountInString(b.label), l.spec.maxLabel)

	if l.opts.Zoom == ZoomDetail {
		// The note line shows the kind unless the caller supplies something else.
		w = max(w, noteCells, min(utf8.RuneCountInString(b.Kind), l.spec.maxLabel))
	}

	return min(max(w+l.spec.pad, l.spec.minW), l.spec.maxW)
}

// setText settles the text drawn in a box of the given width. The label limit of
// the zoom level decides how wide a box gets for its label; a box that is wider
// anyway, for its ports, shows as much of the label as it has room for.
func (b *box) setText(width int, spec *zoomSpec) {
	b.text = fit(b.label, max(spec.maxLabel, width-spec.pad))
}

// sharedPrefix returns the length of a prefix common to all labels that ends at a
// separator, such as the lab name in "dc1-leaf1". Cutting anywhere else would turn
// leaf1 and leaf2 into 1 and 2.
func sharedPrefix(labels []string) int {
	if len(labels) < 2 { //nolint:mnd // a single label has nothing in common
		return 0
	}

	prefix := labels[0]

	for _, s := range labels[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}

	cut := strings.LastIndexAny(prefix, "-_.:/") + 1

	for _, s := range labels {
		if len(s) == cut {
			return 0
		}
	}

	return cut
}

// fit shortens s to at most limit cells. It cuts out the middle: both the role at
// the start and the number at the end of a node name tell nodes apart. A short last
// word, such as a count appended to the name, is kept whole and the name in front
// of it shortened instead.
func fit(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}

	if at := strings.LastIndexByte(s, ' '); at > 0 {
		suffix := utf8.RuneCountInString(s[at:])
		if suffix <= maxSuffix && limit-suffix >= minStem {
			return fit(strings.TrimRight(s[:at], " "), limit-suffix) + s[at:]
		}
	}

	if limit < minStem {
		return string(r[:max(limit, 0)])
	}

	head := limit / 2 //nolint:mnd // split evenly
	tail := limit - head - 1

	return string(r[:head]) + string(truncMark) + string(r[len(r)-tail:])
}

// clip shortens s to at most limit cells by cutting off its end. Unlike a name, a
// note reads from the left and loses its meaning when the middle goes.
func clip(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}

	if limit < 1 {
		return ""
	}

	return string(r[:limit-1]) + string(truncMark)
}

// clean makes a string safe to draw one rune per cell: control and combining
// characters are dropped, and runes a terminal draws two cells wide are replaced.
// In ASCII mode everything outside printable ASCII is replaced as well.
func clean(s string, ascii bool) string {
	var b strings.Builder

	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r), unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r),
			unicode.Is(unicode.Cf, r), r == utf8.RuneError:
		case r > unicode.MaxASCII && (ascii || wide(r)):
			b.WriteByte('?')
		case !unicode.IsPrint(r):
			b.WriteByte('?')
		default:
			b.WriteRune(r)
		}
	}

	return strings.TrimSpace(b.String())
}

// wideRanges lists the East Asian wide and fullwidth blocks and the emoji planes.
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF}, {0x3400, 0x4DBF},
	{0x4E00, 0x9FFF}, {0xA000, 0xA4CF}, {0xA960, 0xA97F}, {0xAC00, 0xD7A3},
	{0xF900, 0xFAFF}, {0xFE30, 0xFE6F}, {0xFF00, 0xFF60}, {0xFFE0, 0xFFE6},
	{0x1F000, 0x1FAFF}, {0x20000, 0x3FFFD},
}

func wide(r rune) bool {
	for _, w := range wideRanges {
		if r >= w[0] && r <= w[1] {
			return true
		}
	}

	return false
}
