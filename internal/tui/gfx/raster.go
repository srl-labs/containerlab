// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"image"
	"math"
)

// bytesPerPixel is the size of an RGBA pixel.
const bytesPerPixel = 4

// strip is the pixels of one text row of a layer, as premultiplied RGBA. Shapes are drawn
// from their distance to each pixel, which gives smooth edges without a path rasterizer.
type strip struct {
	w, h int
	// oy is the pixel row of the scene the strip starts at.
	oy  int
	pix []uint8
	cov []float32
	// cols and spans are scratch space: columns worth looking at, and for every row
	// the columns something was drawn in.
	cols  []int
	spans [][2]int
}

func (b *strip) reset(w, h, oy int) {
	b.w, b.h, b.oy = w, h, oy

	n := w * h * bytesPerPixel
	if cap(b.pix) < n {
		b.pix = make([]uint8, n)
	}

	b.pix = b.pix[:n]
	clear(b.pix)
}

// put composites a color over a pixel, or with erase takes the pixel away.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) put(i int, c Color, cov float32, erase bool) {
	a := cov * float32(c.A) / 255 // 8 bit alpha
	if a <= 0 {
		return
	}

	p := b.pix[i : i+bytesPerPixel : i+bytesPerPixel]
	k := 1 - a

	if erase {
		p[0], p[1] = uint8(float32(p[0])*k), uint8(float32(p[1])*k)
		p[2], p[3] = uint8(float32(p[2])*k), uint8(float32(p[3])*k)

		return
	}

	if k <= 0.002 { // opaque for all purposes
		p[0], p[1], p[2], p[3] = c.R, c.G, c.B, 255 // opaque

		return
	}

	p[0] = uint8(float32(c.R)*a + float32(p[0])*k + 0.5) // rounding
	p[1] = uint8(float32(c.G)*a + float32(p[1])*k + 0.5) // rounding
	p[2] = uint8(float32(c.B)*a + float32(p[2])*k + 0.5) // rounding
	p[3] = uint8(255*a + float32(p[3])*k + 0.5)          // rounding
}

// span fills a run of pixels of one row with a color at full coverage. An opaque color,
// which is what most of a frame is made of, is copied in rather than blended.
func (b *strip) span(row, x0, x1 int, c Color, erase bool) {
	if x1 <= x0 {
		return
	}

	i, end := row+x0*bytesPerPixel, row+x1*bytesPerPixel

	if erase || c.A != 255 { // not opaque
		for ; i < end; i += bytesPerPixel {
			b.put(i, c, 1, erase)
		}

		return
	}

	run := b.pix[i:end]
	run[0], run[1], run[2], run[3] = c.R, c.G, c.B, c.A

	for n := bytesPerPixel; n < len(run); n *= 2 { // doubling what is filled
		copy(run[n:], run[:n])
	}
}

// dots draws a grid of dots. Each pixel is measured against the dot of the grid cell it
// lies in.
func (b *strip) dots(sh *shape) {
	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)
	step := sh.sw
	half := step / 2 //nolint:mnd // the middle of a grid cell

	if y0 >= y1 || x0 >= x1 {
		return
	}

	// Most pixels are nowhere near a dot: the columns that are, and how far each is
	// from the middle of its dot, are worked out once for all rows.
	near := b.scratch(x1 - x0)
	cols := b.cols[:0]

	for x := x0; x < x1; x++ {
		dx := float32(
			math.Mod(float64(float32(x)+0.5-sh.x0), float64(step)),
		) - half // pixel center
		if abs32(dx) > sh.r+1 {
			continue
		}

		near[x-x0] = dx
		cols = append(cols, x)
	}

	b.cols = cols

	for y := y0; y < y1; y++ {
		dy := float32(
			math.Mod(float64(float32(y)+0.5-sh.y0), float64(step)),
		) - half // pixel center
		if abs32(dy) > sh.r+1 {
			continue
		}

		for _, x := range cols {
			dx := near[x-x0]

			if c := clamp01(sh.r + 0.5 - sqrt32(dx*dx+dy*dy)); c > 0 { // anti-aliasing
				b.put(((y-b.oy)*b.w+x)*bytesPerPixel, sh.fill, c, false)
			}
		}
	}
}

// rows returns the pixel rows of the strip a shape can touch, as scene coordinates.
func (b *strip) rows(sh *shape) (from, to int) {
	return max(
			b.oy,
			int(math.Floor(float64(sh.by0))),
		), min(b.oy+b.h,
			int(math.Ceil(float64(sh.by1))),
		)
}

func (b *strip) columns(sh *shape) (from, to int) {
	return max(0, int(math.Floor(float64(sh.bx0)))), min(b.w, int(math.Ceil(float64(sh.bx1))))
}

func (b *strip) draw(sh *shape) {
	switch sh.kind {
	case kindBox, kindShadow:
		b.box(sh)
	case kindRect:
		b.rect(sh)
	case kindSegs:
		b.segs(sh)
	case kindArea:
		b.area(sh)
	case kindMask:
		b.stamp(sh)
	case kindText:
		b.write(sh)
	case kindDots:
		b.dots(sh)
	}
}

func (b *strip) rect(sh *shape) {
	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)

	for y := y0; y < y1; y++ {
		b.span((y-b.oy)*b.w*bytesPerPixel, x0, x1, sh.fill, sh.erase)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}

	return v
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}

	if v > 1 {
		return 1
	}

	return v
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

// box draws a rounded rectangle or its shadow. Each row has a middle stretch that is
// fully inside, filled without looking at the pixels one by one; only the rim is
// measured.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) box(sh *shape) {
	cx, cy := (sh.x0+sh.x1)/2, (sh.y0+sh.y1)/2 // center
	hx, hy := (sh.x1-sh.x0)/2, (sh.y1-sh.y0)/2 // half sizes
	r := min(sh.r, hx, hy)
	shadow := sh.kind == kindShadow
	stroked := !shadow && sh.stroke.A > 0 && sh.sw > 0

	// margin is how far inside the edge a pixel has to be to count as the middle.
	margin := float32(1)
	if stroked {
		margin += sh.sw
	}

	if shadow {
		margin = sh.blur
	}

	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)

	for y := y0; y < y1; y++ {
		dy := abs32(float32(y) + 0.5 - cy) // pixel center
		row := ((y - b.oy) * b.w) * bytesPerPixel

		// The middle stretch of the row, empty when there is none.
		mid0, mid1 := x1, x1

		if half := insideHalf(dy, hx, hy, r, margin); half >= 0 {
			mid0 = max(x0, int(math.Ceil(float64(cx-half-0.5))))    // pixel center
			mid1 = min(x1, int(math.Floor(float64(cx+half-0.5)))+1) // pixel center

			if mid0 >= mid1 {
				mid0, mid1 = x1, x1
			}
		}

		for x := x0; x < x1; x++ {
			if x == mid0 {
				if sh.fill.A > 0 {
					b.span(row, mid0, mid1, sh.fill, sh.erase)
				}

				x = mid1 - 1

				continue
			}

			qx := abs32(float32(x)+0.5-cx) - (hx - r) // pixel center
			qy := dy - (hy - r)
			d := min(max(qx, qy), 0) - r

			if qx > 0 || qy > 0 {
				ox, oy := max(qx, 0), max(qy, 0)
				d += sqrt32(ox*ox + oy*oy)
			}

			i := row + x*bytesPerPixel

			if shadow {
				// Fades over twice the blur.
				t := clamp01((d + sh.blur) / (2 * sh.blur))
				b.put(i, sh.fill, (1-t)*(1-t), false)

				continue
			}

			inside := clamp01(0.5 - d) // half a pixel of anti-aliasing
			if inside <= 0 {
				continue
			}

			b.put(i, sh.fill, inside, sh.erase)

			if stroked {
				// Anti-aliasing.
				b.put(i, sh.stroke, inside*clamp01(0.5+d+sh.sw), false)
			}
		}
	}
}

// insideHalf returns half the width of the stretch of a row that lies at least margin
// inside a rounded rectangle, or a negative value when the row has none. dy is the
// distance of the row from the center.
func insideHalf(dy, hx, hy, r, margin float32) float32 {
	switch {
	case dy > hy-margin || hx <= margin:
		return -1
	case dy <= hy-r:
		return hx - margin
	}

	// The row crosses the rounded corners.
	qy := dy - (hy - r)
	if in := r - margin; in > qy {
		return hx - r + sqrt32(in*in-qy*qy)
	}

	return hx - r
}

// scratch returns a zeroed buffer of coverage values.
func (b *strip) scratch(n int) []float32 {
	if cap(b.cov) < n {
		b.cov = make([]float32, n)
	}

	b.cov = b.cov[:n]
	clear(b.cov)

	return b.cov
}

// segs draws line segments with round ends. The coverage of all of them is collected
// first, so that where they meet the color is laid down once.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) segs(sh *shape) {
	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)

	if y0 >= y1 || x0 >= x1 {
		return
	}

	w := x1 - x0
	cov := b.scratch(w * (y1 - y0))
	half := sh.sw / 2 // half the width
	reach := half + 1

	// For every row the columns that got coverage, so that the rest is not looked at.
	if cap(b.spans) < y1-y0 {
		b.spans = make([][2]int, y1-y0)
	}

	spans := b.spans[:y1-y0]
	for i := range spans {
		spans[i] = [2]int{x1, x0}
	}

	for s := 0; s+3 < len(sh.pts); s += 4 {
		ax, ay, bx, by := sh.pts[s], sh.pts[s+1], sh.pts[s+2], sh.pts[s+3]

		sy0 := max(y0, int(math.Floor(float64(min(ay, by)-reach))))
		sy1 := min(y1, int(math.Ceil(float64(max(ay, by)+reach))))
		sx0 := max(x0, int(math.Floor(float64(min(ax, bx)-reach))))
		sx1 := min(x1, int(math.Ceil(float64(max(ax, bx)+reach))))

		dx, dy := bx-ax, by-ay
		length2 := dx*dx + dy*dy

		for y := sy0; y < sy1; y++ {
			py := float32(y) + 0.5 - ay // pixel center

			// A line that slants passes a row in a few columns only: the part of it
			// within reach of the row tells which.
			rx0, rx1 := sx0, sx1

			if dy != 0 {
				t0, t1 := (py-reach)/dy, (py+reach)/dy
				if t0 > t1 {
					t0, t1 = t1, t0
				}

				t0, t1 = max(t0, 0), min(t1, 1)
				if t0 > t1 {
					continue
				}

				from, to := ax+dx*t0, ax+dx*t1
				rx0 = max(sx0, int(math.Floor(float64(min(from, to)-reach))))
				rx1 = min(sx1, int(math.Ceil(float64(max(from, to)+reach))))
			}

			if rx0 >= rx1 {
				continue
			}

			span := &spans[y-y0]
			span[0], span[1] = min(span[0], rx0), max(span[1], rx1)

			for x := rx0; x < rx1; x++ {
				px := float32(x) + 0.5 - ax // pixel center

				t := float32(0)
				if length2 > 0 {
					t = clamp01((px*dx + py*dy) / length2)
				}

				ex, ey := px-dx*t, py-dy*t
				c := clamp01(half + 0.5 - sqrt32(ex*ex+ey*ey)) // anti-aliasing

				if i := (y-y0)*w + (x - x0); c > cov[i] {
					cov[i] = c
				}
			}
		}
	}

	for y := y0; y < y1; y++ {
		for x := spans[y-y0][0]; x < spans[y-y0][1]; x++ {
			if c := cov[(y-y0)*w+(x-x0)]; c > 0 {
				b.put(((y-b.oy)*b.w+x)*bytesPerPixel, sh.fill, c, sh.erase)
			}
		}
	}
}

// area fills what lies between a curve and the bottom of the shape, fading out downwards.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) area(sh *shape) {
	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)
	top, base := sh.y0, sh.y1

	if base <= top || len(sh.pts) < 4 { // two points
		return
	}

	seg := 0

	for x := x0; x < x1; x++ {
		px := float32(x) + 0.5 // pixel center

		for seg+5 < len(sh.pts) && sh.pts[seg+2] < px {
			seg += 2
		}

		ax, ay, bx, by := sh.pts[seg], sh.pts[seg+1], sh.pts[seg+2], sh.pts[seg+3]

		curve := ay
		if bx > ax {
			curve = ay + (by-ay)*clamp01((px-ax)/(bx-ax))
		}

		for y := max(y0, int(math.Floor(float64(curve)))); y < y1; y++ {
			edge := clamp01(float32(y) + 1 - curve)
			fade := 1 - clamp01((float32(y)+0.5-top)/(base-top)) // pixel center

			b.put(((y-b.oy)*b.w+x)*bytesPerPixel, sh.fill, edge*fade, false)
		}
	}
}

// render draws the shapes that touch the strip and reports whether anything is visible.
func (b *strip) render(shapes []shape) bool {
	for i := range shapes {
		sh := &shapes[i]
		if int(math.Ceil(float64(sh.by1))) <= b.oy ||
			int(math.Floor(float64(sh.by0))) >= b.oy+b.h {
			continue
		}

		b.draw(sh)
	}

	for i := 3; i < len(b.pix); i += bytesPerPixel {
		if b.pix[i] != 0 {
			return true
		}
	}

	return false
}

// opaque is the alpha of a pixel that lets nothing through.
const opaque = 255

// blend lays another strip of the same size on top of this one.
func (b *strip) blend(top *strip) {
	for i := 0; i+3 < len(b.pix) && i+3 < len(top.pix); i += bytesPerPixel {
		a := uint32(top.pix[i+3])
		if a == 0 {
			continue
		}

		for k := range bytesPerPixel {
			under := uint32(b.pix[i+k]) * (opaque - a) / opaque
			b.pix[i+k] = uint8(min(opaque, uint32(top.pix[i+k])+under))
		}
	}
}

// straight returns the pixels with straight alpha, the form the terminal expects. It
// converts in place.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) straight() []uint8 {
	for i := 0; i+3 < len(b.pix); i += bytesPerPixel {
		a := uint32(b.pix[i+3])
		if a == 0 || a == 255 { // nothing to undo
			continue
		}

		for k := range 3 {
			// Undo the premultiplication.
			b.pix[i+k] = uint8(min(255, (uint32(b.pix[i+k])*255+a/2)/a))
		}
	}

	return b.pix
}

// Image renders a whole layer of the scene, for previews and tests.
func (s *Scene) Image(l Layer) *image.NRGBA {
	m := s.ms[l]
	img := image.NewNRGBA(image.Rect(0, 0, m.Width, m.Height))

	var b strip

	for row := range m.Rows {
		from, to := m.rowSpan(row)
		b.reset(m.Width, to-from, from)
		b.render(s.shapes[l])
		copy(img.Pix[from*img.Stride:], b.straight())
	}

	return img
}
