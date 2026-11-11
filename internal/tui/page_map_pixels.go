// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
)

// Sizes of the drawing of the map, in points.
const (
	linkWidth     = 1.6
	bundleWidth   = 2.6
	focusWidth    = 2.8
	glowWidth     = 9
	linkRound     = 7
	linkDash      = 5
	linkDots      = 2
	nodeRound     = 8
	nodeDot       = 3
	shadowedNodes = 60
)

// framesItself tells that the map draws the cards of its body itself: one for the
// drawing and one for the details next to it.
func (*mapPage) framesItself() bool { return true }

// lineArt reports whether a cell of the painted map is part of a line or of the frame of
// a box, which the pixel layer draws instead.
func lineArt(class clabinternaltuitopo.Class) bool {
	switch class {
	case clabinternaltuitopo.ClassNodeBorder, clabinternaltuitopo.ClassLinkUp,
		clabinternaltuitopo.ClassLinkDown, clabinternaltuitopo.ClassLinkMixed,
		clabinternaltuitopo.ClassLinkUnknown:
		return true
	default:
		return false
	}
}

// viewPixels draws the map with the pixel layer. The layout is the one of the plain map:
// the texts stay where they are, on the grid, and the boxes and the lines between them
// are drawn as cards and as routes with rounded corners.
func (p *mapPage) viewPixels(m *Model) string {
	sc := m.scene()
	if sc == nil {
		sc = clabinternaltuigfx.NewScene(m.metrics())
	}

	sk := m.skin()
	width, height := p.viewSize()
	top, left := float64(pixelBodyTop), float64(pixelInset)
	canvas := sk.Bar

	//nolint:mnd // the cards reach half a line and a cell beyond the text
	if p.panelVisible() {
		sc.Box(1, top-0.5, float64(width)+1.4, float64(height)+1, cardRound, canvas, sk.CardEdge, 1)
		sc.Box(left+float64(width)+0.8, top-0.5, float64(m.width-1)-left-float64(width)-0.8,
			float64(height)+1, cardRound, sk.Card, sk.CardEdge, 1)
	} else {
		sc.Box(1, top-0.5, float64(m.width)-2, float64(height)+1, cardRound, canvas, sk.CardEdge, 1)
	}

	if p.layout == nil || len(p.graph.Nodes) == 0 {
		return p.empty(m)
	}

	grid := p.painted(m)

	//nolint:mnd // just inside the edge of the card
	sc.Clip(1.2, top-0.4, float64(width)+1, float64(height)+0.8)
	p.paintLinks(m, sc, canvas)
	p.paintNodes(m, sc)
	sc.Unclip()

	lines := make([]string, height)
	for y := range height {
		lines[y] = p.renderRow(m, grid, y+p.offsetY, width)
		p.paintLabels(sc, grid, y, width, canvas)
	}

	body := strings.Join(lines, "\n")
	if !p.panelVisible() {
		return body
	}

	gap := strings.TrimRight(strings.Repeat(" \n", height), "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, body, gap, p.panel(m, mapPanelWidth, height))
}

// paintLabels clears the room behind the texts written on the links of a row, so that
// the lines do not run through them.
func (p *mapPage) paintLabels(
	sc *clabinternaltuigfx.Scene,
	grid *clabinternaltuitopo.Grid,
	y, width int,
	canvas clabinternaltuigfx.Color,
) {
	start := -1

	for x := 0; x <= width; x++ {
		label := false

		if x < width {
			class := grid.Cell(x+p.offsetX, y+p.offsetY).Class
			label = class == clabinternaltuitopo.ClassEdgeLabel ||
				class == clabinternaltuitopo.ClassEndLabel
		}

		switch {
		case label && start < 0:
			start = x
		case !label && start >= 0:
			//nolint:mnd // a little wider than the text
			sc.Fill(float64(pixelInset+start)-0.4, float64(pixelBodyTop+y)+0.1,
				float64(x-start)+0.8, 0.8, 6, canvas)

			start = -1
		}
	}
}

// paintLinks draws the links, the ones that matter most last so that they end up on top:
// unknown, up, mixed, down, then the one in focus.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) paintLinks(
	m *Model,
	sc *clabinternaltuigfx.Scene,
	canvas clabinternaltuigfx.Color,
) {
	sk := m.skin()
	edges := p.layout.Edges()

	urgency := func(e *clabinternaltuitopo.Edge) int {
		if e.ID == p.edge {
			return int(clabinternaltuitopo.LinkMixed) + 2 // after every state
		}

		switch p.layout.EdgeState(p.paintedState, e.ID) {
		case clabinternaltuitopo.LinkUp:
			return 1
		case clabinternaltuitopo.LinkMixed:
			return 2 // see above
		case clabinternaltuitopo.LinkDown:
			return 3 // see above
		default:
			return 0
		}
	}

	sort.SliceStable(edges, func(i, j int) bool {
		return urgency(&edges[i]) < urgency(&edges[j])
	})

	// The lines are opaque: several links share stretches of their routes, and there
	// a translucent line would add up.
	info := clabinternaltuigfx.RGB(m.th.Palette.Info)
	up := clabinternaltuigfx.Mix(canvas, clabinternaltuigfx.Mix(sk.Ink, info, 0.5), 0.55)
	unknown := clabinternaltuigfx.Mix(canvas, sk.Ink, 0.3)

	ox, oy := float64(pixelInset-p.offsetX)+0.5, float64(pixelBodyTop-p.offsetY)+0.5

	for i := range edges {
		e := &edges[i]

		route := p.layout.Route(e.ID)
		if len(route) == 0 {
			continue
		}

		pts := make([]clabinternaltuigfx.Point, len(route))
		for k, at := range route {
			pts[k] = clabinternaltuigfx.Point{X: float64(at.X) + ox, Y: float64(at.Y) + oy}
		}

		width, dash, color := linkWidth, 0.0, up

		switch p.layout.EdgeState(p.paintedState, e.ID) {
		case clabinternaltuitopo.LinkDown:
			dash, color = linkDash, clabinternaltuigfx.RGB(m.th.Palette.Error)
		case clabinternaltuitopo.LinkMixed:
			color = clabinternaltuigfx.RGB(m.th.Palette.Warning)
		case clabinternaltuitopo.LinkUnknown:
			dash, color = linkDots, unknown
		case clabinternaltuitopo.LinkUp:
		}

		if p.layout.Bundled(e.ID) {
			width = bundleWidth
		}

		if e.ID == p.edge {
			width, color = focusWidth, sk.Accent
			// A glow.
			sc.Route(pts, glowWidth, linkRound, 0,
				clabinternaltuigfx.Mix(canvas, sk.Accent, 0.25))
		}

		sc.Route(pts, width, linkRound, dash, color)
	}
}

// paintNodes draws the boxes of the nodes as cards.
//
//nolint:mnd // the cards sit a little inside the cells of their boxes
func (p *mapPage) paintNodes(m *Model, sc *clabinternaltuigfx.Scene) {
	sk := m.skin()
	fill := clabinternaltuigfx.Mix(sk.Card, sk.Ink, 0.04)
	edge := clabinternaltuigfx.Mix(sk.CardEdge, sk.Ink, 0.12)
	shadows := len(p.graph.Nodes) <= shadowedNodes

	for i := range p.graph.Nodes {
		id := p.graph.Nodes[i].ID

		r, ok := p.layout.NodeRect(id)
		if !ok {
			continue
		}

		x, y := float64(r.X+pixelInset-p.offsetX), float64(r.Y+pixelBodyTop-p.offsetY)
		w, h := float64(r.W), float64(r.H)
		tint := clabinternaltuigfx.RGB(
			m.th.LevelStyle(nodeStateLevel(p.paintedState.Nodes[id])).GetForeground())
		focused := p.node == id && p.edge < 0

		body, line := fill, edge

		switch p.paintedState.Nodes[id] {
		case clabinternaltuitopo.NodeWarn, clabinternaltuitopo.NodeDown:
			line = clabinternaltuigfx.Mix(sk.Card, tint, 0.75)
		case clabinternaltuitopo.NodeReady, clabinternaltuitopo.NodeUnknown:
		}

		if focused {
			body, line = clabinternaltuigfx.Mix(sk.Card, sk.Accent, 0.14), sk.Accent
		}

		// A node of one row is a pill, a taller one a card with a state dot.
		bx, by, bw, bh, round := x+0.3, y+0.2, w-0.6, h-0.4, float64(nodeRound)
		if r.H == 1 {
			bx, by, bw, bh, round = x+0.1, y+0.06, w-0.2, 0.88, chipRound
		}

		if shadows && r.H > 1 {
			sc.Shadow(bx, by, bw, bh, round, 7, 3, sk.Shadow)
		}

		if focused {
			sc.Halo(bx, by, bw, bh, round, 3, sk.Accent.Alpha(0.25))
		}

		sc.Box(bx, by, bw, bh, round, body, line, 1)

		if r.H > 1 {
			sc.Dot(x+w-1.8, y+0.85, nodeDot, tint)
		}
	}
}
