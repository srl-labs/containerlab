// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// What follows lays the free view of the map out and draws it. How it is moved around
// in, and how its nodes are moved, is in page_map_free.go.

// nodeName returns what is written on the card of a node.
func nodeName(n *clabinternaltuitopo.Node) string {
	name := n.Label
	if name == "" {
		name = n.ID
	}

	return clabinternaltuiui.Sanitize(name)
}

// cardSize returns the size of the card of a node at a level of detail, in cells: a pill
// with its name, or an icon with the name next to it and, with most detail, a line under
// the name.
//
//nolint:mnd // sizes of what is drawn, in cells and points
func (*mapPage) cardSize(
	m *Model, sc *clabinternaltuigfx.Scene, n *clabinternaltuitopo.Node,
	level clabinternaltuitopo.Zoom,
) (w, h float64) {
	font := m.textFont()
	name := sc.TextWidth(font, textSize, true, nodeName(n))

	switch level {
	case clabinternaltuitopo.ZoomCompact:
		return math.Max(freePillMin, name+freePillPad), 1
	case clabinternaltuitopo.ZoomDetail:
		note := math.Max(
			sc.TextWidth(font, textSize-1, false, clabinternaltuiui.Sanitize(n.Kind)),
			sc.TextWidth(font, textSize-1, false, freeNoteRoom),
		)

		return sc.Y(2)/sc.X(1) + freeTileGap + math.Max(name, note) + freeCardPad, 4
	default:
		return sc.Y(1.5)/sc.X(1) + freeTileGap + name + freeCardPad, 3
	}
}

// cardSizes returns the sizes of the cards of the nodes that are drawn.
func (p *mapPage) cardSizes(
	m *Model, sc *clabinternaltuigfx.Scene, level clabinternaltuitopo.Zoom,
) [][2]float64 {
	sizes := make([][2]float64, len(p.graph.Nodes))
	for i := range p.graph.Nodes {
		sizes[i][0], sizes[i][1] = p.cardSize(m, sc, &p.graph.Nodes[i], level)
	}

	return sizes
}

// needScale returns the least scale at which no two cards overlap.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) needScale(sizes [][2]float64) float64 {
	f := p.free
	need := 0.0

	for i := range p.graph.Nodes {
		a, ok := f.spots[p.graph.Nodes[i].ID]
		if !ok {
			continue
		}

		for j := i + 1; j < len(p.graph.Nodes); j++ {
			b, ok := f.spots[p.graph.Nodes[j].ID]
			if !ok {
				continue
			}

			// Two cards are apart when they are either next to each other or above
			// one another: the cheaper of the two is what the pair asks for.
			pair := math.Inf(1)

			if dx := math.Abs(a.X - b.X); dx > 0 {
				pair = ((sizes[i][0]+sizes[j][0])/2 + freeGapCols) / dx
			}

			if dy := math.Abs(a.Y - b.Y); dy > 0 {
				rows := (sizes[i][1]+sizes[j][1])/2 + freeGapRows
				pair = math.Min(pair, rows*f.aspect/dy)
			}

			if !math.IsInf(pair, 1) {
				need = math.Max(need, pair)
			}
		}
	}

	return math.Min(need, freeTight)
}

// bounds returns the rectangle of the canvas that holds the spots of the drawn nodes.
func (p *mapPage) bounds() (minX, minY, maxX, maxY float64, ok bool) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)

	for i := range p.graph.Nodes {
		s, found := p.free.spots[p.graph.Nodes[i].ID]
		if !found {
			continue
		}

		minX, maxX = math.Min(minX, s.X), math.Max(maxX, s.X)
		minY, maxY = math.Min(minY, s.Y), math.Max(maxY, s.Y)
		ok = true
	}

	return minX, minY, maxX, maxY, ok
}

// fitFree picks the most detailed level at which the whole drawing fits the view without
// its cards overlapping, and the scale that fills the view with it, within reason: two
// nodes are not put at opposite ends of the window. A drawing too large for the view even
// as pills is shown at the scale that keeps them apart, around the node in focus.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) fitFree(m *Model, sc *clabinternaltuigfx.Scene, width, height float64) {
	f := p.free

	minX, minY, maxX, maxY, ok := p.bounds()
	if !ok {
		return
	}

	room := func(cells, span float64) float64 {
		switch {
		case span <= 0:
			return math.Inf(1)
		case cells <= 0:
			return 0
		default:
			return cells / span
		}
	}

	f.recenter = false

	for _, level := range []clabinternaltuitopo.Zoom{
		clabinternaltuitopo.ZoomDetail, clabinternaltuitopo.ZoomNormal, clabinternaltuitopo.ZoomCompact,
	} {
		sizes := p.cardSizes(m, sc, level)

		maxW, maxH := 0.0, 0.0
		for i := range sizes {
			maxW, maxH = math.Max(maxW, sizes[i][0]), math.Max(maxH, sizes[i][1])
		}

		need := p.needScale(sizes)
		fits := math.Min(
			room(width-maxW-2*freeMarginCols, maxX-minX),
			room((height-maxH-2*freeMarginRows)*f.aspect, maxY-minY),
		)

		if need > fits && level != clabinternaltuitopo.ZoomCompact {
			continue
		}

		scale := math.Min(fits, math.Max(need*freeSpread, 1/clabinternaltuitopo.SpotCol))
		if need > fits {
			scale, f.recenter = need, true
		}

		f.level = level
		f.scale = math.Max(freeMinScale, math.Min(scale, freeMaxScale))

		break
	}

	f.ox = (minX+maxX)/2 - width/2/f.scale
	f.oy = (minY+maxY)/2 - height/2*f.aspect/f.scale
}

// levelFor returns the most detailed level whose cards do not overlap at a scale.
func (p *mapPage) levelFor(
	m *Model, sc *clabinternaltuigfx.Scene, scale float64,
) clabinternaltuitopo.Zoom {
	const slack = 1.001

	for _, level := range []clabinternaltuitopo.Zoom{
		clabinternaltuitopo.ZoomDetail, clabinternaltuitopo.ZoomNormal,
	} {
		if p.needScale(p.cardSizes(m, sc, level)) <= scale*slack {
			return level
		}
	}

	return clabinternaltuitopo.ZoomCompact
}

// settle works out what the frame shows: the scale and the level when they are due, where
// the view is, and from that where every card goes.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) settle(m *Model, sc *clabinternaltuigfx.Scene, width, height float64) {
	f := p.free
	f.aspect = p.aspect

	switch {
	case f.refit:
		p.fitFree(m, sc, width, height)
	case f.relevel:
		f.level = p.levelFor(m, sc, f.scale)
	}

	sizes := p.cardSizes(m, sc, f.level)

	if s, ok := f.spots[p.node]; ok {
		x, y := f.toView(s)

		switch {
		case f.recenter:
			x, y = x-width/2, y-height/2
		case f.reveal:
			w, h := 0.0, 0.0

			for i := range p.graph.Nodes {
				if p.graph.Nodes[i].ID == p.node {
					w, h = sizes[i][0]/2, sizes[i][1]/2
				}
			}

			x = math.Min(0, x-w-mapMargin) + math.Max(0, x+w+mapMargin-width)
			y = math.Min(0, y-h-mapMargin/2) + math.Max(0, y+h+mapMargin/2-height)
		default:
			x, y = 0, 0
		}

		f.ox += x / f.scale
		f.oy += y * f.aspect / f.scale
	}

	f.refit, f.relevel, f.recenter, f.reveal = false, false, false, false

	p.keepInView(width, height)

	// Every card starts on a whole pixel: what is drawn on it then keeps its place
	// on the card wherever the card goes.
	snapX := func(x float64) float64 {
		return math.Round(sc.X(x+pixelInset))/sc.X(1) - pixelInset
	}
	snapY := func(y float64) float64 {
		return math.Round(sc.Y(y+pixelBodyTop))/sc.Y(1) - pixelBodyTop
	}

	f.cards = f.cards[:0]
	f.index = make(map[string]int, len(p.graph.Nodes))

	for i := range p.graph.Nodes {
		n := &p.graph.Nodes[i]

		s, ok := f.spots[n.ID]
		if !ok {
			continue
		}

		x, y := f.toView(s)
		w, h := sizes[i][0], sizes[i][1]

		f.index[n.ID] = len(f.cards)
		f.cards = append(f.cards, freeCard{
			n: n, x: snapX(x - w/2), y: snapY(y - h/2), w: w, h: h,
		})
	}
}

// keepInView stops the view from leaving the drawing behind: at least a corner of the
// rectangle the nodes are in stays in it.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) keepInView(width, height float64) {
	f := p.free

	minX, minY, maxX, maxY, ok := p.bounds()
	if !ok {
		return
	}

	// A part of the view at its edges does not count: what is left of the drawing
	// there is too little to find it again.
	w, h := width/f.scale, height*f.aspect/f.scale
	edgeX, edgeY := math.Min(w/4, freeEdge/f.scale), math.Min(h/4, freeEdge/f.scale)

	f.ox = math.Max(minX-w+edgeX, math.Min(f.ox, maxX-edgeX))
	f.oy = math.Max(minY-h+edgeY, math.Min(f.oy, maxY-edgeY))
}

// prepare implements prepared: the free view is laid out for the frame, so that what is
// said about it above the map, ie. how much its cards show, is what the frame shows.
func (p *mapPage) prepare(m *Model, sc *clabinternaltuigfx.Scene) {
	p.ownText, p.aspect = m.textFont() != nil, sc.Y(1)/sc.X(1)

	if p.free == nil || p.layout == nil || len(p.graph.Nodes) == 0 {
		return
	}

	width, height := p.viewSize()

	p.freshState(m)
	p.settle(m, sc, float64(width), float64(height))

	p.free.settled = true
}

// viewFree draws the free view: all of it by the pixel layer, on a body of blank lines.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) viewFree(
	m *Model, sc *clabinternaltuigfx.Scene, canvas clabinternaltuigfx.Color,
) string {
	width, height := p.viewSize()
	top := float64(pixelBodyTop)

	// Laid out already when the frame was prepared, unless the view is drawn alone.
	if !p.free.settled {
		p.prepare(m, sc)
	}

	p.free.settled = false

	sc.Clip(1.2, top-0.4, float64(width)+1, float64(height)+0.8)
	p.paintFreeDots(m, sc, width, height)
	p.paintGuides(m, sc, width, height)
	p.paintFreeLinks(m, sc, canvas)
	p.paintFreeCards(m, sc, width, height)
	sc.Unclip()

	body := strings.TrimRight(strings.Repeat(strings.Repeat(" ", width)+"\n", height), "\n")
	if !p.panelVisible() {
		return body
	}

	gap := strings.TrimRight(strings.Repeat(" \n", height), "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, body, gap, p.panel(m, mapPanelWidth, height))
}

// paintFreeDots marks the canvas with a grid of dots that goes along when the view is
// moved, as the canvas of the containerlab application does.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) paintFreeDots(m *Model, sc *clabinternaltuigfx.Scene, width, height int) {
	const step, radius = 22, 0.9

	f := p.free
	pitch := math.Max(2, math.Round(sc.Pt(step)))

	// How far the canvas is moved, in pixels, within one step of the grid.
	shift := func(cells, cell float64) float64 {
		moved := math.Mod(cells*cell, pitch)
		if moved < 0 {
			moved += pitch
		}

		return moved / cell
	}

	dx := shift(f.ox*f.scale, sc.X(1))
	dy := shift(f.oy*f.scale/f.aspect, sc.Y(1))

	sc.Dots(1-dx, float64(pixelBodyTop)-0.5-dy, float64(width)+1.4+dx, float64(height)+1+dy,
		step, radius, m.skin().Ink.Alpha(0.1))
}

// paintGuides draws the lines a moved node snapped to.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *mapPage) paintGuides(m *Model, sc *clabinternaltuigfx.Scene, width, height int) {
	f := p.free
	color := m.skin().Accent.Alpha(0.55)

	if f.guideX != nil {
		x, _ := f.toView(clabinternaltuitopo.Spot{X: *f.guideX})
		x += pixelInset

		sc.Route([]clabinternaltuigfx.Point{
			{X: x, Y: float64(pixelBodyTop) - 0.5}, {X: x, Y: float64(pixelBodyTop+height) + 0.5},
		}, 1, 0, 3, color)
	}

	if f.guideY != nil {
		_, y := f.toView(clabinternaltuitopo.Spot{Y: *f.guideY})
		y += pixelBodyTop

		sc.Route([]clabinternaltuigfx.Point{
			{X: 1, Y: y}, {X: float64(width) + 2.4, Y: y},
		}, 1, 0, 3, color)
	}
}

// paintFreeCards draws the nodes: the one in focus on top of the others, and the one that
// is being moved on top of everything.
func (p *mapPage) paintFreeCards(m *Model, sc *clabinternaltuigfx.Scene, width, height int) {
	f := p.free
	shadows := len(f.cards) <= shadowedNodes

	lifted := ""

	switch {
	case p.grabbed != nil && p.grabbed.moving:
		lifted = p.grabbed.node
	case p.moving != nil:
		lifted = p.moving.node
	}

	draw := func(c *freeCard) {
		// What is out of view takes no drawing.
		const edge = 2

		if c.x+c.w < -edge || c.y+c.h < -edge ||
			c.x > float64(width)+edge || c.y > float64(height)+edge {
			return
		}

		p.paintCard(m, sc, c.n, c.x+pixelInset, c.y+pixelBodyTop, c.w, c.h,
			shadows, c.n.ID == lifted)
	}

	for i := range f.cards {
		if id := f.cards[i].n.ID; id != p.node && id != lifted {
			draw(&f.cards[i])
		}
	}

	if c := f.card(p.node); c != nil && p.node != lifted {
		draw(c)
	}

	if c := f.card(lifted); c != nil {
		draw(c)
	}
}

// paintFreeLinks draws every link as a straight line between the cards of its nodes, and
// then what is written on the links, on top of the lines.
func (p *mapPage) paintFreeLinks(
	m *Model, sc *clabinternaltuigfx.Scene, canvas clabinternaltuigfx.Color,
) {
	f := p.free
	edges := p.edgesByUrgency()

	for i := range edges {
		a, b := f.card(edges[i].A), f.card(edges[i].B)
		if a == nil || b == nil {
			continue
		}

		ax, ay := a.center()
		bx, by := b.center()

		p.paintLink(m, sc, &edges[i], []clabinternaltuigfx.Point{
			{X: ax + pixelInset, Y: ay + pixelBodyTop}, {
				X: bx + pixelInset, Y: by + pixelBodyTop,
			},
		}, canvas)
	}

	if m.textFont() == nil {
		return
	}

	for i := range edges {
		if a, b := f.card(edges[i].A), f.card(edges[i].B); a != nil && b != nil {
			p.paintFreeLabels(m, sc, &edges[i], a, b, canvas)
		}
	}
}

// freeText is a piece of a text written on a link.
type freeText struct {
	text  string
	color clabinternaltuigfx.Color
	bold  bool
}

// freeLine is the line of a link that texts are written on, in pixels: where it starts
// and which way it runs, and the stretch of it between the cards that is still free.
type freeLine struct {
	sc       *clabinternaltuigfx.Scene
	font     *clabinternaltuigfx.Font
	canvas   clabinternaltuigfx.Color
	x, y     float64
	ux, uy   float64
	from, to float64
	gap      float64
}

// reach is how far along the line a box centered on it extends.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (l *freeLine) reach(w, h float64) float64 {
	along := math.Inf(1)
	if l.ux != 0 {
		along = w / 2 / math.Abs(l.ux)
	}

	if l.uy != 0 {
		along = math.Min(along, h/2/math.Abs(l.uy))
	}

	return along
}

// measure returns the width of a text made of parts, in cells, and how far along the line
// the box around it extends from its middle.
func (l *freeLine) measure(parts []freeText) (width, along float64) {
	if len(parts) == 0 {
		return 0, 0
	}

	for i := range parts {
		width += l.sc.TextWidth(l.font, freeLabelSize, parts[i].bold, parts[i].text)
	}

	width += float64(len(parts)-1) * freeLabelPad

	return width, l.reach(l.sc.X(width+freeLabelPad), l.sc.Y(freeLabelHeight))
}

// write puts a text on the line, its middle at a distance from the start of the line, on
// a patch of the canvas that keeps the line from running through it.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (l *freeLine) write(parts []freeText, width, at float64) {
	sc := l.sc
	x := (l.x+l.ux*at)/sc.X(1) + pixelInset
	y := (l.y+l.uy*at)/sc.Y(1) + pixelBodyTop

	sc.Fill(x-(width+freeLabelPad)/2, y-freeLabelHeight/2, width+freeLabelPad,
		freeLabelHeight, freeLabelRound, l.canvas)

	x -= width / 2

	for i := range parts {
		w := sc.TextWidth(l.font, freeLabelSize, parts[i].bold, parts[i].text)
		// The line of a text is one row tall.
		sc.Text(l.font, x, y-0.5, w, freeLabelSize, parts[i].bold,
			clabinternaltuigfx.AlignStart, parts[i].text, parts[i].color)

		x += w + freeLabelPad
	}
}

// freeLineOf returns the line between two cards, nil when the cards leave no room
// between them.
func (p *mapPage) freeLineOf(
	m *Model, sc *clabinternaltuigfx.Scene, a, b *freeCard, canvas clabinternaltuigfx.Color,
) *freeLine {
	acx, acy := a.center()
	bcx, bcy := b.center()

	l := &freeLine{sc: sc, font: m.textFont(), canvas: canvas, x: sc.X(acx), y: sc.Y(acy)}
	dx, dy := sc.X(bcx)-l.x, sc.Y(bcy)-l.y

	length := math.Hypot(dx, dy)
	if length < 1 {
		return nil
	}

	l.ux, l.uy = dx/length, dy/length
	l.gap = sc.Pt(freeLabelGap)
	l.from = l.reach(sc.X(a.w), sc.Y(a.h)) + l.gap
	l.to = length - l.reach(sc.X(b.w), sc.Y(b.h)) - l.gap

	if l.to <= l.from {
		return nil
	}

	return l
}

// paintFreeLabels writes the texts of a link: the names of its interfaces next to the
// cards, and between them how many links the line stands for and the traffic on it. A
// link too short for all of them keeps the names.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) paintFreeLabels(
	m *Model, sc *clabinternaltuigfx.Scene, e *clabinternaltuitopo.Edge, a, b *freeCard,
	canvas clabinternaltuigfx.Color,
) {
	focused := e.ID == p.edge

	tone := func(c clabinternaltuigfx.Color) clabinternaltuigfx.Color {
		if focused {
			return m.skin().Accent
		}

		return c
	}

	var near, far, middle []freeText

	faint := tone(clabinternaltuigfx.RGB(m.th.Faint.GetForeground()))
	ends := p.paintedState.EndLabels[e.ID]

	if ends[0] != "" {
		near = []freeText{{text: clabinternaltuiui.Sanitize(ends[0]), color: faint, bold: focused}}
	}

	if ends[1] != "" {
		far = []freeText{{text: clabinternaltuiui.Sanitize(ends[1]), color: faint, bold: focused}}
	}

	if n := len(e.Links); n > 1 {
		middle = append(middle, freeText{
			text: strconv.Itoa(n) + "x", color: tone(
				clabinternaltuigfx.RGB(m.th.Palette.Warning)),
		})
	}

	if rate := p.paintedState.EdgeLabels[e.ID]; rate != "" {
		middle = append(middle, freeText{
			text: rate, color: tone(clabinternaltuigfx.RGB(m.th.Palette.Muted)),
		})
	}

	if near == nil && far == nil && middle == nil {
		return
	}

	l := p.freeLineOf(m, sc, a, b, canvas)
	if l == nil {
		return
	}

	nearWidth, nearReach := l.measure(near)
	farWidth, farReach := l.measure(far)

	// Both names, or neither: one of them alone would say little.
	if names := 2 * (nearReach + farReach); names > 0 && l.to-l.from >= names+l.gap {
		if near != nil {
			l.write(near, nearWidth, l.from+nearReach)

			l.from += 2*nearReach + l.gap
		}

		if far != nil {
			l.write(far, farWidth, l.to-farReach)

			l.to -= 2*farReach + l.gap
		}
	}

	if width, reach := l.measure(middle); middle != nil && l.to-l.from >= 2*reach {
		l.write(middle, width, (l.from+l.to)/2)
	}
}

// hitFree returns what is drawn at a position of the free view: the card on top there,
// else the link closest to it, when one is close.
func (p *mapPage) hitFree(x, y float64) (node string, edge int) {
	f := p.free

	inside := func(c *freeCard) bool {
		return c != nil && x >= c.x && x < c.x+c.w && y >= c.y && y < c.y+c.h
	}

	if p.edge < 0 && inside(f.card(p.node)) {
		return p.node, noEdge
	}

	for i := len(f.cards) - 1; i >= 0; i-- {
		if inside(&f.cards[i]) {
			return f.cards[i].n.ID, noEdge
		}
	}

	edge = noEdge
	best := freeReach

	for _, e := range p.layout.Edges() {
		a, b := f.card(e.A), f.card(e.B)
		if a == nil || b == nil {
			continue
		}

		ax, ay := a.center()
		bx, by := b.center()

		// Rows are taller than columns are wide: all of it in widths of a cell.
		if d := segmentDistance(x, y*f.aspect, ax, ay*f.aspect, bx, by*f.aspect); d < best {
			best, edge = d, e.ID
		}
	}

	return "", edge
}

// segmentDistance returns how far a point is from a line between two others.
func segmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay

	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l))
	}

	return math.Hypot(px-ax-t*dx, py-ay-t*dy)
}

// freeNeighbor returns the node the focus moves to in a direction: the closest one that
// lies that way, one that lies more that way than sideways when there is any.
func (p *mapPage) freeNeighbor(dir clabinternaltuitopo.Dir) (string, bool) {
	const (
		sideways = 2   // how much a step aside counts against one ahead
		offside  = 1e6 // what a node that lies more aside than ahead is set back by
	)

	f := p.free

	from := f.card(p.node)
	if from == nil {
		return "", false
	}

	fx, fy := from.center()
	best, bestCost := "", math.Inf(1)

	for i := range f.cards {
		c := &f.cards[i]
		if c.n.ID == p.node {
			continue
		}

		cx, cy := c.center()
		dx, dy := cx-fx, (cy-fy)*f.aspect

		var ahead, aside float64

		switch dir {
		case clabinternaltuitopo.DirLeft:
			ahead, aside = -dx, math.Abs(dy)
		case clabinternaltuitopo.DirRight:
			ahead, aside = dx, math.Abs(dy)
		case clabinternaltuitopo.DirUp:
			ahead, aside = -dy, math.Abs(dx)
		case clabinternaltuitopo.DirDown:
			ahead, aside = dy, math.Abs(dx)
		}

		if ahead <= 0 {
			continue
		}

		cost := ahead + sideways*aside
		if aside > ahead {
			cost += offside
		}

		if cost < bestCost {
			best, bestCost = c.n.ID, cost
		}
	}

	return best, best != ""
}

// freeEdgesOf returns the links of a node in the order tab steps through them: clockwise
// around the node, from the top.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) freeEdgesOf(node string) []int {
	f := p.free

	from := f.card(node)
	if from == nil {
		return nil
	}

	fx, fy := from.center()

	type stop struct {
		id    int
		angle float64
	}

	var stops []stop

	for _, e := range p.layout.Edges() {
		other := e.B
		if e.B == node {
			other = e.A
		} else if e.A != node {
			continue
		}

		c := f.card(other)
		if c == nil {
			continue
		}

		cx, cy := c.center()

		angle := math.Atan2(cx-fx, -(cy-fy)*f.aspect)
		if angle < 0 {
			angle += 2 * math.Pi
		}

		stops = append(stops, stop{id: e.ID, angle: angle})
	}

	sort.SliceStable(stops, func(i, j int) bool { return stops[i].angle < stops[j].angle })

	ids := make([]int, len(stops))
	for i := range stops {
		ids[i] = stops[i].id
	}

	return ids
}
