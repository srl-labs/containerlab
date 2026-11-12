// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "strings"

// Class is the semantic class of a cell. The caller maps classes to colors.
type Class uint8

const (
	ClassBlank Class = iota
	ClassNodeBorder
	ClassNodeLabel
	ClassNodeNote
	ClassLinkUp
	ClassLinkDown
	ClassLinkMixed
	ClassLinkUnknown
	ClassEdgeLabel
	ClassBadge
	ClassEndLabel
)

// Cell is one character of a painted drawing. Every cell is one column wide.
type Cell struct {
	Rune     rune
	Class    Class
	State    NodeState // for node cells: the node's state
	Selected bool      // belongs to the focused node or edge
}

// Grid is a painted drawing.
type Grid struct {
	w, h  int
	cells []Cell
}

func newGrid(w, h int) *Grid {
	g := &Grid{w: w, h: h, cells: make([]Cell, w*h)}
	for i := range g.cells {
		g.cells[i].Rune = ' '
	}

	return g
}

// Size returns the size of the grid in cells.
func (g *Grid) Size() (w, h int) { return g.w, g.h }

// Cell returns the cell at a position. Positions outside the grid are blank.
func (g *Grid) Cell(x, y int) Cell {
	if !g.has(x, y) {
		return Cell{Rune: ' '}
	}

	return g.cells[y*g.w+x]
}

func (g *Grid) has(x, y int) bool { return x >= 0 && y >= 0 && x < g.w && y < g.h }

// at returns the cell at a position known to be inside the grid.
func (g *Grid) at(x, y int) *Cell { return &g.cells[y*g.w+x] }

// blank reports whether nothing has been drawn at a position inside the grid.
func (g *Grid) blank(x, y int) bool {
	return g.has(x, y) && g.cells[y*g.w+x].Class == ClassBlank
}

// Lines returns the grid as plain text, one string per row, without trailing
// blanks.
func (g *Grid) Lines() []string {
	lines := make([]string, g.h)

	var b strings.Builder

	for y := range lines {
		b.Reset()

		for x := range g.w {
			b.WriteRune(g.cells[y*g.w+x].Rune)
		}

		lines[y] = strings.TrimRight(b.String(), " ")
	}

	return lines
}
