// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"hash/fnv"
	"image"
	"image/draw"
	"math"
	"sort"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Font is a typeface the pixel layer writes text with: a regular and a bold cut, and a
// fallback for the symbols these do not have.
//
// Glyphs are lined up with the pixels of the screen, as small text on a desktop is: the
// heights a typeface lines its letters up on (the baseline, the tops of the lowercase and
// of the capitals, the ascenders and descenders) are moved to whole pixels, and each glyph
// is moved by less than half a pixel so that its main vertical edge lands on one. Without
// that the edges fall between two pixels and are drawn half in each, which makes small
// text look blurred. Widths are not touched: text keeps its width, and the pen moves on
// as it would.
type Font struct {
	cuts [3]*sfnt.Font
	id   uint64

	mu       sync.Mutex
	buf      sfnt.Buffer
	rast     vector.Rasterizer
	advances map[advanceKey]float32
	edges    map[advanceKey]float32
	zones    map[zoneKey]*zones
	glyphs   map[glyphKey]*glyph
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

type advanceKey struct {
	faceKey
	r rune
}

type zoneKey struct {
	faceKey
	// grid is how many pixels make one pixel of the screen.
	grid uint8
}

type glyphKey struct {
	zoneKey
	r rune
	// frac is where the glyph starts between two pixels, in quarters.
	frac uint8
}

// glyph is a rendered character: its coverage, and where that sits relative to the pen.
type glyph struct {
	w, h      int
	left, top int
	alpha     []uint8
}

// subpixels is the number of positions between two pixels a glyph is rendered for.
const subpixels = 4

// NewFont parses the cuts of a typeface.
func NewFont(regular, bold, fallback []byte) (*Font, error) {
	f := &Font{
		advances: map[advanceKey]float32{},
		edges:    map[advanceKey]float32{},
		zones:    map[zoneKey]*zones{},
		glyphs:   map[glyphKey]*glyph{},
	}
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

// ppem is a size in pixels, as the outlines are scaled to it.
func ppem(size float32) fixed.Int26_6 {
	return fixed.Int26_6(math.Round(float64(size) * 64)) //nolint:mnd // 26.6 fixed point
}

// index returns the cut a character is drawn from and its glyph there: characters the
// cut does not have come from the fallback. The lock is held.
func (f *Font) index(cut int, r rune) (int, sfnt.GlyphIndex) {
	if i, err := f.cuts[cut].GlyphIndex(&f.buf, r); err == nil && i != 0 {
		return cut, i
	}

	i, _ := f.cuts[cutFallback].GlyphIndex(&f.buf, r)

	return cutFallback, i
}

// advance returns how far a character of a cut at a size moves the pen, in pixels. The
// lock is held.
func (f *Font) advance(cut int, size float32, r rune) float32 {
	key := advanceKey{faceKey: faceKey{cut: cut, size: size}, r: r}
	if a, ok := f.advances[key]; ok {
		return a
	}

	use, i := f.index(cut, r)

	adv, err := f.cuts[use].GlyphAdvance(&f.buf, i, ppem(size), font.HintingNone)
	if err != nil {
		adv = 0
	}

	a := float32(adv) / 64 //nolint:mnd // 26.6 fixed point
	f.advances[key] = a

	return a
}

// edge returns where the main vertical edge of a character sits, in pixels from the pen
// and in quarters of a pixel: the leftmost straight vertical edge of its outline that is
// long enough to be a stem. It is not a number for a character without one, such as a
// round letter, which is left where the pen puts it: moving it gains little and would
// show in the spacing. The lock is held.
func (f *Font) edge(cut int, size float32, r rune) float32 {
	key := advanceKey{faceKey: faceKey{cut: cut, size: size}, r: r}
	if e, ok := f.edges[key]; ok {
		return e
	}

	use, i := f.index(cut, r)

	e := float32(math.NaN())
	if segs, err := f.cuts[use].LoadGlyph(&f.buf, i, ppem(size), nil); err == nil &&
		len(segs) > 0 {
		e = stemEdge(segs, size)
	}

	e = float32(math.Round(float64(e*subpixels))) / subpixels
	f.edges[key] = e

	return e
}

// stemEdge returns the leftmost vertical edge of an outline at a size that is long enough
// to be a stem, in pixels, or not a number when it has none.
func stemEdge(segs sfnt.Segments, size float32) float32 {
	const (
		upright = 1.0 / 16 // how far an edge may lean and still count as vertical
		stem    = 0.25     // how long an edge is at least, as a part of the size
	)

	edge := float32(math.Inf(1))

	var px, py float32

	for _, seg := range segs {
		end := seg.Args[argCount(seg.Op)-1]
		x, y := float32(end.X)/64, float32(end.Y)/64 //nolint:mnd // 26.6 fixed point

		if seg.Op == sfnt.SegmentOpLineTo && abs32(x-px) <= upright && abs32(y-py) >= size*stem {
			edge = min(edge, x)
		}

		px, py = x, y
	}

	if math.IsInf(float64(edge), 1) {
		return float32(math.NaN())
	}

	return edge
}

// zones are the heights of a cut at a size that glyphs line up on, in pixels above the
// baseline: from is where the outlines have them, to where the hinted glyphs do. Between
// two of them a glyph is stretched evenly, beyond the outermost ones it is moved.
type zones struct {
	from, to []float32
}

// at returns where a height of an outline goes.
func (z *zones) at(y float32) float32 {
	n := len(z.from)

	switch {
	case n == 0:
		return y
	case y <= z.from[0]:
		return y + z.to[0] - z.from[0]
	case y >= z.from[n-1]:
		return y + z.to[n-1] - z.from[n-1]
	}

	i := 1
	for z.from[i] < y {
		i++
	}

	t := (y - z.from[i-1]) / (z.from[i] - z.from[i-1])

	return z.to[i-1] + t*(z.to[i]-z.to[i-1])
}

// extent returns how far a glyph reaches above and below the baseline, in pixels: both
// are heights, the lower one negative when the glyph goes below the baseline.
func (f *Font) extent(cut int, size float32, r rune) (top, bottom float32, ok bool) {
	i, err := f.cuts[cut].GlyphIndex(&f.buf, r)
	if err != nil || i == 0 {
		return 0, 0, false
	}

	segs, err := f.cuts[cut].LoadGlyph(&f.buf, i, ppem(size), nil)
	if err != nil || len(segs) == 0 {
		return 0, 0, false
	}

	// The outlines have their y axis pointing down.
	b := segs.Bounds()

	return float32(-b.Min.Y) / 64, float32(-b.Max.Y) / 64, true //nolint:mnd // 26.6 fixed point
}

// zonesOf returns the heights a cut at a size lines its glyphs up on, and where on the
// pixels of the screen they go. The lock is held.
//
// A round letter reaches a little beyond the flat ones, so that it looks as tall: that
// overshoot is kept only as far as it makes whole pixels, which at the sizes of the
// layer is none, and the round tops sit on the same pixel as the flat ones.
func (f *Font) zonesOf(cut int, size float32, grid int) *zones {
	key := zoneKey{faceKey: faceKey{cut: cut, size: size}, grid: uint8(grid)}
	if z, ok := f.zones[key]; ok {
		return z
	}

	q := float32(grid)
	snap := func(v float32) float32 { return float32(math.Round(float64(v/q))) * q }

	type pair struct{ from, to float32 }

	pairs := []pair{{0, 0}} // the baseline

	// flat and round add a height that flat letters reach, and the one the round
	// letters of the same kind reach.
	flat := func(r rune, top bool) (float32, bool) {
		t, b, ok := f.extent(cut, size, r)
		if !top {
			t = b
		}

		if ok {
			pairs = append(pairs, pair{t, snap(t)})
		}

		return t, ok
	}
	round := func(r rune, top bool, base float32) {
		t, b, ok := f.extent(cut, size, r)
		if !top {
			t = b
		}

		if ok {
			pairs = append(pairs, pair{t, snap(base) + snap(t-base)})
		}
	}

	if x, ok := flat('x', true); ok {
		round('o', true, x)
	}

	if h, ok := flat('H', true); ok {
		round('O', true, h)
	}

	round('o', false, 0)
	flat('l', true)
	flat('p', false)

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].from < pairs[j].from })

	z := &zones{}

	for _, p := range pairs {
		n := len(z.from)
		if n > 0 && p.from-z.from[n-1] < 1.0/64 { // the same height
			continue
		}

		// Heights keep their order.
		if n > 0 {
			p.to = max(p.to, z.to[n-1])
		}

		z.from, z.to = append(z.from, p.from), append(z.to, p.to)
	}

	f.zones[key] = z

	return z
}

// glyph returns a character of a cut at a size, hinted for a grid and rendered for a
// position between pixels. The lock is held.
func (f *Font) glyph(cut int, size float32, grid int, r rune, frac uint8) *glyph {
	key := glyphKey{
		zoneKey: zoneKey{faceKey: faceKey{cut: cut, size: size}, grid: uint8(grid)},
		r:       r, frac: frac,
	}
	if g, ok := f.glyphs[key]; ok {
		return g
	}

	g := &glyph{}
	f.glyphs[key] = g

	use, i := f.index(cut, r)
	// The zones first: finding them loads other glyphs into the buffer the outline is
	// read from.
	z := f.zonesOf(use, size, grid)

	segs, err := f.cuts[use].LoadGlyph(&f.buf, i, ppem(size), nil)
	if err != nil || len(segs) == 0 {
		return g
	}

	dx := float32(frac) / subpixels

	// The points of the outline, moved to where the glyph starts and hinted, with the
	// y axis pointing down as in the image.
	pts := make([][2]float32, 0, 3*len(segs)) //nolint:mnd // at most three points a segment
	x0, y0 := float32(math.Inf(1)), float32(math.Inf(1))
	x1, y1 := float32(math.Inf(-1)), float32(math.Inf(-1))

	for _, seg := range segs {
		for k := range seg.Args[:argCount(seg.Op)] {
			x := float32(seg.Args[k].X)/64 + dx      //nolint:mnd // 26.6 fixed point
			y := -z.at(-float32(seg.Args[k].Y) / 64) //nolint:mnd // 26.6 fixed point
			pts = append(pts, [2]float32{x, y})
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
		}
	}

	left, top := int(math.Floor(float64(x0))), int(math.Floor(float64(y0)))
	w, h := int(math.Ceil(float64(x1)))-left, int(math.Ceil(float64(y1)))-top

	if w <= 0 || h <= 0 {
		return g
	}

	g.w, g.h, g.left, g.top, g.alpha = w, h, left, top, f.fill(segs, pts, left, top, w, h)

	return g
}

// fill rasterizes an outline whose points were moved, into a coverage mask of a size
// whose top left corner sits at a position. The lock is held.
func (f *Font) fill(segs sfnt.Segments, pts [][2]float32, left, top, w, h int) []uint8 {
	f.rast.Reset(w, h)
	f.rast.DrawOp = draw.Src

	ox, oy := float32(left), float32(top)
	k := 0
	next := func() (float32, float32) {
		p := pts[k]
		k++

		return p[0] - ox, p[1] - oy
	}

	for _, seg := range segs {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			f.rast.MoveTo(next())
		case sfnt.SegmentOpLineTo:
			f.rast.LineTo(next())
		case sfnt.SegmentOpQuadTo:
			ax, ay := next()
			bx, by := next()
			f.rast.QuadTo(ax, ay, bx, by)
		case sfnt.SegmentOpCubeTo:
			ax, ay := next()
			bx, by := next()
			cx, cy := next()
			f.rast.CubeTo(ax, ay, bx, by, cx, cy)
		}
	}

	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	f.rast.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})

	return mask.Pix
}

// argCount is the number of points a segment of an outline has.
func argCount(op sfnt.SegmentOp) int {
	switch op {
	case sfnt.SegmentOpQuadTo:
		return 2 //nolint:mnd // a control point and the end
	case sfnt.SegmentOpCubeTo:
		return 3 //nolint:mnd // two control points and the end
	case sfnt.SegmentOpMoveTo, sfnt.SegmentOpLineTo:
	}

	return 1
}

// measure returns the width of a text in pixels.
func (f *Font) measure(cut int, size float32, text string) float32 {
	f.mu.Lock()
	defer f.mu.Unlock()

	w := float32(0)
	for _, r := range text {
		w += f.advance(cut, size, r)
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
	base := s.baseline(y, px)

	// The box leaves room for glyphs that reach beyond their advance.
	top, bottom := base-float64(px)*textAscent, base+float64(px)*textDescent

	s.add(&shape{
		kind: kindText, font: f, text: text, bold: bold, fill: c, grid: uint8(s.m.grid()),
		x0: f32(left), y0: f32(base), r: px,
		bx0: f32(left - 1), by0: f32(top), bx1: f32(left + w + 2), by1: f32(bottom),
	})
}

// baseline returns where the baseline of a line of text sits, in pixels, for the top of
// its line in rows and its size in pixels: on a whole pixel of the screen, as the glyphs
// are hinted to be.
func (s *Scene) baseline(y float64, px float32) float64 {
	q := float64(s.m.grid())

	//nolint:mnd // the middle of the line
	return math.Round((s.Y(y+0.5)+float64(px)*textBaseline)/q) * q
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
	base := s.baseline(y, px)
	// The box leaves room for glyphs that reach beyond their cells.
	right := left + s.cw*float64(len([]rune(text))) + overhang
	top, bottom := base-float64(px)*textAscent, base+float64(px)*textDescent

	s.add(&shape{
		kind: kindText, font: f, text: text, bold: bold, fill: c, adv: f32(s.cw),
		grid: uint8(s.m.grid()), x0: f32(left), y0: f32(base), r: px,
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
	cut := cutOf(sh.bold)
	q := float64(max(1, sh.grid))

	for _, r := range sh.text {
		at := float64(pen)
		if sh.adv > 0 {
			// One character a cell, in the middle of it.
			at += float64(sh.adv-f.advance(cut, sh.r, r)) / 2 //nolint:mnd // centered
		}

		// The main vertical edge on a pixel of the screen.
		if e := float64(f.edge(cut, sh.r, r)); !math.IsNaN(e) {
			at = math.Round((at+e)/q)*q - e
		}

		quarters := math.Round(at * subpixels)
		whole := math.Floor(quarters / subpixels)
		g := f.glyph(cut, sh.r, int(q), r, uint8(quarters-whole*subpixels))

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
			pen += f.advance(cut, sh.r, r)
		}
	}
}
