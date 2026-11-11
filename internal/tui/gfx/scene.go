// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package gfx draws the pixel layer of the UI: the surfaces, controls and charts a
// terminal that speaks the kitty graphics protocol shows under and over its text. A
// frame is described as a scene of shapes positioned in cells, rendered one strip per
// text row, and only the strips that changed are sent to the terminal.
package gfx

import (
	"encoding/binary"
	"hash/fnv"
	"image/color"
	"math"
)

// Color is a color with straight (not premultiplied) alpha.
type Color struct{ R, G, B, A uint8 }

// RGB converts a color to an opaque Color. A nil color is transparent.
func RGB(c color.Color) Color {
	if c == nil {
		return Color{}
	}

	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)

	return Color{R: n.R, G: n.G, B: n.B, A: n.A}
}

// Alpha returns the color with its opacity scaled.
//
//nolint:mnd // the arithmetic of pixels and colors
func (c Color) Alpha(a float64) Color {
	c.A = uint8(math.Round(clamp(float64(c.A)*a, 0, 255))) // 8 bit alpha

	return c
}

// Mix blends a towards b by t, in [0, 1].
func Mix(a, b Color, t float64) Color {
	lerp := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t))
	}

	return Color{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: lerp(a.A, b.A)}
}

// Luma returns the perceived lightness of a color, in [0, 1].
//
//nolint:mnd // the arithmetic of pixels and colors
func (c Color) Luma() float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255 // Rec. 709
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// Metrics describe the window: its size in cells and the size of that area in pixels.
type Metrics struct {
	Cols, Rows    int
	Width, Height int
}

// Valid reports whether the metrics describe a window something can be drawn in.
func (m Metrics) Valid() bool {
	return m.Cols > 0 && m.Rows > 0 && m.Width >= m.Cols && m.Height >= m.Rows
}

// CellWidth is the width of a cell in pixels, which need not be a whole number.
func (m Metrics) CellWidth() float64 { return float64(m.Width) / float64(m.Cols) }

// CellHeight is the height of a cell in pixels.
func (m Metrics) CellHeight() float64 { return float64(m.Height) / float64(m.Rows) }

// rowSpan returns the pixel rows a text row covers.
func (m Metrics) rowSpan(row int) (from, to int) {
	ch := m.CellHeight()

	return int(math.Round(float64(row) * ch)), int(math.Round(float64(row+1) * ch))
}

// Layer is where a shape is drawn relative to the text.
type Layer int

// The layers of a scene.
const (
	// Under is drawn below the text and below the backgrounds of the cells, so that
	// text that sets a background of its own, ie. a highlighted line, still shows it.
	Under Layer = iota
	// Over is drawn on top of the text.
	Over

	layerCount
)

// baseCell is the cell height the sizes given in points are designed for, and minPoint
// the least a point is in pixels, however few of them a layer is drawn with.
const (
	baseCell = 19
	minPoint = 0.25
)

type shapeKind uint8

const (
	// kindBox is a rounded rectangle, filled and/or stroked along its inner edge.
	kindBox shapeKind = iota
	// kindShadow is a rounded rectangle whose edge fades out over the blur.
	kindShadow
	// kindRect is a rectangle on whole pixels, without anti-aliasing.
	kindRect
	// kindSegs is a set of line segments with round caps.
	kindSegs
	// kindArea fills the area between a curve and a base line with a vertical fade.
	kindArea
	// kindMask is a mask scaled to the box.
	kindMask
)

// shape is one drawing instruction, in pixels.
type shape struct {
	kind  shapeKind
	erase bool
	// The box of the geometry, and the box of the pixels it can touch.
	x0, y0, x1, y1     float32
	bx0, by0, bx1, by1 float32
	r, sw, blur        float32
	fill, stroke       Color
	// pts holds the segments as x1, y1, x2, y2, or the curve of an area as x, y.
	pts  []float32
	mask *Mask
	hash uint64
}

// coarsePixels is about how many pixels the layer on top of the text is drawn with. It
// only dims what is under it, which takes no detail, and a terminal keeps a budget of
// image memory that is better spent on the layer below.
const coarsePixels = 1_000_000

// coarse returns the metrics of a window drawn with fewer pixels.
func (m Metrics) coarse() Metrics {
	// Every cell keeps at least a pixel.
	fits := func(k int) bool { return m.Width/k >= m.Cols && m.Height/k >= m.Rows }

	k := 1
	for (m.Width/k)*(m.Height/k) > coarsePixels && fits(k+1) {
		k++
	}

	m.Width, m.Height = m.Width/k, m.Height/k

	return m
}

// Scene is a frame of the pixel layer.
type Scene struct {
	// ms holds the metrics each layer is drawn with, m the ones of the layer shapes
	// are being added to, with the size of its cells and of a point in pixels.
	ms     [layerCount]Metrics
	m      Metrics
	cw, ch float64
	u      float64
	layer  Layer
	shapes [layerCount][]shape
	// clip bounds what the shapes that follow can touch, in pixels, when clipped is set.
	clip    [4]float32
	clipped bool
}

// NewScene starts an empty frame for a window.
func NewScene(m Metrics) *Scene {
	s := &Scene{}
	s.ms[Under], s.ms[Over] = m, m.coarse()

	return s.On(Under)
}

// On selects the layer the shapes that follow are drawn on.
func (s *Scene) On(l Layer) *Scene {
	s.layer, s.m = l, s.ms[l]
	s.cw, s.ch = s.m.CellWidth(), s.m.CellHeight()
	s.u = math.Max(s.ch/baseCell, minPoint)

	return s
}

// X and Y convert cells to pixels.
func (s *Scene) X(cells float64) float64 { return cells * s.cw }

// Y converts rows to pixels.
func (s *Scene) Y(rows float64) float64 { return rows * s.ch }

// Pt converts points, the unit of radii and line widths, to pixels.
func (s *Scene) Pt(points float64) float64 { return points * s.u }

// Clip limits the shapes that follow to a rectangle given in cells, until Unclip.
func (s *Scene) Clip(x, y, w, h float64) {
	s.clip = [4]float32{
		f32(math.Round(s.X(x))), f32(math.Round(s.Y(y))),
		f32(math.Round(s.X(x + w))), f32(math.Round(s.Y(y + h))),
	}
	s.clipped = true
}

// Unclip lifts the limit set by Clip.
func (s *Scene) Unclip() { s.clipped = false }

func (s *Scene) add(sh *shape) {
	if s.clipped {
		sh.bx0, sh.by0 = max(sh.bx0, s.clip[0]), max(sh.by0, s.clip[1])
		sh.bx1, sh.by1 = min(sh.bx1, s.clip[2]), min(sh.by1, s.clip[3])
	}

	if sh.bx1 <= sh.bx0 || sh.by1 <= sh.by0 {
		return
	}

	sh.hash = hashShape(sh)
	s.shapes[s.layer] = append(s.shapes[s.layer], *sh)
}

func f32(v float64) float32 { return float32(v) }

// Box draws a rounded rectangle. Position and size are in cells, the radius and the width
// of the stroke in points. The stroke runs along the inside of the edge.
func (s *Scene) Box(x, y, w, h, radius float64, fill, stroke Color, strokeWidth float64) {
	s.boxPx(
		s.X(x),
		s.Y(y),
		s.X(x+w),
		s.Y(y+h),
		s.Pt(radius),
		fill,
		stroke,
		s.Pt(strokeWidth),
		false,
	)
}

// Fill draws a filled rounded rectangle.
func (s *Scene) Fill(x, y, w, h, radius float64, fill Color) {
	s.Box(x, y, w, h, radius, fill, Color{}, 0)
}

// Outline draws the edge of a rounded rectangle, one point wide.
func (s *Scene) Outline(x, y, w, h, radius float64, stroke Color) {
	s.Box(x, y, w, h, radius, Color{}, stroke, 1)
}

// Halo draws a soft ring just outside a rounded rectangle, the way a focused control or a
// selected item is marked.
func (s *Scene) Halo(x, y, w, h, radius, width float64, c Color) {
	grow := s.Pt(width)
	s.boxPx(s.X(x)-grow, s.Y(y)-grow, s.X(x+w)+grow, s.Y(y+h)+grow, s.Pt(radius)+grow,
		Color{}, c, grow, false)
}

// Punch makes a rounded rectangle of the layer transparent again.
//
//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) Punch(x, y, w, h, radius float64) {
	s.boxPx(s.X(x), s.Y(y), s.X(x+w), s.Y(y+h), s.Pt(radius),
		Color{A: 255}, Color{}, 0, true) // opaque
}

func (s *Scene) boxPx(x0, y0, x1, y1, r float64, fill, stroke Color, sw float64, erase bool) {
	if fill.A == 0 && (stroke.A == 0 || sw <= 0) {
		return
	}

	// The edges sit on whole pixels, so that straight sides stay crisp.
	x0, y0, x1, y1 = math.Round(x0), math.Round(y0), math.Round(x1), math.Round(y1)
	if stroke.A > 0 && sw > 0 {
		sw = math.Max(1, math.Round(sw))
	}

	s.add(&shape{
		kind: kindBox, erase: erase,
		x0: f32(x0), y0: f32(y0), x1: f32(x1), y1: f32(y1),
		bx0: f32(x0), by0: f32(y0), bx1: f32(x1), by1: f32(y1),
		r: f32(r), sw: f32(sw), fill: fill, stroke: stroke,
	})
}

// Shadow draws the soft shadow of a rounded rectangle: blur is how far it fades, drop
// how far it is shifted down, both in points.
func (s *Scene) Shadow(x, y, w, h, radius, blur, drop float64, c Color) {
	if c.A == 0 {
		return
	}

	b := s.Pt(blur)
	x0, y0, x1, y1 := s.X(x), s.Y(y)+s.Pt(drop), s.X(x+w), s.Y(y+h)+s.Pt(drop)

	s.add(&shape{
		kind: kindShadow,
		x0:   f32(x0), y0: f32(y0), x1: f32(x1), y1: f32(y1),
		bx0: f32(x0 - b), by0: f32(y0 - b), bx1: f32(x1 + b), by1: f32(y1 + b),
		r: f32(s.Pt(radius)), blur: f32(b), fill: c,
	})
}

// HLine draws a horizontal hairline, one point thick, at a row boundary or anywhere else.
func (s *Scene) HLine(x, y, w float64, c Color) {
	t := math.Max(1, math.Round(s.u))
	y0 := math.Round(s.Y(y))
	s.rectPx(math.Round(s.X(x)), y0, math.Round(s.X(x+w)), y0+t, c)
}

// Rect fills a rectangle with square corners, snapped to whole pixels.
func (s *Scene) Rect(x, y, w, h float64, c Color) {
	s.rectPx(math.Round(s.X(x)), math.Round(s.Y(y)), math.Round(s.X(x+w)), math.Round(s.Y(y+h)), c)
}

func (s *Scene) rectPx(x0, y0, x1, y1 float64, c Color) {
	if c.A == 0 {
		return
	}

	s.add(&shape{
		kind: kindRect,
		x0:   f32(x0), y0: f32(y0), x1: f32(x1), y1: f32(y1),
		bx0: f32(x0), by0: f32(y0), bx1: f32(x1), by1: f32(y1),
		fill: c,
	})
}

// Dot draws a filled circle. The center is in cells, the radius in points.
func (s *Scene) Dot(cx, cy, radius float64, c Color) {
	s.circlePx(s.X(cx), s.Y(cy), s.Pt(radius), c, Color{}, 0)
}

// Ring draws the outline of a circle.
func (s *Scene) Ring(cx, cy, radius, width float64, c Color) {
	s.circlePx(s.X(cx), s.Y(cy), s.Pt(radius), Color{}, c, s.Pt(width))
}

func (s *Scene) circlePx(cx, cy, r float64, fill, stroke Color, sw float64) {
	if fill.A == 0 && (stroke.A == 0 || sw <= 0) {
		return
	}

	s.add(&shape{
		kind: kindBox,
		x0:   f32(cx - r), y0: f32(cy - r), x1: f32(cx + r), y1: f32(cy + r),
		bx0: f32(cx - r), by0: f32(cy - r), bx1: f32(cx + r), by1: f32(cy + r),
		r: f32(r), sw: f32(sw), fill: fill, stroke: stroke,
	})
}

// Point is a position in cells.
type Point struct{ X, Y float64 }

//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) polylinePx(px [][2]float64, width float64, c Color) {
	if len(px) < 2 { // a line needs two points
		return
	}

	segs := make([]float32, 0, 4*(len(px)-1)) // four coordinates a segment
	for i := 1; i < len(px); i++ {
		segs = append(segs, f32(px[i-1][0]), f32(px[i-1][1]), f32(px[i][0]), f32(px[i][1]))
	}

	s.segsPx(segs, width, c)
}

func (s *Scene) segsPx(segs []float32, width float64, c Color) {
	if c.A == 0 || len(segs) == 0 {
		return
	}

	half := f32(width/2 + 1) // half the width and the anti-aliased edge
	sh := shape{kind: kindSegs, sw: f32(width), fill: c, pts: segs}
	sh.bx0, sh.by0 = float32(math.Inf(1)), float32(math.Inf(1))
	sh.bx1, sh.by1 = float32(math.Inf(-1)), float32(math.Inf(-1))

	for i := 0; i < len(segs); i += 2 {
		sh.bx0, sh.bx1 = min(sh.bx0, segs[i]-half), max(sh.bx1, segs[i]+half)
		sh.by0, sh.by1 = min(sh.by0, segs[i+1]-half), max(sh.by1, segs[i+1]+half)
	}

	s.add(&sh)
}

// Route draws a line through points given in cells, with its corners rounded by a radius
// in points. A dash length above zero, in points, draws it dashed.
//
//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) Route(pts []Point, width, radius, dash float64, c Color) {
	if len(pts) < 2 { // a line needs two points
		return
	}

	const steps = 6

	r := s.Pt(radius)
	px := make([][2]float64, 0, len(pts)*steps)
	px = append(px, [2]float64{s.X(pts[0].X), s.Y(pts[0].Y)})

	for i := 1; i+1 < len(pts); i++ {
		a := [2]float64{s.X(pts[i-1].X), s.Y(pts[i-1].Y)}
		p := [2]float64{s.X(pts[i].X), s.Y(pts[i].Y)}
		b := [2]float64{s.X(pts[i+1].X), s.Y(pts[i+1].Y)}
		la, lb := math.Hypot(a[0]-p[0], a[1]-p[1]), math.Hypot(b[0]-p[0], b[1]-p[1])

		// The corner is cut off at most half way along each of its sides.
		cut := math.Min(r, math.Min(la, lb)/2) // half a side
		if cut < 1 {
			px = append(px, p)

			continue
		}

		from := [2]float64{p[0] + (a[0]-p[0])/la*cut, p[1] + (a[1]-p[1])/la*cut}
		to := [2]float64{p[0] + (b[0]-p[0])/lb*cut, p[1] + (b[1]-p[1])/lb*cut}

		for k := 0; k <= steps; k++ {
			t := float64(k) / steps
			u := 1 - t
			px = append(px, [2]float64{
				u*u*from[0] + 2*u*t*p[0] + t*t*to[0], // a quadratic curve
				u*u*from[1] + 2*u*t*p[1] + t*t*to[1], // a quadratic curve
			})
		}
	}

	last := pts[len(pts)-1]
	px = append(px, [2]float64{s.X(last.X), s.Y(last.Y)})

	if dash > 0 {
		s.segsPx(dashes(px, s.Pt(dash), s.Pt(dash)), s.Pt(width), c)

		return
	}

	s.polylinePx(px, s.Pt(width), c)
}

// dashes cuts a line through points into dashes of the given length and gap.
func dashes(pts [][2]float64, on, off float64) []float32 {
	var segs []float32

	pos, drawing := 0.0, true

	for i := 1; i < len(pts); i++ {
		ax, ay, bx, by := pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]
		length := math.Hypot(bx-ax, by-ay)

		for done := 0.0; done < length; {
			limit := on
			if !drawing {
				limit = off
			}

			step := math.Min(limit-pos, length-done)
			if drawing {
				t0, t1 := done/length, (done+step)/length
				segs = append(segs,
					f32(ax+(bx-ax)*t0), f32(ay+(by-ay)*t0), f32(ax+(bx-ax)*t1), f32(ay+(by-ay)*t1))
			}

			done += step
			pos += step

			if pos >= limit {
				pos, drawing = 0, !drawing
			}
		}
	}

	return segs
}

// Arc draws part of a circle outline, from a start angle over a sweep, both in turns
// (1 is the full circle, 0 points up, clockwise).
//
//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) Arc(cx, cy, radius, width, start, sweep float64, c Color) {
	const steps = 24

	n := max(2, int(math.Ceil(steps*math.Abs(sweep)))) // at least one piece
	pts := make([][2]float64, 0, n+1)
	px, py, r := s.X(cx), s.Y(cy), s.Pt(radius)

	for i := 0; i <= n; i++ {
		a := (start + sweep*float64(i)/float64(n)) * 2 * math.Pi // turns to radians
		pts = append(pts, [2]float64{px + r*math.Sin(a), py - r*math.Cos(a)})
	}

	s.polylinePx(pts, s.Pt(width), c)
}

// Chart draws the recent history of a value as a smooth line over a fading fill, in the
// rectangle given in cells. Values are scaled to limit; a limit that is not positive
// scales them to their peak.
//
//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) Chart(
	x, y, w, h float64,
	series []float64,
	limit, width float64,
	c Color,
	fill float64,
) {
	if len(series) < 2 || w <= 0 || h <= 0 { // a line needs two points
		return
	}

	if limit <= 0 {
		for _, v := range series {
			limit = math.Max(limit, v)
		}
	}

	if limit <= 0 {
		limit = 1
	}

	x0, y0, pw, ph := s.X(x), s.Y(y), s.X(w), s.Y(h)
	lw := s.Pt(width)
	// Keep the line inside the rectangle, also at the extremes.
	top, bottom := y0+lw, y0+ph-lw

	knots := make([][2]float64, len(series))
	for i, v := range series {
		knots[i] = [2]float64{
			x0 + pw*float64(i)/float64(len(series)-1),
			bottom - (bottom-top)*clamp(v/limit, 0, 1),
		}
	}

	pts := smooth(knots)

	if fill > 0 {
		area := make([]float32, 0, 2*len(pts)) // two coordinates a point
		for _, p := range pts {
			area = append(area, f32(p[0]), f32(p[1]))
		}

		s.add(&shape{
			kind: kindArea,
			x0:   f32(x0), y0: f32(y0), x1: f32(x0 + pw), y1: f32(y0 + ph),
			bx0: f32(x0), by0: f32(y0), bx1: f32(x0 + pw), by1: f32(y0 + ph),
			fill: c.Alpha(fill), pts: area,
		})
	}

	s.polylinePx(pts, lw, c)
}

// smooth rounds a line through knots off: between two knots the line leaves and arrives
// level, which keeps it from overshooting the values.
//
//nolint:mnd // the arithmetic of pixels and colors
func smooth(knots [][2]float64) [][2]float64 {
	const steps = 6

	pts := make([][2]float64, 0, steps*len(knots))
	pts = append(pts, knots[0])

	for i := 1; i < len(knots); i++ {
		a, b := knots[i-1], knots[i]
		if b[0]-a[0] < steps {
			pts = append(pts, b)

			continue
		}

		for k := 1; k <= steps; k++ {
			t := float64(k) / steps
			e := t * t * (3 - 2*t) // smoothstep
			pts = append(pts, [2]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*e})
		}
	}

	return pts
}

func hashShape(sh *shape) uint64 {
	h := fnv.New64a()

	var buf [8]byte

	put := func(v float32) {
		binary.LittleEndian.PutUint32(buf[:4], math.Float32bits(v))
		_, _ = h.Write(buf[:4])
	}

	erase := byte(0)
	if sh.erase {
		erase = 1
	}

	_, _ = h.Write([]byte{
		byte(sh.kind), erase,
		sh.fill.R, sh.fill.G, sh.fill.B, sh.fill.A,
		sh.stroke.R, sh.stroke.G, sh.stroke.B, sh.stroke.A,
	})

	for _, v := range [...]float32{
		sh.x0, sh.y0, sh.x1, sh.y1, sh.bx0, sh.by0, sh.bx1, sh.by1, sh.r, sh.sw, sh.blur,
	} {
		put(v)
	}

	for _, v := range sh.pts {
		put(v)
	}

	if sh.mask != nil {
		binary.LittleEndian.PutUint64(buf[:], sh.mask.id)
		_, _ = h.Write(buf[:])
	}

	return h.Sum64()
}
