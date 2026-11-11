// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"hash/fnv"
	"image"
	"math"
)

// Mask is a drawing given as the opacity of its pixels, ie. a logo. It is drawn in any
// color and at any size.
type Mask struct {
	w, h  int
	alpha []uint8
	id    uint64
}

// NewMask takes the opacity of the pixels of an image.
//
//nolint:mnd // the arithmetic of pixels and colors
func NewMask(img image.Image) *Mask {
	b := img.Bounds()
	m := &Mask{w: b.Dx(), h: b.Dy(), alpha: make([]uint8, b.Dx()*b.Dy())}

	for y := range m.h {
		for x := range m.w {
			_, _, _, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			m.alpha[y*m.w+x] = uint8(a >> 8) //nolint:gosec // 16 to 8 bit
		}
	}

	h := fnv.New64a()
	_, _ = h.Write(m.alpha)
	m.id = h.Sum64()

	return m
}

// Aspect returns the width of the mask for a height of one.
func (m *Mask) Aspect() float64 {
	if m == nil || m.h == 0 {
		return 0
	}

	return float64(m.w) / float64(m.h)
}

// Stamp draws a mask in a color, centered on a position in cells and scaled to a height
// in points.
func (s *Scene) Stamp(m *Mask, cx, cy, height float64, c Color) {
	s.StampAt(m, cx-s.Pt(height)*m.Aspect()/s.cw/2, cy, height, c) // centered
}

// StampAt draws a mask in a color, starting at a column and centered on a row, both in
// cells, and scaled to a height in points.
func (s *Scene) StampAt(m *Mask, x, cy, height float64, c Color) {
	if m == nil || m.h == 0 || c.A == 0 {
		return
	}

	h := s.Pt(height)
	w := h * m.Aspect()
	x0, y0 := math.Round(s.X(x)), math.Round(s.Y(cy)-h/2) // centered

	s.add(&shape{
		kind: kindMask, mask: m, fill: c,
		x0: f32(x0), y0: f32(y0), x1: f32(x0 + math.Round(w)), y1: f32(y0 + math.Round(h)),
		bx0: f32(x0), by0: f32(y0), bx1: f32(x0 + math.Round(w)), by1: f32(y0 + math.Round(h)),
	})
}

// stamp draws a mask scaled to the box of the shape. Each pixel takes the average of the
// part of the mask it covers.
//
//nolint:mnd // the arithmetic of pixels and colors
func (b *strip) stamp(sh *shape) {
	m := sh.mask
	y0, y1 := b.rows(sh)
	x0, x1 := b.columns(sh)
	kx := float64(m.w) / float64(sh.x1-sh.x0)
	ky := float64(m.h) / float64(sh.y1-sh.y0)

	for y := y0; y < y1; y++ {
		sy0 := int((float64(y) - float64(sh.y0)) * ky)
		sy1 := max(sy0+1, int(math.Ceil((float64(y)+1-float64(sh.y0))*ky)))

		for x := x0; x < x1; x++ {
			sx0 := int((float64(x) - float64(sh.x0)) * kx)
			sx1 := max(sx0+1, int(math.Ceil((float64(x)+1-float64(sh.x0))*kx)))

			sum, n := 0, 0

			for sy := max(0, sy0); sy < min(m.h, sy1); sy++ {
				for sx := max(0, sx0); sx < min(m.w, sx1); sx++ {
					sum += int(m.alpha[sy*m.w+sx])
					n++
				}
			}

			if n > 0 && sum > 0 {
				// 8 bit.
				b.put(((y-b.oy)*b.w+x)*bytesPerPixel, sh.fill, float32(sum)/float32(n*255), false)
			}
		}
	}
}
