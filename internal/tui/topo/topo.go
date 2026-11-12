// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package topo lays a lab topology out on a character grid and paints it with
// orthogonal link routing. It has no terminal dependencies: the result is a grid of
// runes tagged with semantic classes, which the caller maps to colors.
package topo

// NodeState is the readiness of a node as shown by its box.
type NodeState int

const (
	NodeUnknown NodeState = iota
	NodeReady
	NodeWarn
	NodeDown
)

// LinkState is the operational state of a link.
type LinkState int

const (
	LinkUnknown LinkState = iota
	LinkUp
	LinkDown
	LinkMixed
)

// Node is one box of the drawing.
type Node struct {
	ID     string  // unique node name
	Label  string  // text shown in the box; defaults to ID when empty
	Kind   string  //
	Role   string  // optional: spine, leaf, ...
	Group  string  //
	Level  int     // tier, 1 = top; 0 = unknown
	X, Y   float64 // saved drawing position in canvas pixels
	HasPos bool
}

// Link connects two nodes. Links whose node is unknown are ignored.
type Link struct {
	A, B           string
	AIface, BIface string
}

// Graph is the input of a layout.
type Graph struct {
	Nodes []Node
	Links []Link
}

// Mode selects how nodes are assigned to rows.
type Mode int

const (
	// ModeAuto uses saved positions when at least 80 % of the nodes have one, then
	// tiers when levels or roles describe the lab, then the graph structure.
	ModeAuto Mode = iota
	ModeSaved
	ModeTiers
	ModeGraph
)

// Zoom is a level of detail, not a scale factor.
type Zoom int

const (
	ZoomCompact Zoom = iota // one-row labels
	ZoomNormal              // three-row boxes
	ZoomDetail              // four-row boxes with a note line
)

// Options tune a layout.
type Options struct {
	Mode  Mode
	Zoom  Zoom
	ASCII bool // draw with - | + instead of box-drawing characters
	// Room asks for a number of columns on the link between two nodes, given by
	// their IDs in either order. It counts where the link runs straight between two
	// neighboring boxes of a row: such a link is otherwise as short as the gap
	// between the boxes, which leaves no place for the texts Paint is given.
	Room map[[2]string]int
	// RowGap is the least number of lines between two rows of boxes that links
	// run between, zero for one. Two lines let a link that runs straight down
	// carry a text at each of its ends.
	RowGap int
	// Margin is the number of blank columns kept at the left and at the right of
	// the drawing, for the texts of links that run along its edge.
	Margin int
}

// Edge is one drawn connection: all links between a pair of nodes.
type Edge struct {
	ID    int
	A, B  string
	Links []int // indexes into Graph.Links
}

// Rect is a cell rectangle.
type Rect struct{ X, Y, W, H int }

// Dir is a keyboard navigation direction.
type Dir int

const (
	DirLeft Dir = iota
	DirRight
	DirUp
	DirDown
)

// Hit tells what is drawn at a cell.
type Hit struct {
	Node string // "" if none
	Edge int    // -1 if none
}

// State is the live data painted over a layout.
type State struct {
	Nodes      map[string]NodeState
	Links      map[int]LinkState // keyed by index into Graph.Links
	EdgeLabels map[int]string    // optional text per Edge ID, e.g. a traffic rate
	// EndLabels holds optional texts per Edge ID for the end at Edge.A and the one at
	// Edge.B, e.g. the names of the interfaces. Each is written next to its own node.
	EndLabels map[int][2]string
	NodeNotes map[string]string // optional second line inside the box at ZoomDetail
}

// Selection is the keyboard focus.
type Selection struct {
	Node string // focused node, "" if none
	Edge int    // focused edge ID, -1 if none
}

// Layout is a computed drawing. It is immutable and safe for concurrent readers.
type Layout struct {
	opts    Options
	spec    zoomSpec
	mode    Mode
	nodes   []*box // sorted by ID
	byID    map[string]int
	rank    []int // natural name order of each node
	edges   []*wire
	tier    []int // tier per node when levels or roles apply, else nil
	w, h    int
	owner   []int32   // per cell: 0 blank, n+1 for node n, -(e+1) for edge e
	nav     [][][]int // per unit and row, the node indexes from left to right
	netIDs  map[*net]int
	stemIDs map[*side]int
	units   []*unit
}

// Compute lays the graph out. The result depends only on the content of the graph,
// not on the order of its nodes and links.
func Compute(g Graph, o Options) *Layout {
	l := newLayout(g, o)
	if len(l.nodes) == 0 {
		return l
	}

	l.tier = l.tiers()
	l.mode = l.pickMode()

	var (
		units []*unit
		loose []int
	)

	if l.mode == ModeSaved {
		units = []*unit{l.savedUnit()}
	} else {
		units, loose = l.componentUnits()
	}

	l.units = units
	l.pack(units, loose)
	l.w += max(l.opts.Margin, 0)
	l.finish()

	return l
}

// Size returns the size of the full drawing in cells.
func (l *Layout) Size() (w, h int) { return l.w, l.h }

// Mode returns the mode that was actually used.
func (l *Layout) Mode() Mode { return l.mode }

// Edges returns one edge per connected node pair, ordered by ID.
func (l *Layout) Edges() []Edge {
	out := make([]Edge, len(l.edges))
	for i, e := range l.edges {
		out[i] = e.Edge
		out[i].Links = append([]int(nil), e.Links...)
	}

	return out
}

// NodeRect returns the cells covered by a node's box.
func (l *Layout) NodeRect(id string) (Rect, bool) {
	i, ok := l.byID[id]
	if !ok {
		return Rect{}, false
	}

	return l.nodes[i].rect, true
}

// Drawn reports whether a graph is laid out by the saved positions of its nodes when a
// mode is asked for: when they are asked for and there are any, or when the choice is
// left open and most of the nodes have one.
func Drawn(g Graph, mode Mode) bool {
	placed, total := 0, 0

	for i := range g.Nodes {
		if g.Nodes[i].ID == "" {
			continue
		}

		total++

		if g.Nodes[i].HasPos {
			placed++
		}
	}

	return drawn(mode, placed, total)
}

func drawn(mode Mode, placed, total int) bool {
	switch mode {
	case ModeSaved:
		return placed > 0
	case ModeTiers, ModeGraph:
		return false
	default:
		// A few stray positions say nothing about the picture the user drew.
		return total > 0 && placed*5 >= total*4
	}
}

// pickMode resolves the requested mode against what the graph can support.
func (l *Layout) pickMode() Mode {
	withPos := 0

	for _, b := range l.nodes {
		if b.HasPos {
			withPos++
		}
	}

	switch {
	case drawn(l.opts.Mode, withPos, len(l.nodes)):
		return ModeSaved
	case l.opts.Mode == ModeGraph, l.tier == nil:
		return ModeGraph
	default:
		return ModeTiers
	}
}
