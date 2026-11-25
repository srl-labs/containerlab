// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"maps"
	"math"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// Sizes of the free view of the map. Distances on screen are in widths of a cell unless
// said otherwise, scales in columns for a pixel of the canvas.
const (
	// freeGapCols and freeGapRows are kept between two cards when it is worked out how
	// far a drawing has to be spread for its cards not to overlap.
	freeGapCols = 2.0
	freeGapRows = 0.8
	// freeMarginCols and freeMarginRows are kept around a drawing fitted to the view.
	freeMarginCols = 2.0
	freeMarginRows = 0.8
	// freeSpread is how much more room than its cards need a fitted drawing gets at
	// most, and freeTight the scale beyond which a pair of nodes that sit almost on
	// top of each other no longer spreads the whole drawing.
	freeSpread = 1.3
	freeTight  = 0.3
	// The bounds of the scale, and what a step of zooming multiplies it by.
	freeMinScale = 0.004
	freeMaxScale = 1.5
	freeZoomStep = 1.25
	// freeDragStart is how far the pointer travels with the button held before a
	// click becomes a drag.
	freeDragStart = 0.4
	// freeSnap is how close to being lined up with another node a moved node snaps
	// into line with it.
	freeSnap = 0.7
	// freeReach is how close to a link a click has to be to mean it.
	freeReach = 0.9
	// freeStep is how far an arrow key moves a node, freeFineStep how far with shift
	// held: about a pixel.
	freeStep     = 1.0
	freeFineStep = 0.125
	// freeEdge is how much of a drawing stays in view, at the least, when the view is
	// moved away from it.
	freeEdge = 6.0
	// freeUndoDepth is the number of moves that can be taken back.
	freeUndoDepth = 64

	// The texts on the links: their size in points, the room around them in cells
	// and rows, and the gap between them and whatever is next on the link, in points.
	freeLabelSize   = textSize - 1
	freeLabelPad    = 0.8
	freeLabelHeight = 0.82
	freeLabelGap    = 4
	freeLabelRound  = 6

	// freeTileGap and freeCardPad make up the width of a card next to its icon and
	// its text, and freeNoteRoom is the room kept for the load figures under a name.
	freeTileGap  = 0.8
	freeCardPad  = 2.6
	freePillPad  = 2.0
	freePillMin  = 4.0
	freeNoteRoom = "100% 999M"
)

// freeCard is a node of the free view as last drawn: its card, in cells of the view.
type freeCard struct {
	n          *clabinternaltuitopo.Node
	x, y, w, h float64
}

func (c *freeCard) center() (x, y float64) { return c.x + c.w/2, c.y + c.h/2 }

// freeMap is the map drawn the way the containerlab application draws a topology: every
// node where it was put, and the links as straight lines between them. It is what the
// pixel layer shows for a lab that has a drawing, and what a node can be moved in.
type freeMap struct {
	// spots holds where the nodes of the lab are on the canvas, also the ones that
	// are not drawn.
	spots map[string]clabinternaltuitopo.Spot
	// scale is the number of columns a pixel of the canvas takes, ox and oy the spot
	// of the canvas at the top left corner of the view, and aspect the height of a
	// cell over its width.
	scale, ox, oy, aspect float64
	// level is how much the cards show.
	level clabinternaltuitopo.Zoom
	// cards are the nodes as last drawn, index where each of them is in the list.
	cards []freeCard
	index map[string]int
	// shape tells which nodes the view was fitted for.
	shape string

	// What the next frame has to work out before it draws: the scale and the level
	// that show the whole drawing, the level for the scale, a view centered on the
	// node in focus, or one that at least shows it.
	refit, relevel, recenter, reveal bool
	// guideX and guideY are the lines a moved node snapped to, as spots.
	guideX, guideY *float64
	// settled is set once the cards were placed for the frame that is being drawn.
	settled bool
}

// toView returns where a spot of the canvas is in the view, in cells.
func (f *freeMap) toView(s clabinternaltuitopo.Spot) (x, y float64) {
	return (s.X - f.ox) * f.scale, (s.Y - f.oy) * f.scale / f.aspect
}

// toCanvas returns the spot of the canvas at a position of the view.
func (f *freeMap) toCanvas(x, y float64) clabinternaltuitopo.Spot {
	return clabinternaltuitopo.Spot{X: f.ox + x/f.scale, Y: f.oy + y*f.aspect/f.scale}
}

func (f *freeMap) card(id string) *freeCard {
	if i, ok := f.index[id]; ok && i < len(f.cards) {
		return &f.cards[i]
	}

	return nil
}

// placedSpot is where a node was moved to here, for as long as the backend does not
// report the node there itself.
type placedSpot struct {
	at clabinternaltuitopo.Spot
	// base is what the backend reported for the node when it was moved, drawn
	// whether it had a position at all. A report that differs is news: of the move
	// that was saved, or of someone else moving the node.
	base  clabinternaltuitopo.Spot
	drawn bool
	// sent is set once the spot was handed to the backend.
	sent bool
}

// freeGrab is what the pointer holds on to since the button went down.
type freeGrab struct {
	// node is the node under the pointer, empty when it is the drawing itself.
	node string
	// x and y are where the button went down, in cells of the view.
	x, y float64
	// spot is where the node was then, ox and oy where the view was.
	spot   clabinternaltuitopo.Spot
	ox, oy float64
	// moving is set once the pointer traveled far enough for a drag.
	moving bool
}

// keyMove is a node being moved with the keyboard.
type keyMove struct {
	node string
	// from is where the node was, placed what was known about it then.
	from   clabinternaltuitopo.Spot
	placed *placedSpot
}

// adoption is a computed layout that was turned into the drawing of the lab by moving a
// node in it, until the move is settled.
type adoption struct {
	// mode is the layout that was asked for, placed what had been moved before.
	mode   clabinternaltuitopo.Mode
	placed map[string]placedSpot
	// replaces is set when the lab had a drawing already.
	replaces bool
}

// placedMsg tells how saving the positions of nodes went.
type placedMsg struct {
	lab string
	err error
}

// movable reports whether the nodes of the map can be moved: the free view is drawn by
// the pixel layer, with its own text.
func (p *mapPage) movable() bool { return p.pix && p.ownText && p.layout != nil }

// freeWanted reports whether the lab is to be shown in the free view.
func (p *mapPage) freeWanted() bool {
	return p.pix && p.ownText && clabinternaltuitopo.Drawn(p.full, p.mode)
}

// syncFree brings the free view in line with the lab: it is set up when the lab is shown
// by its drawing, with a spot for every node, and dropped otherwise.
func (p *mapPage) syncFree(wanted bool) {
	if !wanted {
		p.free = nil

		return
	}

	known := map[string]clabinternaltuitopo.Spot{}

	for i := range p.full.Nodes {
		if n := &p.full.Nodes[i]; n.HasPos {
			known[n.ID] = clabinternaltuitopo.Spot{X: n.X, Y: n.Y}
		}
	}

	if p.free == nil {
		p.free = &freeMap{
			refit: true, level: clabinternaltuitopo.ZoomNormal,
			scale: 1 / clabinternaltuitopo.SpotCol,
		}
	}

	p.free.spots = clabinternaltuitopo.Seed(p.full, known, p.aspect)
	p.free.aspect = p.aspect

	// A drawing that gained or lost nodes is fitted anew, when that is what the
	// user left the view at.
	ids := make([]string, 0, len(p.graph.Nodes))
	for i := range p.graph.Nodes {
		ids = append(ids, p.graph.Nodes[i].ID)
	}

	if shape := strings.Join(ids, "\x00"); shape != p.free.shape {
		p.free.refit = p.free.refit || (p.fit && p.free.shape != "")
		p.free.shape = shape
	}
}

// prunePlaced forgets the moves the backend caught up with.
func (p *mapPage) prunePlaced() {
	for id, o := range p.placed {
		n := p.topoNode(id)
		if n == nil || n.HasPos != o.drawn || (n.HasPos &&
			clabinternaltuitopo.Spot{X: n.X, Y: n.Y} != o.base) {
			delete(p.placed, id)
		}
	}
}

func (p *mapPage) topoNode(id string) *clabinternaltuibackend.TopoNode {
	for i := range p.topology.Nodes {
		if p.topology.Nodes[i].Name == id {
			return &p.topology.Nodes[i]
		}
	}

	return nil
}

// put moves a node to a spot.
func (p *mapPage) put(id string, s clabinternaltuitopo.Spot) {
	o, ok := p.placed[id]
	if !ok {
		if n := p.topoNode(id); n != nil && n.HasPos {
			o.base, o.drawn = clabinternaltuitopo.Spot{X: n.X, Y: n.Y}, true
		}
	}

	o.at, o.sent = s, false

	if p.placed == nil {
		p.placed = map[string]placedSpot{}
	}

	p.placed[id] = o

	if p.free != nil {
		p.free.spots[id] = s
	}
}

// remember keeps where nodes are, so that their next move can be taken back.
func (p *mapPage) remember(ids ...string) {
	if p.free == nil {
		return
	}

	entry := map[string]clabinternaltuitopo.Spot{}

	for _, id := range ids {
		if s, ok := p.free.spots[id]; ok {
			entry[id] = s
		}
	}

	p.undo = append(p.undo, entry)
	if len(p.undo) > freeUndoDepth {
		p.undo = p.undo[len(p.undo)-freeUndoDepth:]
	}
}

// adopt turns the layout on screen into the drawing of the lab: every node gets the spot
// it is drawn at, and the view goes on from where it is, so that nothing moves but what
// the user moves. It is what moving a node in a computed layout starts with.
func (p *mapPage) adopt() {
	if p.free != nil || !p.movable() {
		return
	}

	replaces := false
	for i := range p.topology.Nodes {
		replaces = replaces || p.topology.Nodes[i].HasPos
	}

	before := maps.Clone(p.placed)

	p.adopted = &adoption{mode: p.mode, placed: before, replaces: replaces || len(before) > 0}

	bare := clabinternaltuitopo.Graph{Links: p.full.Links}
	bare.Nodes = append(bare.Nodes, p.full.Nodes...)

	spots := clabinternaltuitopo.Seed(bare, p.layout.Spots(p.aspect), p.aspect)
	for id, s := range spots {
		p.put(id, s)
	}

	ox := float64(p.offsetX) * clabinternaltuitopo.SpotCol
	oy := float64(p.offsetY) * clabinternaltuitopo.SpotCol * p.aspect
	level := p.zoom

	p.mode = clabinternaltuitopo.ModeAuto
	p.compute()

	if f := p.free; f != nil {
		f.scale, f.ox, f.oy, f.level = 1/clabinternaltuitopo.SpotCol, ox, oy, level
		f.refit, f.relevel, f.recenter, f.reveal = false, false, false, false
	}
}

// revert takes an adoption back: the lab is shown the way it was before.
func (p *mapPage) revert() {
	a := p.adopted
	if a == nil {
		return
	}

	p.adopted, p.placed, p.mode = nil, a.placed, a.mode
	p.grabbed, p.moving = nil, nil

	if n := len(p.undo); n > 0 {
		p.undo = p.undo[:n-1]
	}

	p.compute()
}

// commit settles a move: the positions are saved. A move that turned a computed layout
// into the drawing of a lab that has one already asks first, as it replaces that drawing.
func (p *mapPage) commit(m *Model) tea.Cmd {
	a := p.adopted
	if a == nil {
		return p.save(m)
	}

	if !a.replaces {
		p.adopted = nil

		m.notifyf(clabinternaltuitheme.LevelInfo,
			"this layout is now the drawing of %s — u takes a move back", p.lab)

		return p.save(m)
	}

	replaced := false

	form := newForm("Replace the drawing", "Replace", func(formValues) tea.Cmd {
		replaced = true
		p.adopted = nil

		return p.save(m)
	}).dangerous().text(
		p.lab+" has a drawing already: the positions its nodes were given, here",
		"or in the containerlab application. Moving a node in this layout makes",
		"the layout the drawing, and the saved positions are lost.",
	)
	form.onClose = func() {
		if !replaced {
			p.revert()
		}
	}

	return m.ask(form)
}

// save hands the positions that are not saved yet to the backend. With the first of
// them go the positions of the nodes that were never placed: what is saved is the drawing
// as it is on screen, not one node of it.
func (p *mapPage) save(m *Model) tea.Cmd {
	if p.free != nil {
		for i := range p.topology.Nodes {
			n := &p.topology.Nodes[i]
			if _, moved := p.placed[n.Name]; n.HasPos || moved {
				continue
			}

			if s, ok := p.free.spots[n.Name]; ok {
				p.put(n.Name, s)
			}
		}
	}

	positions := map[string]clabinternaltuibackend.Position{}

	for id, o := range p.placed {
		if o.sent {
			continue
		}

		o.sent = true
		p.placed[id] = o
		positions[id] = clabinternaltuibackend.Position{X: o.at.X, Y: o.at.Y}
	}

	if len(positions) == 0 {
		return nil
	}

	be, lab := m.be, p.lab

	return func() tea.Msg { return placedMsg{lab: lab, err: be.PlaceNodes(lab, positions)} }
}

// update hears how saving went. A drawing that cannot be saved stays as it is on screen
// for as long as the UI runs, which the user is told once for a lab.
func (p *mapPage) update(m *Model, msg tea.Msg) tea.Cmd {
	placed, ok := msg.(placedMsg)
	if !ok || placed.err == nil || placed.lab == p.unsaved {
		return nil
	}

	p.unsaved = placed.lab

	m.notifyf(clabinternaltuitheme.LevelWarning,
		"the drawing of %s is not saved, it lasts until the UI is closed: %s",
		placed.lab, clabinternaltuiui.Sanitize(placed.err.Error()))

	return nil
}

// grab implements grabber: in the free view the pointer holds on to the node under it, or
// to the drawing itself, and in a computed layout to a node, which turns the layout into
// a drawing once it is moved.
func (p *mapPage) grab(_ *Model, x, y float64) bool {
	if !p.movable() || p.moving != nil {
		return false
	}

	if width, height := p.viewSize(); x < 0 || y < 0 || x >= float64(width) ||
		y >= float64(height) {
		return false
	}

	g := &freeGrab{x: x, y: y}

	if f := p.free; f != nil {
		g.node, _ = p.hitFree(x, y)
		g.ox, g.oy = f.ox, f.oy
	} else {
		g.node = p.layout.Hit(int(x)+p.offsetX, int(y)+p.offsetY).Node
		if g.node == "" {
			// A computed layout is moved by whole cells, as before.
			return false
		}
	}

	p.grabbed = g

	return true
}

// dragTo implements grabber.
func (p *mapPage) dragTo(_ *Model, x, y float64, free bool) bool {
	g := p.grabbed
	if g == nil {
		return false
	}

	dx, dy := x-g.x, y-g.y

	if !g.moving {
		if math.Hypot(dx, dy*p.aspect) < freeDragStart {
			return false
		}

		if g.node != "" {
			p.adopt()

			if p.free == nil {
				p.grabbed = nil

				return false
			}

			g.spot, g.ox, g.oy = p.free.spots[g.node], p.free.ox, p.free.oy
			p.remember(g.node)
		}

		g.moving = true
	}

	f := p.free
	if f == nil {
		return false
	}

	if g.node == "" {
		f.ox, f.oy = g.ox-dx/f.scale, g.oy-dy*f.aspect/f.scale

		return true
	}

	spot := clabinternaltuitopo.Spot{X: g.spot.X + dx/f.scale, Y: g.spot.Y + dy*f.aspect/f.scale}

	f.guideX, f.guideY = nil, nil

	if !free {
		spot = p.snap(g.node, spot)
	}

	p.put(g.node, spot)

	return true
}

// drop implements grabber.
func (p *mapPage) drop(m *Model) tea.Cmd {
	g := p.grabbed
	p.grabbed = nil

	if p.free != nil {
		p.free.guideX, p.free.guideY = nil, nil
	}

	if g == nil || !g.moving || g.node == "" {
		return nil
	}

	return p.commit(m)
}

// snap lines a moved node up with the nodes it is almost in line with, and takes note of
// the lines it found, which are drawn as guides.
func (p *mapPage) snap(id string, s clabinternaltuitopo.Spot) clabinternaltuitopo.Spot {
	f := p.free
	reach := freeSnap / f.scale
	bestX, bestY := reach, reach

	for i := range f.cards {
		other := f.cards[i].n.ID
		if other == id {
			continue
		}

		at := f.spots[other]

		if d := math.Abs(at.X - s.X); d < bestX {
			bestX, f.guideX = d, &at.X
		}

		if d := math.Abs(at.Y - s.Y); d < bestY {
			bestY, f.guideY = d, &at.Y
		}
	}

	if f.guideX != nil {
		s.X = *f.guideX
	}

	if f.guideY != nil {
		s.Y = *f.guideY
	}

	return s
}

// zoomAt implements zoomer: the wheel scales the free view around the pointer.
func (p *mapPage) zoomAt(m *Model, notches int, x, y float64) bool {
	if p.free == nil {
		return false
	}

	if width, _ := p.viewSize(); x >= float64(width) {
		return false
	}

	p.zoomFree(m, -notches, x, y)

	return true
}

// zoomFree scales the free view by a number of steps, keeping the spot at a position of
// the view where it is. It reports whether the scale changed.
func (p *mapPage) zoomFree(_ *Model, steps int, x, y float64) bool {
	f := p.free

	scale := math.Max(freeMinScale, math.Min(f.scale*math.Pow(freeZoomStep, float64(steps)),
		freeMaxScale))
	if scale == f.scale {
		return false
	}

	at := f.toCanvas(x, y)
	f.scale = scale
	f.ox, f.oy = at.X-x/scale, at.Y-y*f.aspect/scale
	f.relevel, f.refit = true, false
	p.fit = false

	return true
}

// freeKey handles the keys of the free view that the computed layouts know differently.
//
//nolint:mnd // halves and doubles of sizes, in cells and pixels
func (p *mapPage) freeKey(m *Model, key string) bool {
	info := clabinternaltuitheme.LevelInfo
	width, height := p.viewSize()
	cx, cy := float64(width)/2, float64(height)/2

	switch key {
	case "+", "=":
		if !p.zoomFree(m, 1, cx, cy) {
			m.notifyf(info, "this is the largest zoom")
		}
	case "-":
		if !p.zoomFree(m, -1, cx, cy) {
			m.notifyf(info, "this is the smallest zoom")
		}
	case "f":
		p.fit, p.free.refit = true, true

		m.notifyf(info, "fitted to the window")
	default:
		return false
	}

	return true
}

// startMove picks the node in focus up, to be moved with the arrow keys.
func (p *mapPage) startMove(m *Model) {
	if !p.movable() || p.node == "" {
		return
	}

	p.edge = noEdge
	p.adopt()

	if p.free == nil {
		return
	}

	move := &keyMove{node: p.node, from: p.free.spots[p.node]}
	if o, ok := p.placed[p.node]; ok {
		move.placed = &o
	}

	p.remember(p.node)
	p.moving = move
	p.free.reveal = true

	m.notifyf(clabinternaltuitheme.LevelInfo,
		"moving %s — arrows move it, shift for small steps, ⏎ puts it down, esc puts it back",
		p.node)
}

// moveKey handles a key while a node is moved with the keyboard.
func (p *mapPage) moveKey(m *Model, key string) tea.Cmd {
	f, move := p.free, p.moving
	if f == nil {
		p.moving = nil

		return nil
	}

	step := freeStep

	arrow, fine := strings.CutPrefix(key, "shift+")
	if fine {
		step = freeFineStep
	}

	dx, dy := 0.0, 0.0

	switch arrow {
	case "left":
		dx = -step
	case "right":
		dx = step
	case "up":
		dy = -step
	case "down":
		dy = step
	}

	switch {
	case dx != 0 || dy != 0:
		at := f.spots[move.node]
		p.put(move.node, clabinternaltuitopo.Spot{X: at.X + dx/f.scale, Y: at.Y + dy/f.scale})
		f.reveal = true
	case key == keyEnter || key == "g" || key == "space" || key == " ":
		p.moving = nil

		return p.commit(m)
	case key == keyEsc || key == "q":
		p.cancelMove()
	case key == "+" || key == "=" || key == "-" || key == "f":
		p.freeKey(m, key)
	}

	return nil
}

// cancelMove puts the node that is moved with the keyboard back where it was.
func (p *mapPage) cancelMove() {
	move := p.moving
	p.moving = nil

	if move == nil {
		return
	}

	if p.adopted != nil {
		p.revert()

		return
	}

	if n := len(p.undo); n > 0 {
		p.undo = p.undo[:n-1]
	}

	if move.placed != nil {
		p.placed[move.node] = *move.placed
	} else {
		delete(p.placed, move.node)
	}

	if p.free != nil {
		p.free.spots[move.node] = move.from
		p.free.reveal = true
	}
}

// undoMove takes the last move back.
func (p *mapPage) undoMove(m *Model) tea.Cmd {
	if p.free == nil || len(p.undo) == 0 {
		m.notifyf(clabinternaltuitheme.LevelInfo, "no move to take back")

		return nil
	}

	last := p.undo[len(p.undo)-1]
	p.undo = p.undo[:len(p.undo)-1]

	names := make([]string, 0, len(last))

	for id, s := range last {
		p.put(id, s)

		names = append(names, id)
	}

	sort.Strings(names)
	m.notifyf(clabinternaltuitheme.LevelInfo, "%s is back where it was", strings.Join(names, ", "))

	return p.save(m)
}

// freshState gathers the state of the lab anew when what it shows has changed.
func (p *mapPage) freshState(m *Model) {
	want := mapPainted{
		layout: p.layout, node: p.node, edge: p.edge, labels: p.labels, revision: p.revision,
	}

	if p.stateFor != want {
		p.paintedState, p.stateFor = p.state(m), want
		// The grid of a computed layout is painted anew when it is shown again.
		p.grid = nil
	}
}
