// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"math"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// Sizes of the drawing of the map, in points.
const (
	linkWidth     = 2.5
	bundleWidth   = 3.4
	focusWidth    = 4
	linkCasing    = 1.5
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

// nodeText reports whether a cell of the painted map is text written in the box of a
// node.
func nodeText(class clabinternaltuitopo.Class) bool {
	return class == clabinternaltuitopo.ClassNodeLabel ||
		class == clabinternaltuitopo.ClassNodeNote
}

// viewPixels draws the map with the pixel layer. The layout is the one of the plain map:
// the texts stay where they are, on the grid, and the boxes and the lines between them
// are drawn as cards and as routes with rounded corners.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) viewPixels(m *Model) string {
	sc := m.scene()
	if sc == nil {
		sc = m.newScene()
	}

	sk := m.skin()
	width, height := p.viewSize()
	top, left := float64(pixelBodyTop), float64(pixelInset)
	canvas := sk.Bar
	p.ownText = m.textFont() != nil

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

	p.aspect = sc.Y(1) / sc.X(1)

	if p.free != nil {
		return p.viewFree(m, sc, canvas)
	}

	grid := p.painted(m)

	//nolint:mnd // just inside the edge of the card
	sc.Clip(1.2, top-0.4, float64(width)+1, float64(height)+0.8)
	sc.Dots(1, top-0.5, float64(width)+1.4, float64(height)+1, 22, 0.9, sk.Ink.Alpha(0.1))
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

// edgesByUrgency returns the links in the order they are drawn in, the ones that matter
// most last so that they end up on top: unknown, up, mixed, down, then the one in focus.
func (p *mapPage) edgesByUrgency() []clabinternaltuitopo.Edge {
	edges := p.layout.Edges()

	urgency := func(e *clabinternaltuitopo.Edge) int {
		if e.ID == p.edge {
			return int(clabinternaltuitopo.LinkMixed) + 2 //nolint:mnd // after every state
		}

		switch p.layout.EdgeState(p.paintedState, e.ID) {
		case clabinternaltuitopo.LinkUp:
			return 1
		case clabinternaltuitopo.LinkMixed:
			return 2 //nolint:mnd // see above
		case clabinternaltuitopo.LinkDown:
			return 3 //nolint:mnd // see above
		default:
			return 0
		}
	}

	sort.SliceStable(edges, func(i, j int) bool {
		return urgency(&edges[i]) < urgency(&edges[j])
	})

	return edges
}

// paintLink draws the line of a link through points given in cells.
//
// The lines are opaque: several links share stretches of their routes, and there a
// translucent line would add up. The colors are the ones of the containerlab app, mixed
// with the canvas as it does for links that run in right angles.
func (p *mapPage) paintLink(
	m *Model,
	sc *clabinternaltuigfx.Scene,
	e *clabinternaltuitopo.Edge,
	pts []clabinternaltuigfx.Point,
	canvas clabinternaltuigfx.Color,
) {
	sk := m.skin()
	width, dash := linkWidth, 0.0
	color := clabinternaltuigfx.Mix(canvas, appLinkUp, appLinkMix)

	switch p.layout.EdgeState(p.paintedState, e.ID) {
	case clabinternaltuitopo.LinkDown:
		dash, color = linkDash, clabinternaltuigfx.Mix(canvas, appLinkDown, appLinkMix)
	case clabinternaltuitopo.LinkMixed:
		color = clabinternaltuigfx.RGB(m.th.Palette.Warning)
	case clabinternaltuitopo.LinkUnknown:
		color = clabinternaltuigfx.Mix(canvas, appLinkIdle, appLinkMix)
	case clabinternaltuitopo.LinkUp:
	}

	if p.layout.Bundled(e.ID) {
		width = bundleWidth
	}

	if e.ID == p.edge {
		width, color = focusWidth, sk.Accent
		// A glow.
		sc.Route(pts, glowWidth, linkRound, 0,
			clabinternaltuigfx.Mix(canvas, sk.Accent, 0.25)) //nolint:mnd // a quarter
	}

	// A casing in the color of the canvas: where two links cross, the one on top
	// cuts a small gap in the other, so that a crossing never reads as a turn.
	sc.Route(pts, width+2*linkCasing, linkRound, 0, canvas)
	sc.Route(pts, width, linkRound, dash, color)
}

// paintLinks draws the links along the routes of the layout.
func (p *mapPage) paintLinks(
	m *Model,
	sc *clabinternaltuigfx.Scene,
	canvas clabinternaltuigfx.Color,
) {
	edges := p.edgesByUrgency()

	//nolint:mnd // the middle of a cell
	ox, oy := float64(pixelInset-p.offsetX)+0.5, float64(pixelBodyTop-p.offsetY)+0.5

	for i := range edges {
		route := p.layout.Route(edges[i].ID)
		if len(route) == 0 {
			continue
		}

		pts := make([]clabinternaltuigfx.Point, len(route))
		for k, at := range route {
			pts[k] = clabinternaltuigfx.Point{X: float64(at.X) + ox, Y: float64(at.Y) + oy}
		}

		p.paintLink(m, sc, &edges[i], pts, canvas)
	}
}

// Colors of the topology of the containerlab app (clab-ui: TopologyEdge.tsx,
// SvgGenerator.ts, NodeRuntimeBadge.tsx and graph.ts).
//
//nolint:mnd // the components of the colors
var (
	appLinkUp   = clabinternaltuigfx.Color{R: 0x00, G: 0xdf, B: 0x2b, A: 0xff}
	appLinkDown = clabinternaltuigfx.Color{R: 0xdf, G: 0x2b, B: 0x00, A: 0xff}
	appLinkIdle = clabinternaltuigfx.Color{R: 0x96, G: 0x97, B: 0x99, A: 0xff}
	// appIconColor is the color of a node icon, and appIconInk the one of its frame
	// on a dark and on a light background.
	appIconColor    = clabinternaltuigfx.Color{R: 0x00, G: 0xc9, B: 0xff, A: 0xff}
	appIconInkDark  = clabinternaltuigfx.Color{R: 0xdd, G: 0xf8, B: 0xff, A: 0xff}
	appIconInkLight = clabinternaltuigfx.Color{R: 0x00, G: 0x11, B: 0x35, A: 0xff}
	// The badges of nodes that are not running: fill and border.
	appBadgeBooting = [2]clabinternaltuigfx.Color{
		{R: 0x0e, G: 0xa5, B: 0xe9, A: 0xff},
		{R: 0x0c, G: 0x4a, B: 0x6e, A: 0xff},
	}
	appBadgeStopped = [2]clabinternaltuigfx.Color{
		{R: 0xef, G: 0x44, B: 0x44, A: 0xff},
		{R: 0x7f, G: 0x1d, B: 0x1d, A: 0xff},
	}
	appBadgeUnknown = [2]clabinternaltuigfx.Color{
		{R: 0x64, G: 0x74, B: 0x8b, A: 0xff},
		{R: 0x33, G: 0x41, B: 0x55, A: 0xff},
	}
)

// appLinkMix is how much of its color a link keeps over the canvas.
const appLinkMix = 0.7

// iconInk returns the color of the frame of a node icon on the background of the theme.
func (m *Model) iconInk() clabinternaltuigfx.Color {
	if m.skin().Card.Luma() > 0.5 { //nolint:mnd // a light background
		return appIconInkLight
	}

	return appIconInkDark
}

// nodeSymbol picks the icon of a node the way the containerlab app does: by the role it
// was given, ie. with the graph-icon label, and else by what its kind tells.
func nodeSymbol(role, kind string) clabinternaltuigfx.Symbol {
	role = strings.ToLower(strings.TrimSpace(role))

	switch role {
	case "pe", "p":
		return clabinternaltuigfx.SymbolRouter
	case "bridge":
		return clabinternaltuigfx.SymbolSwitch
	case "superspine":
		return clabinternaltuigfx.SymbolSuperSpine
	case "host":
		return clabinternaltuigfx.SymbolClient
	}

	if clabinternaltuigfx.HasSymbol(role) {
		return clabinternaltuigfx.Symbol(role)
	}

	kind = strings.ToLower(kind)

	switch {
	case kind == "linux":
		return clabinternaltuigfx.SymbolClient
	case kind == "host" || kind == "ext-container":
		return clabinternaltuigfx.SymbolServer
	case strings.Contains(kind, "bridge") || strings.Contains(kind, "ovs"):
		return clabinternaltuigfx.SymbolSwitch
	case strings.Contains(kind, "forti") || strings.Contains(kind, "panos") ||
		strings.Contains(kind, "firewall"):
		return clabinternaltuigfx.SymbolFirewall
	case strings.Contains(kind, "k8s"):
		return clabinternaltuigfx.SymbolContainer
	default:
		return clabinternaltuigfx.SymbolRouter
	}
}

// paintNodes draws the boxes of the nodes as cards.
func (p *mapPage) paintNodes(m *Model, sc *clabinternaltuigfx.Scene) {
	shadows := len(p.graph.Nodes) <= shadowedNodes

	for i := range p.graph.Nodes {
		r, ok := p.layout.NodeRect(p.graph.Nodes[i].ID)
		if !ok {
			continue
		}

		p.paintCard(m, sc, &p.graph.Nodes[i],
			float64(r.X+pixelInset-p.offsetX), float64(r.Y+pixelBodyTop-p.offsetY),
			float64(r.W), float64(r.H), shadows, false)
	}
}

// paintCard draws a node as a card whose top left corner is at a position in cells. A
// card that is lifted, as one is while it is moved, throws a longer shadow.
//
//nolint:mnd // the cards sit a little inside the cells of their boxes
func (p *mapPage) paintCard(
	m *Model, sc *clabinternaltuigfx.Scene, n *clabinternaltuitopo.Node,
	x, y, w, h float64, shadows, lifted bool,
) {
	sk := m.skin()
	fill := clabinternaltuigfx.Mix(sk.Card, sk.Ink, 0.04)
	edge := clabinternaltuigfx.Mix(sk.CardEdge, sk.Ink, 0.12)
	rows := int(math.Round(h))

	tint := clabinternaltuigfx.RGB(
		m.th.LevelStyle(nodeStateLevel(p.paintedState.Nodes[n.ID])).GetForeground())
	focused := p.node == n.ID && p.edge < 0

	body, line := fill, edge

	switch p.paintedState.Nodes[n.ID] {
	case clabinternaltuitopo.NodeWarn, clabinternaltuitopo.NodeDown:
		line = clabinternaltuigfx.Mix(sk.Card, tint, 0.75)
	case clabinternaltuitopo.NodeReady, clabinternaltuitopo.NodeUnknown:
	}

	if focused {
		body, line = clabinternaltuigfx.Mix(sk.Card, sk.Accent, 0.14), sk.Accent
	}

	// A node of one row is a pill, a taller one a card with a state dot.
	bx, by, bw, bh, round := x+0.3, y+0.2, w-0.6, h-0.4, float64(nodeRound)
	if rows == 1 {
		bx, by, bw, bh, round = x+0.1, y+0.06, w-0.2, 0.88, chipRound
	}

	switch {
	case lifted:
		sc.Shadow(bx, by, bw, bh, round, 18, 9, sk.Shadow)
	case shadows && rows > 1:
		sc.Shadow(bx, by, bw, bh, round, 7, 3, sk.Shadow)
	}

	if focused {
		sc.Halo(bx, by, bw, bh, round, 3, sk.Accent.Alpha(0.25))
	}

	sc.Box(bx, by, bw, bh, round, body, line, 1)

	if !p.ownText {
		if rows > 1 {
			sc.Dot(x+w-1.8, y+0.85, nodeDot, tint)
		}

		return
	}

	p.paintNodeContent(m, sc, n, rows, bx, bw, y, focused)
}

// nodeBadge returns the colors of the badge of a node that is not running.
func nodeBadge(state clabinternaltuitopo.NodeState) ([2]clabinternaltuigfx.Color, bool) {
	switch state {
	case clabinternaltuitopo.NodeWarn:
		return appBadgeBooting, true
	case clabinternaltuitopo.NodeDown:
		return appBadgeStopped, true
	case clabinternaltuitopo.NodeUnknown:
		return appBadgeUnknown, true
	case clabinternaltuitopo.NodeReady:
	}

	return [2]clabinternaltuigfx.Color{}, false
}

// paintNodeContent draws what is in the card of a node when the layer writes the text: an
// icon tile as the containerlab app shows nodes, with the state of the node as a badge,
// and next to it the name and what is noted under it. A card too narrow for the icon
// gets the text alone, centered.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) paintNodeContent(
	m *Model, sc *clabinternaltuigfx.Scene, n *clabinternaltuitopo.Node, rows int,
	bx, bw, y float64, focused bool,
) {
	font, sk := m.textFont(), m.skin()

	name := n.Label
	if name == "" {
		name = n.ID
	}

	name = clabinternaltuiui.Sanitize(name)

	note := ""
	if rows > 3 {
		note = p.paintedState.NodeNotes[n.ID]
		if note == "" {
			note = n.Kind
		}
	}

	ink := clabinternaltuigfx.Mix(sk.Ink, sk.Card, 0.08)
	if focused {
		ink = sk.Accent
	}

	muted := clabinternaltuigfx.RGB(m.th.Palette.Muted)

	if rows == 1 {
		sc.Text(font, bx, y, bw, textSize, focused, clabinternaltuigfx.AlignCenter, name, ink)

		return
	}

	// The lines of text sit where the plain map has them: the name on the second
	// line of the box, the note under it.
	nameRow, noteRow := y+1, y+2
	textWidth := math.Max(
		sc.TextWidth(font, textSize, true, name),
		sc.TextWidth(font, textSize-1, false, note),
	)

	// The icon is a square as tall as the text block and a little more.
	side := 1.5
	if note != "" {
		side = 2.0
	}

	tile := sc.Y(side) / sc.X(1)
	gap := 0.8

	if tile+gap+textWidth > bw-0.5 {
		sc.Text(font, bx, nameRow, bw, textSize, true, clabinternaltuigfx.AlignCenter, name, ink)
		sc.Text(
			font,
			bx,
			noteRow,
			bw,
			textSize-1,
			false,
			clabinternaltuigfx.AlignCenter,
			note,
			muted,
		)

		if badge, ok := nodeBadge(p.paintedState.Nodes[n.ID]); ok {
			sc.Dot(bx+bw-1, y+0.85, nodeDot+1.4, badge[1])
			sc.Dot(bx+bw-1, y+0.85, nodeDot, badge[0])
		}

		return
	}

	left := bx + (bw-tile-gap-textWidth)/2

	mid := nameRow + 0.5
	if note != "" {
		mid = nameRow + 1
	}

	iconSize := sc.Y(side) / sc.Pt(1)
	sc.NodeIcon(nodeSymbol(n.Role, n.Kind), left+tile/2, mid, iconSize, appIconColor, m.iconInk())

	// A node that runs has no badge, as in the app; one that does not gets one on the
	// corner of its icon.
	if badge, ok := nodeBadge(p.paintedState.Nodes[n.ID]); ok {
		sc.Dot(left+tile*0.86, mid+side*0.36, 5.2, badge[1])
		sc.Dot(left+tile*0.86, mid+side*0.36, 3.8, badge[0])
	}

	tx := left + tile + gap
	sc.Text(font, tx, nameRow, 0, textSize, true, clabinternaltuigfx.AlignStart, name, ink)
	sc.Text(font, tx, noteRow, 0, textSize-1, false, clabinternaltuigfx.AlignStart, note, muted)
}
