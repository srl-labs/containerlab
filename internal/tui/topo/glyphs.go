// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

// weight is the line style of a stroke. Higher values win where strokes of one
// owner meet.
type weight uint8

const (
	weightLight  weight = iota
	weightDashed        // a link that is down
	weightDotted        // a link of unknown state
	weightDouble        // several links drawn as one
	weightHeavy         // the focused node or edge
	weightCount
)

// lineRunes maps the direction bits of a cell (north 1, east 2, south 4, west 8) to
// a glyph, per weight. Box drawing has no dashed corners and no double half lines;
// the nearest glyph stands in.
var lineRunes = [weightCount][]rune{
	weightLight:  []rune(" ╵╶└╷│┌├╴┘─┴┐┤┬┼"),
	weightDashed: []rune(" ╎╌└╎╎┌├╌┘╌┴┐┤┬┼"),
	weightDotted: []rune(" ┊┈└┊┊┌├┈┘┈┴┐┤┬┼"),
	weightDouble: []rune(" ║═╚║║╔╠═╝═╩╗╣╦╬"),
	weightHeavy:  []rune(" ╹╺┗╻┃┏┣╸┛━┻┓┫┳╋"),
}

// asciiRunes holds the horizontal, vertical and junction characters per weight.
var asciiRunes = [weightCount][3]rune{
	weightLight:  {'-', '|', '+'},
	weightDashed: {'.', ':', '+'},
	weightDotted: {'~', '!', '+'},
	weightDouble: {'=', '"', '+'},
	weightHeavy:  {'#', '#', '#'},
}

// portRunes holds the glyph of a port by stem direction, border weight (light or
// heavy) and stem weight (light, heavy or double). Box drawing has no heavy tee
// with a double stem, so that combination falls back to all heavy.
var portRunes = map[uint8][2][3]rune{
	dirN: {{'┴', '┸', '╨'}, {'┷', '┻', '┻'}},
	dirS: {{'┬', '┰', '╥'}, {'┯', '┳', '┳'}},
	dirE: {{'├', '┝', '╞'}, {'┠', '┣', '┣'}},
	dirW: {{'┤', '┥', '╡'}, {'┨', '┫', '┫'}},
}

// lineRune returns the glyph for a line cell.
func lineRune(w weight, bits uint8, ascii bool) rune {
	if !ascii {
		return lineRunes[w][bits]
	}

	switch {
	case bits == 0:
		return ' '
	case bits&horizontal == 0:
		return asciiRunes[w][1]
	case bits&vertical == 0:
		return asciiRunes[w][0]
	default:
		return asciiRunes[w][2]
	}
}

// portRune returns the glyph for the border cell where an edge attaches.
func portRune(stem uint8, border, line weight, ascii bool) rune {
	if ascii {
		return asciiRunes[weightLight][2]
	}

	b, s := 0, 0
	if border == weightHeavy {
		b = 1
	}

	switch line {
	case weightHeavy:
		s = 1
	case weightDouble:
		s = 2
	}

	return portRunes[stem][b][s]
}
