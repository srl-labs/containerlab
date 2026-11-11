// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	_ "embed" // the fonts
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

//go:embed assets/fonts/Inter-400.ttf
var fontRegular []byte

//go:embed assets/fonts/Inter-600.ttf
var fontBold []byte

//go:embed assets/fonts/DejaVuSans.ttf
var fontSymbols []byte

//go:embed assets/fonts/JetBrainsMono-400.ttf
var fontMono []byte

//go:embed assets/fonts/JetBrainsMono-700.ttf
var fontMonoBold []byte

// monoFont is the typeface of fixed width the pixel layer sets literal text in, where
// the terminal cannot show it.
var monoFont = sync.OnceValue(func() *clabinternaltuigfx.Font {
	f, err := clabinternaltuigfx.NewFont(fontMono, fontMonoBold, fontSymbols)
	if err != nil {
		return nil
	}

	return f
})

// uiFont is the typeface the pixel layer writes text with.
var uiFont = sync.OnceValue(func() *clabinternaltuigfx.Font {
	f, err := clabinternaltuigfx.NewFont(fontRegular, fontBold, fontSymbols)
	if err != nil {
		return nil
	}

	return f
})

// escape starts a control sequence.
const escape = 0x1b

// textSize is the size of the text of the pixel layer, in points.
const textSize = 13

// textStyle is how a cell of a line of styled text looks.
type textStyle struct {
	fg, bg         clabinternaltuigfx.Color
	hasFG, hasBG   bool
	bold, reverse  bool
	faint, underln bool
}

// textCell is one cell of a line of styled text.
type textCell struct {
	r  rune
	st textStyle
	// mark is how the piece of text the cell belongs to is aligned, zero when it
	// carries no mark, and piece tells the marked pieces of a line apart.
	mark  int
	piece int
}

func (c *textCell) blank() bool { return c.r == ' ' && !c.st.hasBG && !c.st.reverse && c.mark == 0 }

// ansiColors are the colors of the sixteen color palette, for text that uses them.
//
//nolint:mnd // the components of the colors
var ansiColors = [16]clabinternaltuigfx.Color{
	rgb(0x1c, 0x21, 0x28),
	rgb(0xf8, 0x51, 0x49),
	rgb(0x3f, 0xb9, 0x50),
	rgb(0xd2, 0x99, 0x22),
	rgb(0x58, 0xa6, 0xff),
	rgb(0xbc, 0x8c, 0xff),
	rgb(0x39, 0xc5, 0xcf),
	rgb(0xc9, 0xd1, 0xd9),
	rgb(0x6e, 0x76, 0x81),
	rgb(0xff, 0x7b, 0x72),
	rgb(0x56, 0xd3, 0x64),
	rgb(0xe3, 0xb3, 0x41),
	rgb(0x79, 0xc0, 0xff),
	rgb(0xd2, 0xa8, 0xff),
	rgb(0x56, 0xd4, 0xdd),
	rgb(0xf0, 0xf6, 0xfc),
}

func rgb(r, g, b uint8) clabinternaltuigfx.Color {
	return clabinternaltuigfx.Color{R: r, G: g, B: b, A: 0xff} //nolint:mnd // opaque
}

// color256 returns a color of the 256 color palette.
//
//nolint:mnd // the layout of the palette
func color256(n int) clabinternaltuigfx.Color {
	switch {
	case n < 16:
		return ansiColors[n]
	case n < 232:
		n -= 16
		level := func(v int) uint8 {
			if v == 0 {
				return 0
			}

			return uint8(55 + 40*v)
		}

		return clabinternaltuigfx.Color{
			R: level(n / 36),
			G: level(n / 6 % 6),
			B: level(n % 6),
			A: 0xff,
		}
	default:
		v := uint8(8 + 10*(n-232))

		return clabinternaltuigfx.Color{R: v, G: v, B: v, A: 0xff}
	}
}

// sgr applies the parameters of a select graphic rendition sequence to a style.
//
//nolint:mnd,gocyclo // the numbers of the attributes
func (st *textStyle) sgr(params []int) {
	if len(params) == 0 {
		*st = textStyle{}
	}

	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == 0:
			*st = textStyle{}
		case p == 1:
			st.bold = true
		case p == 2:
			st.faint = true
		case p == 4:
			st.underln = true
		case p == 7:
			st.reverse = true
		case p == 22:
			st.bold, st.faint = false, false
		case p == 24:
			st.underln = false
		case p == 27:
			st.reverse = false
		case p == 39:
			st.hasFG = false
		case p == 49:
			st.hasBG = false
		case p >= 30 && p <= 37:
			st.fg, st.hasFG = ansiColors[p-30], true
		case p >= 90 && p <= 97:
			st.fg, st.hasFG = ansiColors[p-90+8], true
		case p >= 40 && p <= 47:
			st.bg, st.hasBG = ansiColors[p-40], true
		case p >= 100 && p <= 107:
			st.bg, st.hasBG = ansiColors[p-100+8], true
		case p == 38 || p == 48:
			var c clabinternaltuigfx.Color

			switch {
			case i+4 < len(params) && params[i+1] == 2:
				c = clabinternaltuigfx.Color{
					R: uint8(
						params[i+2],
					),
					G: uint8(params[i+3]),
					B: uint8(params[i+4]),
					A: 0xff, //nolint:gosec // color components
				}
				i += 4
			case i+2 < len(params) && params[i+1] == 5:
				c = color256(params[i+2] & 0xff)
				i += 2
			default:
				continue
			}

			if p == 38 {
				st.fg, st.hasFG = c, true
			} else {
				st.bg, st.hasBG = c, true
			}
		}
	}
}

// parseLine turns a line of styled text into its cells.
func parseLine(line string) []textCell {
	cells := make([]textCell, 0, len(line))

	var st textStyle

	mark, piece := 0, 0

	for i := 0; i < len(line); {
		if line[i] == escape && i+1 < len(line) && line[i+1] == '[' {
			// A control sequence runs until its final byte.
			end := i + len("\x1b[")
			for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
				end++
			}

			if end >= len(line) {
				break
			}

			params := csiParams(line[i+2 : end])

			switch line[end] {
			case 'm':
				st.sgr(params)
			case 'z':
				mark = 0
				if len(params) > 0 {
					mark = params[0]
				}

				piece++
			}

			i = end + 1

			continue
		}

		if line[i] == escape { // another escape sequence: skip its introducer
			i += 2 // ESC and what follows

			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		i += size

		cell := textCell{r: r, st: st, mark: mark}
		if mark != 0 {
			cell.piece = piece
		}

		cells = append(cells, cell)

		// A character that takes two cells is followed by an empty one.
		if clabinternaltuiui.Width(string(r)) > 1 {
			cells = append(cells, textCell{r: 0, st: st, mark: mark, piece: cell.piece})
		}
	}

	return cells
}

func csiParams(s string) []int {
	if s == "" {
		return nil
	}

	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ':' })
	params := make([]int, 0, len(parts))

	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimLeft(p, "<=>?"))
		if err != nil {
			n = 0
		}

		params = append(params, n)
	}

	return params
}

// numeric matches a value that reads best flush right: a number with an optional unit.
var numeric = regexp.MustCompile(`^[↓↑+-]?\d+(\.\d+)?\s?(%|[kMGTµnmshdwy]{0,2}|ms|mo)$`)

// pixelLine writes the text of a line of the window with the pixel layer. Text that is
// set apart by more than a space, or that changes its look after a space, starts at the
// cell it starts at in the terminal: columns stay columns. What comes in between flows in
// the font, whose letters differ in width.
//
// In a dialog the line under the cursor of a list gets a highlight, and a field to type in,
// which the terminal shows between brackets, gets a box.
func (m *Model) pixelLine(sc *clabinternaltuigfx.Scene, row, col int, line string, dialog bool) {
	cells := parseLine(line)

	if dialog {
		m.pixelControls(sc, row, col, cells)
	}

	// A letter of the font can be wider than a cell: a piece of text never starts
	// before the one in front of it has ended.
	free := float64(col)

	for i := 0; i < len(cells); {
		if cells[i].blank() || cells[i].r == 0 {
			i++

			continue
		}

		end := fieldEnd(cells, i)
		free = m.pixelField(sc, row, col+i, cells[i:end], free) + textGap
		i = end
	}
}

// pixelCells writes a line the way the terminal would: every character in its cell, in a
// font of fixed width. It is how the layer sets what a node or a command printed.
func (m *Model) pixelCells(sc *clabinternaltuigfx.Scene, row, col int, line string) {
	font := monoFont()
	if font == nil {
		return
	}

	sk := m.skin()
	ink := clabinternaltuigfx.Mix(sk.Ink, sk.Card, 0.1) //nolint:mnd // a little softer than pure

	// As large as fits the width of a cell, and no larger than the other text.
	size := min(textSize, sc.X(1)/sc.Pt(1)/font.Advance('0'))
	cells := parseLine(line)

	for i := 0; i < len(cells); {
		if cells[i].blank() || cells[i].r == 0 {
			i++

			continue
		}

		// A run of cells of one look.
		end := i + 1
		for end < len(cells) && cells[end].st == cells[i].st && !cells[end].blank() {
			end++
		}

		var text strings.Builder

		for k := i; k < end; k++ {
			if cells[k].r == 0 {
				text.WriteRune(' ')
			} else {
				text.WriteRune(cells[k].r)
			}
		}

		st := &cells[i].st
		fg, bg := ink, clabinternaltuigfx.Color{}

		if st.hasFG {
			fg = st.fg
		}

		if st.hasBG {
			bg = st.bg
		}

		if st.reverse {
			fg, bg = sk.Card, fg
		}

		if bg.A != 0 {
			sc.Rect(float64(col+i), float64(row), float64(end-i), 1, bg)
		}

		if st.faint {
			fg = fg.Alpha(0.6) //nolint:mnd // dimmed
		}

		sc.Cells(font, float64(col+i), float64(row), size, st.bold, text.String(), fg)

		i = end
	}
}

// textGap is the least room between two pieces of text of a line, in cells.
const textGap = 0.55

// cursorMark is the glyph in front of the line under the cursor of a list or a form.
const cursorMark = '▸'

// pixelControls draws the controls of a line of a dialog: the box of a field to type
// in, and the highlight of the line under the cursor.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelControls(sc *clabinternaltuigfx.Scene, row, col int, cells []textCell) {
	sk := m.skin()
	y := float64(row)

	first, focused := -1, false

	for i := range cells {
		if !cells[i].blank() {
			first, focused = i, cells[i].r == cursorMark

			break
		}
	}

	open, closing := -1, -1

	for i := range cells {
		switch cells[i].r {
		case '[':
			if open < 0 {
				open = i
			}
		case ']':
			closing = i
		}
	}

	// A field to type in: wide enough to be one, not a mark like [x].
	if open >= 0 && closing-open >= 6 {
		cells[open].r, cells[closing].r = ' ', ' '
		x, w := float64(col+open)-0.2, float64(closing-open)+1.4

		edge := sk.CardEdge
		if focused {
			edge = sk.Accent
			sc.Halo(x, y+0.04, w, 0.92, 5, 2.5, sk.Accent.Alpha(0.22))
		}

		sc.Box(x, y+0.04, w, 0.92, 5, sk.Bar, edge, 1)

		return
	}

	if focused && first >= 0 {
		sc.Box(float64(col)+0.6, y+0.02, float64(len(cells))-1.2, 0.96, 6,
			sk.Selection, sk.Accent.Alpha(0.35), 1)
	}
}

// fieldEnd returns where the piece of text that starts at a cell ends.
func fieldEnd(cells []textCell, from int) int {
	first := &cells[from]
	end := from + 1

	if first.mark != 0 {
		for end < len(cells) && cells[end].mark != 0 && cells[end].piece == first.piece {
			end++
		}

		return end
	}

	for end < len(cells) {
		c := &cells[end]

		switch {
		case c.mark != 0:
			return end
		case !c.blank():
			end++
		case end+1 < len(cells) && !cells[end+1].blank() && cells[end+1].mark == 0 &&
			cells[end+1].st == cells[end-1].st && !cells[end-1].blank():
			// A single space inside text of one look: the text goes on.
			end += 2 // the space and the cell after it
		default:
			return end
		}
	}

	return end
}

// pixelField writes one piece of text, in the cells it takes from a column on.
//
// free is where the text in front of it on the line ends: the piece starts there at the
// earliest. It returns where it ends itself.
//
//nolint:funlen // the parts of a piece of text
func (m *Model) pixelField(
	sc *clabinternaltuigfx.Scene, row, col int, cells []textCell, free float64,
) float64 {
	font := uiFont()
	sk := m.skin()
	ink := clabinternaltuigfx.Mix(sk.Ink, sk.Card, 0.1) //nolint:mnd // a little softer than pure

	// The runs of one look the piece is made of.
	type run struct {
		text  string
		st    textStyle
		cells int
	}

	var runs []run

	for i := range cells {
		r := cells[i].r
		if r == 0 {
			if len(runs) > 0 {
				runs[len(runs)-1].cells++
			}

			continue
		}

		if len(runs) == 0 || runs[len(runs)-1].st != cells[i].st {
			runs = append(runs, run{st: cells[i].st})
		}

		last := &runs[len(runs)-1]
		last.text += string(r)
		last.cells++
	}

	// A piece on a background of its own is a chip: its text is centered on it.
	boxed := true
	for i := range runs {
		boxed = boxed && (runs[i].st.hasBG || runs[i].st.reverse)
	}

	align := clabinternaltuigfx.AlignStart

	switch cells[0].mark {
	case 1:
		align = clabinternaltuigfx.AlignEnd
		runs[0].text = strings.TrimLeft(runs[0].text, " ")
	case 2: //nolint:mnd // the center mark
		align = clabinternaltuigfx.AlignCenter
		runs[0].text = strings.TrimLeft(runs[0].text, " ")
	case 0:
		var whole strings.Builder
		for i := range runs {
			whole.WriteString(runs[i].text)
		}

		if numeric.MatchString(strings.TrimSpace(whole.String())) {
			align = clabinternaltuigfx.AlignEnd
		}
	}

	if boxed {
		align = clabinternaltuigfx.AlignCenter
		runs[0].text = strings.TrimLeft(runs[0].text, " ")
		runs[len(runs)-1].text = strings.TrimRight(runs[len(runs)-1].text, " ")
	} else if align != clabinternaltuigfx.AlignStart {
		runs[len(runs)-1].text = strings.TrimRight(runs[len(runs)-1].text, " ")
	}

	width := 0.0
	for i := range runs {
		width += sc.TextWidth(font, textSize, runs[i].st.bold, runs[i].text)
	}

	x, span := float64(col), float64(len(cells))

	switch align {
	case clabinternaltuigfx.AlignCenter:
		x += (span - width) / 2 //nolint:mnd // centered
	case clabinternaltuigfx.AlignEnd:
		x += span - width
	case clabinternaltuigfx.AlignStart:
	}

	x = max(x, free)
	at := float64(col)

	for i := range runs {
		st := &runs[i].st
		fg, bg := ink, clabinternaltuigfx.Color{}

		if st.hasFG {
			fg = st.fg
		}

		if st.hasBG {
			bg = st.bg
		}

		if st.reverse {
			fg, bg = sk.Card, fg
		}

		if bg.A != 0 {
			//nolint:mnd // a chip a little smaller than its cells
			sc.Fill(at, float64(row)+0.06, float64(runs[i].cells), 0.88, 4, bg)
		}

		if st.faint {
			fg = fg.Alpha(0.6) //nolint:mnd // dimmed
		}

		sc.Text(
			font,
			x,
			float64(row),
			0,
			textSize,
			st.bold,
			clabinternaltuigfx.AlignStart,
			runs[i].text,
			fg,
		)

		w := sc.TextWidth(font, textSize, st.bold, runs[i].text)
		if st.underln {
			sc.Rect(x, float64(row)+0.86, w, 0.06, fg) //nolint:mnd // under the baseline
		}

		x += w
		at += float64(runs[i].cells)
	}

	return x
}
