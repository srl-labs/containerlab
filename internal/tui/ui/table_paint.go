// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"image/color"

	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
)

// pixelGutter is the room in front of the rows of a table drawn with a pixel layer: the
// cursor bar, a checkbox two cells wide and a space.
const pixelGutter = 4

// Skin holds the colors of the pixel layer. They are derived from the theme and from the
// background of the terminal, so that the surfaces sit on it naturally.
type Skin struct {
	// Bar is the background of the header and of the bar at the bottom.
	Bar clabinternaltuigfx.Color
	// Card is the fill of a panel and CardEdge its border.
	Card     clabinternaltuigfx.Color
	CardEdge clabinternaltuigfx.Color
	// Ink is the color of text on the background.
	Ink clabinternaltuigfx.Color
	// Line separates areas, RowLine the rows of a table.
	Line    clabinternaltuigfx.Color
	RowLine clabinternaltuigfx.Color
	// Raised is the fill of buttons, chips and key caps, RaisedEdge their border.
	Raised     clabinternaltuigfx.Color
	RaisedEdge clabinternaltuigfx.Color
	// Control is the outline of an empty checkbox or input.
	Control clabinternaltuigfx.Color

	Selection clabinternaltuigfx.Color
	Accent    clabinternaltuigfx.Color
	OnAccent  clabinternaltuigfx.Color
	Thumb     clabinternaltuigfx.Color
	Shadow    clabinternaltuigfx.Color
	Scrim     clabinternaltuigfx.Color
}

// DecoKind tells what the pixel layer draws for a table cell.
type DecoKind int

// The decorations of a cell.
const (
	DecoNone DecoKind = iota
	// DecoDot is a filled state dot, drawn in place of the text.
	DecoDot
	// DecoSpinner is the ring of a state in transition, drawn in place of the text.
	DecoSpinner
	// DecoRing is the hollow dot of something that is off, drawn in place of the text.
	DecoRing
	// DecoPill is a tinted pill behind the text.
	DecoPill
	// DecoSwatch is a small square of color in front of the text, which leaves two
	// cells of room for it.
	DecoSwatch
	// DecoChart is a small chart of the series, drawn in place of the text.
	DecoChart
)

// Deco is the decoration of a table cell.
type Deco struct {
	Kind  DecoKind
	Color color.Color
	// Series and Limit are the values of a chart and what its full height stands for.
	Series []float64
	Limit  float64
}

// replacesText reports whether the decoration is drawn instead of the text of the cell.
func (d Deco) replacesText() bool {
	switch d.Kind {
	case DecoDot, DecoSpinner, DecoRing, DecoChart:
		return true
	case DecoNone, DecoPill, DecoSwatch:
		return false
	}

	return false
}

// Sizes of the decorations, in points and fractions of a cell.
const (
	cardRadius   = 9
	rowRadius    = 6
	pillRadius   = 9
	checkRadius  = 4
	dotRadius    = 3.6
	haloRadius   = 6.5
	ringRadius   = 4.2
	ringWidth    = 2
	chartWidth   = 1.3
	chartFill    = 0.3
	checkWidth   = 1.7
	checkHeight  = 0.74
	tintOpacity  = 0.16
	trackOpacity = 0.4
)

// Paint draws the pixel layer of the table: the card it sits on, the highlight of the row
// under the cursor, the checkboxes of the marks, the decorations of the cells and the
// scrollbar. x and y are where the text of the table starts, in cells.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (t *Table) Paint(sc *clabinternaltuigfx.Scene, x, y float64, sk *Skin) {
	if t.width <= 0 || t.height <= 0 {
		return
	}

	t.syncScrollbar()
	t.ensureLayout()

	w, h := float64(t.width), float64(t.height)

	sc.Box(x-1, y-0.5, w+2, h+1, cardRadius, sk.Card, sk.CardEdge, 1)
	sc.HLine(x-1, y+1, w+2, sk.CardEdge)

	rowWidth := float64(t.rowWidth())
	body := t.bodyHeight()

	for i := t.offset; i < min(len(t.shown), t.offset+body); i++ {
		ry := y + 1 + float64(i-t.offset)
		row := &t.rows[t.shown[i]]
		cursor := i == t.cursor && t.Focused

		switch {
		case cursor:
			sc.Box(
				x-0.4,
				ry+0.02,
				rowWidth+0.6,
				0.96,
				rowRadius, // a hair inside the row
				sk.Selection,
				sk.Accent.Alpha(0.35),
				1,
			) // a faint edge
			sc.Fill(x-0.4, ry+0.14, 0.38, 0.72, 2, sk.Accent) // the cursor bar
		case i > t.offset && (i-1 != t.cursor || !t.Focused):
			sc.HLine(x, ry, rowWidth, sk.RowLine)
		}

		t.paintMark(sc, x+1, ry, row, sk)
		t.paintCells(sc, x, ry, row)
	}

	if from, to := t.thumb(); to > from {
		bx := x + w - 0.75 // inside the last column

		// The track.
		sc.Fill(bx, y+1.2, 0.45, float64(body)-0.4, 3, sk.Thumb.Alpha(trackOpacity))
		// The thumb.
		sc.Fill(bx, y+1.2+float64(from), 0.45, float64(to-from)-0.4, 3, sk.Thumb)
	}
}

// paintMark draws the checkbox of a row.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (t *Table) paintMark(sc *clabinternaltuigfx.Scene, x, ry float64, row *Row, sk *Skin) {
	top := ry + (1-checkHeight)/2 // centered in the row

	if _, marked := t.marks[row.Key]; !marked {
		sc.Outline(x, top, checkWidth, checkHeight, checkRadius, sk.Control)

		return
	}

	sc.Fill(x, top, checkWidth, checkHeight, checkRadius, sk.Accent)
	sc.Icon(clabinternaltuigfx.IconCheck, x+checkWidth/2, ry+0.5, 13, 1.8, sk.OnAccent)
}

// paintCells draws the decorations of the cells of a row.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (t *Table) paintCells(sc *clabinternaltuigfx.Scene, x, ry float64, row *Row) {
	pos := float64(t.gutter())

	for k, c := range t.layout.cols {
		if k > 0 {
			pos += float64(t.gapAfter(t.layout.cols[k-1]))
		}

		width := float64(t.layout.widths[k])
		cx := x + pos
		pos += width

		cell := t.cell(row, c)
		if cell.Deco.Kind == DecoNone {
			continue
		}

		tint := clabinternaltuigfx.RGB(cell.Deco.Color)
		mid := ry + 0.5 // the middle of the row

		switch cell.Deco.Kind {
		case DecoDot:
			sc.Dot(cx+0.5, mid, haloRadius, tint.Alpha(tintOpacity))
			sc.Dot(cx+0.5, mid, dotRadius, tint)
		case DecoSpinner:
			sc.Ring(cx+0.5, mid, ringRadius, ringWidth, tint.Alpha(0.25))
			sc.Arc(cx+0.5, mid, ringRadius-1, ringWidth, 0.05, 0.3, tint)
		case DecoRing:
			sc.Ring(cx+0.5, mid, ringRadius, ringWidth, tint) // the middle of the cell
		case DecoPill:
			tw := float64(Width(Truncate(cell.Text, t.layout.widths[k])))
			sc.Fill(cx-0.8, ry+0.08, tw+1.6, 0.84, pillRadius, tint.Alpha(tintOpacity))
		case DecoSwatch:
			sc.Fill(cx, ry+0.24, 1.2, 0.52, 3, tint) // a small square
		case DecoChart:
			sc.Chart(
				cx,
				ry+0.14,
				width,
				0.72,
				cell.Deco.Series,
				cell.Deco.Limit, // inside the row
				chartWidth,
				tint,
				chartFill,
			)
		case DecoNone:
		}
	}
}
