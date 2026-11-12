// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"hash/fnv"
	"image"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Font is a typeface the pixel layer writes text with: a regular and a bold cut, and a
// fallback for the symbols these do not have.
type Font struct {
	cuts [3]*sfnt.Font
	id   uint64

	mu     sync.Mutex
	faces  map[faceKey]font.Face
	glyphs map[glyphKey]*glyph
}

// The cuts of a font.
const (
	cutRegular = iota
	cutBold
	cutFallback
)

type faceKey struct {
	cut  int
	size float32
}

type glyphKey struct {
	faceKey
	r rune
	// frac is where the glyph starts between two pixels, in quarters.
	frac uint8
}

// glyph is a rendered character: its coverage, where that sits relative to the pen, and
// how far the pen moves on.
type glyph struct {
	w, h      int
	left, top int
	alpha     []uint8
	advance   float32
}

// subpixels is the number of positions between two pixels a glyph is rendered for.
const subpixels = 4

// NewFont parses the cuts of a typeface.
func NewFont(regular, bold, fallback []byte) (*Font, error) {
	f := &Font{faces: map[faceKey]font.Face{}, glyphs: map[glyphKey]*glyph{}}
	h := fnv.New64a()

	for i, data := range [][]byte{regular, bold, fallback} {
		cut, err := sfnt.Parse(data)
		if err != nil {
			return nil, err
		}

		f.cuts[i] = cut
		_, _ = h.Write(data[:min(len(data), 4096)]) //nolint:mnd // enough to tell fonts apart
	}

	f.id = h.Sum64()

	return f, nil
}

// face returns a cut at a size in pixels. The lock is held.
func (f *Font) face(cut int, size float32) font.Face {
	key := faceKey{cut: cut, size: size}
	if face, ok := f.faces[key]; ok {
		return face
	}

	face, err := opentype.NewFace(f.cuts[cut], &opentype.FaceOptions{
		Size: float64(size), DPI: 72, Hinting: font.HintingNone, //nolint:mnd // a point is a pixel
	})
	if err != nil {
		face = nil
	}

	f.faces[key] = face

	return face
}

// glyph returns a character of a cut at a size, rendered for a position between pixels.
// Characters the cut does not have come from the fallback. The lock is held.
func (f *Font) glyph(cut int, size float32, r rune, frac uint8) *glyph {
	key := glyphKey{faceKey: faceKey{cut: cut, size: size}, r: r, frac: frac}
	if g, ok := f.glyphs[key]; ok {
		return g
	}

	var buf sfnt.Buffer

	use := cut
	if i, err := f.cuts[cut].GlyphIndex(&buf, r); err != nil || i == 0 {
		use = cutFallback
	}

	g := &glyph{}
	f.glyphs[key] = g

	face := f.face(use, size)
	if face == nil {
		return g
	}

	dot := fixed.Point26_6{
		X: fixed.Int26_6(int(frac) * 64 / subpixels),
	} // 26.6 fixed point
	dr, mask, maskp, advance, ok := face.Glyph(dot, r)
	g.advance = float32(advance) / 64 //nolint:mnd // 26.6 fixed point

	if !ok {
		return g
	}

	g.w, g.h, g.left, g.top = dr.Dx(), dr.Dy(), dr.Min.X, dr.Min.Y
	g.alpha = make([]uint8, g.w*g.h)

	if a, isAlpha := mask.(*image.Alpha); isAlpha {
		for y := range g.h {
			from := a.PixOffset(maskp.X, maskp.Y+y)
			copy(g.alpha[y*g.w:(y+1)*g.w], a.Pix[from:from+g.w])
		}
	}

	return g
}

// measure returns the width of a text in pixels.
func (f *Font) measure(cut int, size float32, text string) float32 {
	f.mu.Lock()
	defer f.mu.Unlock()

	w := float32(0)
	for _, r := range text {
		w += f.glyph(cut, size, r, 0).advance
	}

	return w
}

// Align is where a text sits in the room it is given.
type Align int

// The alignments of a text.
const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// Sizes of text, as parts of its size: how far the baseline sits below the middle of a
// line, and how far glyphs reach above and below the baseline.
const (
	textBaseline = 0.36
	textAscent   = 1.0
	textDescent  = 0.3
)

// TextWidth returns the width of a text in cells, at a size in points.
func (s *Scene) TextWidth(f *Font, size float64, bold bool, text string) float64 {
	if f == nil {
		return 0
	}

	return float64(f.measure(cutOf(bold), f32(s.Pt(size)), text)) / s.cw
}

func cutOf(bold bool) int {
	if bold {
		return cutBold
	}

	return cutRegular
}

// Text writes a line of text. x and width, in cells, are the room it is aligned in, y is
// the top of its line in rows, and its size is in points.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (s *Scene) Text(
	f *Font, x, y, width, size float64, bold bool, align Align, text string, c Color,
) {
	if f == nil || text == "" || c.A == 0 {
		return
	}

	px := f32(s.Pt(size))
	w := float64(f.measure(cutOf(bold), px, text))
	left := s.X(x)

	switch align {
	case AlignCenter:
		left += (s.X(width) - w) / 2 //nolint:mnd // centered
	case AlignEnd:
		left += s.X(width) - w
	case AlignStart:
	}

	// A quarter of a pixel is as fine as glyphs are placed.
	left = math.Round(left*subpixels) / subpixels
	base := math.Round(s.Y(y+0.5) + float64(px)*textBaseline) //nolint:mnd // the middle of the line

	// The box leaves room for glyphs that reach beyond their advance.
	top, bottom := base-float64(px)*textAscent, base+float64(px)*textDescent

	s.add(&shape{
		kind: kindText, font: f, text: text, bold: bold, fill: c,
		x0: f32(left), y0: f32(base), r: px,
		bx0: f32(left - 1), by0: f32(top), bx1: f32(left + w + 2), by1: f32(bottom),
	})
}

// overhang is how far, in pixels, a glyph may reach beyond the room it is given.
const overhang = 2

// Cells writes a line of text with every character in a cell of its own, from a column
// on, the way a terminal sets text: for what was printed in columns. A character that is
// narrower than its cell sits in the middle of it.
func (s *Scene) Cells(f *Font, x, y, size float64, bold bool, text string, c Color) {
	if f == nil || text == "" || c.A == 0 {
		return
	}

	px := f32(s.Pt(size))
	left := math.Round(s.X(x)*subpixels) / subpixels
	base := math.Round(s.Y(y+0.5) + float64(px)*textBaseline) //nolint:mnd // the middle of the line
	// The box leaves room for glyphs that reach beyond their cells.
	right := left + s.cw*float64(len([]rune(text))) + overhang
	top, bottom := base-float64(px)*textAscent, base+float64(px)*textDescent

	s.add(&shape{
		kind: kindText, font: f, text: text, bold: bold, fill: c, adv: f32(s.cw),
		x0: f32(left), y0: f32(base), r: px,
		bx0: f32(left - 1), by0: f32(top), bx1: f32(right), by1: f32(bottom),
	})
}

// Advance returns how far a character of the regular cut moves the pen, as a part of the
// size of the text. For a font of fixed width that is the width of every character.
func (f *Font) Advance(r rune) float64 {
	const size = 64

	return float64(f.measure(cutRegular, size, string(r))) / size
}

// Texts returns the texts written on a layer, in the order they were written. It is there
// for tests, which cannot read pixels.
func (s *Scene) Texts(l Layer) []string {
	var texts []string

	for i := range s.shapes[l] {
		if s.shapes[l][i].kind == kindText {
			texts = append(texts, s.shapes[l][i].text)
		}
	}

	return texts
}

// write draws the glyphs of a text.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (b *strip) write(sh *shape) {
	f := sh.font

	f.mu.Lock()
	defer f.mu.Unlock()

	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)
	pen := sh.x0
	base := int(sh.y0)

	for _, r := range sh.text {
		at := pen
		if sh.adv > 0 {
			// One character a cell, in the middle of it.
			at += (sh.adv - f.glyph(cutOf(sh.bold), sh.r, r, 0).advance) / 2 //nolint:mnd // centered
		}

		whole := float32(math.Floor(float64(at)))
		g := f.glyph(cutOf(sh.bold), sh.r, r, uint8((at-whole)*subpixels))

		gx, gy := int(whole)+g.left, base+g.top

		for row := max(gy, y0); row < min(gy+g.h, y1); row++ {
			line := g.alpha[(row-gy)*g.w:]
			out := (row - b.oy) * b.w

			for col := max(gx, x0); col < min(gx+g.w, x1); col++ {
				if a := line[col-gx]; a != 0 {
					at := (out + col) * bytesPerPixel
					b.put(at, sh.fill, float32(a)/255, false)
				}
			}
		}

		if sh.adv > 0 {
			pen += sh.adv
		} else {
			pen += g.advance
		}
	}
}
