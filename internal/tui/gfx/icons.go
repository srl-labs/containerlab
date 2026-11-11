// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

// Icon names a small line drawing.
type Icon int

// The icons.
const (
	IconRouter Icon = iota
	IconSwitch
	IconHost
	IconCheck
)

// iconGrid is the size of the square the icons are designed in.
const iconGrid = 16

// iconArt describes an icon on the design grid: lines through points, circles as center
// and radius, and rounded rectangles as position, size and corner radius.
type iconArt struct {
	lines   [][]float64
	circles [][3]float64
	rects   [][5]float64
}

// icons holds the drawings, in coordinates on the design grid.
var icons = map[Icon]iconArt{
	IconRouter: {
		circles: [][3]float64{{8, 8, 6.6}},
		lines: [][]float64{
			{8, 4, 8, 12},
			{4, 8, 12, 8},
			{6.4, 5.6, 8, 4, 9.6, 5.6},
			{6.4, 10.4, 8, 12, 9.6, 10.4},
		},
	},
	IconSwitch: {
		rects: [][5]float64{{1.4, 3.6, 13.2, 8.8, 2}},
		lines: [][]float64{
			{4.5, 6.8, 11, 6.8, 9.4, 5.2},
			{11.5, 9.4, 5, 9.4, 6.6, 11},
		},
	},
	IconHost: {
		rects: [][5]float64{{2, 2.8, 12, 8, 1.5}},
		lines: [][]float64{{5.5, 13.4, 10.5, 13.4}, {8, 10.8, 8, 13.4}},
	},
	IconCheck: {
		lines: [][]float64{{3.4, 8.4, 6.6, 11.6, 12.6, 4.8}},
	},
}

// Icon draws an icon centered on a position in cells. Its size and the width of its
// lines are in points.
//
//nolint:mnd // the arithmetic of pixels and colors
func (s *Scene) Icon(icon Icon, cx, cy, size, width float64, c Color) {
	art, ok := icons[icon]
	if !ok || c.A == 0 {
		return
	}

	k := s.Pt(size) / iconGrid
	ox, oy := s.X(cx)-s.Pt(size)/2, s.Y(cy)-s.Pt(size)/2 // centered
	lw := s.Pt(width)

	var segs []float32

	for _, line := range art.lines {
		for i := 2; i+1 < len(line); i += 2 {
			segs = append(segs,
				f32(ox+line[i-2]*k), f32(oy+line[i-1]*k), f32(ox+line[i]*k), f32(oy+line[i+1]*k))
		}
	}

	s.segsPx(segs, lw, c)

	for _, circle := range art.circles {
		// The stroke of a box runs inside its edge: grow it by half the line.
		r := circle[2]*k + lw/2 // half the line
		s.circlePx(ox+circle[0]*k, oy+circle[1]*k, r, Color{}, c, lw)
	}

	for _, rect := range art.rects {
		half := lw / 2 // half the line
		x0, y0 := ox+rect[0]*k-half, oy+rect[1]*k-half
		x1, y1 := ox+(rect[0]+rect[2])*k+half, oy+(rect[1]+rect[3])*k+half

		s.add(&shape{
			kind: kindBox,
			x0:   f32(x0), y0: f32(y0), x1: f32(x1), y1: f32(y1),
			bx0: f32(x0), by0: f32(y0), bx1: f32(x1), by1: f32(y1),
			r: f32(rect[4]*k + half), sw: f32(lw), stroke: c,
		})
	}
}
