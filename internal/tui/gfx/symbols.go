// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"math"
	"regexp"
	"strconv"
	"sync"
)

// Symbol names one of the node icons of the containerlab app: a line drawing in a
// rounded hexagon.
type Symbol string

// The node icons, by the names the app knows them by.
const (
	SymbolRouter     Symbol = "router"
	SymbolSwitch     Symbol = "switch"
	SymbolLeaf       Symbol = "leaf"
	SymbolSpine      Symbol = "spine"
	SymbolSuperSpine Symbol = "super-spine"
	SymbolDCGW       Symbol = "dcgw"
	SymbolFirewall   Symbol = "firewall"
	SymbolContainer  Symbol = "container"
	SymbolVM         Symbol = "vm"
	SymbolServer     Symbol = "server"
	SymbolClient     Symbol = "client"
	SymbolController Symbol = "controller"
	SymbolCloud      Symbol = "cloud"
	SymbolPON        Symbol = "pon"
	SymbolRGW        Symbol = "rgw"
	SymbolUE         Symbol = "ue"
)

// The artwork is drawn on a grid of 64 units, of which the icon shows the 56 in the
// middle. The frame and the symbols are the ones of the app (clab-ui, SvgGenerator.ts).
const (
	symbolGrid   = 64
	symbolView   = 56
	frameStroke  = 2.4
	symbolStroke = 2.2

	symbolFrame = "M29 6.6Q32 4.9 35 6.6L53 17Q56 18.7 56 22.2V41.8Q56 45.3 53 47L35 57.4" +
		"Q32 59.1 29 57.4L11 47Q8 45.3 8 41.8V22.2Q8 18.7 11 17Z"
)

// symbolSpec describes a symbol the way its SVG does: paths that are stroked, rectangles
// with rounded corners that are stroked (x, y, width, height, radius), dots and
// rectangles that are filled.
type symbolSpec struct {
	paths []string
	rects [][5]float64
	dots  [][3]float64
	fills [][5]float64
}

//nolint:lll // the artwork
var symbolSpecs = map[Symbol]symbolSpec{
	SymbolRouter: {
		paths: []string{
			"M28 28L21 21M21 27V21H27M36 28L43 21M37 21H43V27M28 36L21 43M21 37V43H27M36 36L43 43M37 43H43V37",
		},
	},
	SymbolSwitch: {paths: []string{"M20 26H44M38 20L44 26L38 32M44 38H20M26 32L20 38L26 44"}},
	SymbolLeaf: {
		paths: []string{"M28 29L21 22M21 28V22H27M36 29L43 22M37 22H43V28M21 36H43M21 42H43"},
	},
	SymbolSpine: {paths: []string{
		"M20 26H44M24 22L20 26L24 30M40 22L44 26L40 30M20 38H44M24 34L20 38L24 42M40 34L44 38L40 42",
	}},
	SymbolSuperSpine: {
		paths: []string{"M20 22H44M20 32H44M24 28L20 32L24 36M40 28L44 32L40 36M20 42H44"},
	},
	SymbolDCGW: {
		paths: []string{"M24 20V44M40 20V44M19 32H45M23 28L19 32L23 36M41 28L45 32L41 36"},
	},
	SymbolFirewall: {
		rects: [][5]float64{{20, 21, 24, 22, 2.5}},
		paths: []string{"M20 28.3H44M20 35.7H44M32 21V28.3M26 28.3V35.7M38 28.3V35.7M32 35.7V43"},
	},
	SymbolContainer: {
		paths: []string{"M24 21H21V43H24M40 21H43V43H40"},
		fills: [][5]float64{
			{27, 25, 4, 5, 0.8},
			{34, 25, 4, 5, 0.8},
			{27, 34, 4, 5, 0.8},
			{34, 34, 4, 5, 0.8},
		},
	},
	SymbolVM: {
		paths: []string{"M26 21H42Q45 21 45 24V37", "M25 32L28 35L25 38M32 38H34"},
		rects: [][5]float64{{19, 27, 20, 16, 2.5}},
	},
	SymbolServer: {
		rects: [][5]float64{{21, 20, 22, 10, 2}, {21, 34, 22, 10, 2}},
		paths: []string{"M25 25H30M25 39H30"},
		dots:  [][3]float64{{38, 25, 1.6}, {38, 39, 1.6}},
	},
	SymbolClient: {
		rects: [][5]float64{{19, 21, 26, 17, 2.5}},
		paths: []string{"M32 38V43M26 43H38"},
	},
	SymbolController: {
		paths: []string{"M24 20V44M32 20V44M40 20V44"},
		dots:  [][3]float64{{24, 36, 3}, {32, 26, 3}, {40, 33, 3}},
	},
	SymbolCloud: {
		paths: []string{"M25 41H40A5.5 5.5 0 0 0 40 30A8 8 0 0 0 24.3 31.05A5 5 0 0 0 25 41Z"},
	},
	SymbolPON: {
		paths: []string{"M19 32H27M27 32L40 23M27 32H40M27 32L40 41"},
		dots:  [][3]float64{{42.5, 22, 2.2}, {42.5, 32, 2.2}, {42.5, 42, 2.2}},
	},
	SymbolRGW: {
		paths: []string{
			"M20 31L32 21L44 31M24 28V43H40V28M29.2 36.7A4 4 0 0 1 34.8 36.7M26.7 34.2A7.5 7.5 0 0 1 37.3 34.2",
		},
		dots: [][3]float64{{32, 39.5, 1.5}},
	},
	SymbolUE: {
		rects: [][5]float64{{25, 19, 14, 26, 3}},
		paths: []string{"M30 40H34"},
	},
}

// symbolArt is a symbol ready to draw: lines through points of the grid, and what is
// filled.
type symbolArt struct {
	lines [][][2]float64
	dots  [][3]float64
	fills [][5]float64
}

// symbolArts returns the frame of the icons and the art of every symbol.
var symbolArts = sync.OnceValues(func() ([][][2]float64, map[Symbol]symbolArt) {
	arts := make(map[Symbol]symbolArt, len(symbolSpecs))

	for name, spec := range symbolSpecs {
		art := symbolArt{dots: spec.dots, fills: spec.fills}

		for _, p := range spec.paths {
			art.lines = append(art.lines, flattenPath(p)...)
		}

		for _, r := range spec.rects {
			art.lines = append(art.lines, flattenPath(roundedRectPath(r))...)
		}

		arts[name] = art
	}

	return flattenPath(symbolFrame), arts
})

// HasSymbol reports whether there is a node icon of that name.
func HasSymbol(name string) bool {
	_, ok := symbolSpecs[Symbol(name)]

	return ok
}

func roundedRectPath(r [5]float64) string {
	x, y, w, h, rx := r[0], r[1], r[2], r[3], r[4]
	n := func(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) + " " }

	return "M" + n(x+rx) + n(y) + "H" + n(x+w-rx) + "Q" + n(x+w) + n(y) + n(x+w) + n(y+rx) +
		"V" + n(y+h-rx) + "Q" + n(x+w) + n(y+h) + n(x+w-rx) + n(y+h) +
		"H" + n(x+rx) + "Q" + n(x) + n(y+h) + n(x) + n(y+h-rx) +
		"V" + n(y+rx) + "Q" + n(x) + n(y) + n(x+rx) + n(y) + "Z"
}

var pathToken = regexp.MustCompile(`[A-Za-z]|-?\d*\.?\d+`)

// curveParts is the number of straight pieces a curve of the artwork is drawn with.
const curveParts = 8

// flattenPath turns the data of an SVG path into lines through points. It knows the
// commands the artwork uses, all with absolute coordinates: M, L, H, V, Q, A and Z.
//
//nolint:gocyclo,funlen // one case a command
func flattenPath(data string) [][][2]float64 {
	tokens := pathToken.FindAllString(data, -1)

	var (
		lines [][][2]float64
		line  [][2]float64
		cmd   byte
		at    [2]float64
		start [2]float64
	)

	flush := func() {
		if len(line) > 1 {
			lines = append(lines, line)
		}

		line = nil
	}

	i := 0
	num := func() float64 {
		if i >= len(tokens) {
			return 0
		}

		v, _ := strconv.ParseFloat(tokens[i], 64)
		i++

		return v
	}

	for i < len(tokens) {
		if c := tokens[i][0]; (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			cmd = c
			i++
		}

		switch cmd {
		case 'M':
			flush()

			at = [2]float64{num(), num()}
			start = at
			line = append(line, at)
			cmd = 'L' // further pairs draw lines
		case 'L':
			at = [2]float64{num(), num()}
			line = append(line, at)
		case 'H':
			at[0] = num()
			line = append(line, at)
		case 'V':
			at[1] = num()
			line = append(line, at)
		case 'Q':
			c, to := [2]float64{num(), num()}, [2]float64{num(), num()}

			for k := 1; k <= curveParts; k++ {
				t := float64(k) / curveParts
				u := 1 - t
				line = append(line, [2]float64{
					u*u*at[0] + 2*u*t*c[0] + t*t*to[0], // a quadratic curve
					u*u*at[1] + 2*u*t*c[1] + t*t*to[1], // a quadratic curve
				})
			}

			at = to
		case 'A':
			rx, ry := num(), num()
			_ = num() // the rotation of the axes, which the artwork does not use
			large, sweep := num() != 0, num() != 0
			to := [2]float64{num(), num()}
			line = append(line, flattenArc(at, to, rx, ry, large, sweep)...)
			at = to
		case 'Z':
			line = append(line, start)
			at = start

			flush()
		default:
			i++
		}
	}

	flush()

	return lines
}

// flattenArc returns the points of an elliptical arc between two points, the way SVG
// defines it, without the first one.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func flattenArc(from, to [2]float64, rx, ry float64, large, sweep bool) [][2]float64 {
	if rx == 0 || ry == 0 {
		return [][2]float64{to}
	}

	// From the end points to the center, as the SVG specification gives it.
	dx, dy := (from[0]-to[0])/2, (from[1]-to[1])/2 //nolint:mnd // half the chord

	if scale := dx*dx/(rx*rx) + dy*dy/(ry*ry); scale > 1 {
		rx, ry = rx*math.Sqrt(scale), ry*math.Sqrt(scale)
	}

	num := rx*rx*ry*ry - rx*rx*dy*dy - ry*ry*dx*dx
	den := rx*rx*dy*dy + ry*ry*dx*dx

	k := 0.0
	if den > 0 && num > 0 {
		k = math.Sqrt(num / den)
	}

	if large == sweep {
		k = -k
	}

	cxp, cyp := k*rx*dy/ry, -k*ry*dx/rx
	cx, cy := cxp+(from[0]+to[0])/2, cyp+(from[1]+to[1])/2 //nolint:mnd // the middle of the chord

	a0 := math.Atan2((dy-cyp)/ry, (dx-cxp)/rx)
	a1 := math.Atan2((-dy-cyp)/ry, (-dx-cxp)/rx)
	delta := a1 - a0

	switch {
	case sweep && delta < 0:
		delta += 2 * math.Pi //nolint:mnd // a full turn
	case !sweep && delta > 0:
		delta -= 2 * math.Pi //nolint:mnd // a full turn
	}

	// Fine enough for a smooth curve.
	parts := max(curveParts, int(math.Ceil(math.Abs(delta)/(math.Pi/8))))
	pts := make([][2]float64, 0, parts)

	for i := 1; i <= parts; i++ {
		a := a0 + delta*float64(i)/float64(parts)
		pts = append(pts, [2]float64{cx + rx*math.Cos(a), cy + ry*math.Sin(a)})
	}

	return pts
}

// NodeIcon draws a node icon of the containerlab app, centered on a position in cells:
// the symbol in a color, in a rounded hexagon drawn in an ink that reads on the
// background. Its size is in points. A name that is not known draws the router.
func (s *Scene) NodeIcon(name Symbol, cx, cy, size float64, color, ink Color) {
	frame, arts := symbolArts()

	art, ok := arts[name]
	if !ok {
		art = arts[SymbolRouter]
	}

	k := s.Pt(size) / symbolView
	ox, oy := s.X(cx)-symbolGrid/2*k, s.Y(cy)-symbolGrid/2*k // the middle of the grid

	segs := func(lines [][][2]float64) []float32 {
		var out []float32

		for _, line := range lines {
			for i := 1; i < len(line); i++ {
				out = append(
					out,
					f32(
						ox+line[i-1][0]*k,
					),
					f32(oy+line[i-1][1]*k),
					f32(ox+line[i][0]*k),
					f32(oy+line[i][1]*k),
				)
			}
		}

		return out
	}

	s.segsPx(segs(frame), frameStroke*k, ink)
	s.segsPx(segs(art.lines), symbolStroke*k, color)

	for _, d := range art.dots {
		s.circlePx(ox+d[0]*k, oy+d[1]*k, d[2]*k, color, Color{}, 0)
	}

	for _, r := range art.fills {
		s.boxPx(
			ox+r[0]*k,
			oy+r[1]*k,
			ox+(r[0]+r[2])*k,
			oy+(r[1]+r[3])*k,
			r[4]*k,
			color,
			Color{},
			0,
			false,
		)
	}
}
