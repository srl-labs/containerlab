// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
)

// freeMapModel returns a UI that shows the map of the demo lab with the pixel layer.
func freeMapModel(t *testing.T) (*Model, *mapPage) {
	t.Helper()

	m, _ := newPixelTextModel(t, 140, 40)
	press(m, "enter", "5")
	m.render()

	p, ok := m.top().(*mapPage)
	if !ok {
		t.Fatalf("expected the map, got %T", m.top())
	}

	return m, p
}

// picture writes what the pixel layer shows to a file, when asked to: see ansiDirEnv.
func picture(t *testing.T, m *Model, name string) {
	t.Helper()

	m.render()

	dir := os.Getenv(ansiDirEnv)
	if dir == "" {
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, m.px.drawn.Image(clabinternaltuigfx.Under)); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, name+".png"), buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// point sets where the pointer is, in cells of the window, and returns the cell.
func point(m *Model, x, y float64, mod tea.KeyMod) tea.Mouse {
	m.pointer = [2]float64{x, y}

	return tea.Mouse{X: int(x), Y: int(y), Button: tea.MouseLeft, Mod: mod}
}

// pressMouse presses the button at a position of the window and moves the pointer to
// another one in steps, with a modifier key held.
func pressMouse(m *Model, fromX, fromY, toX, toY float64, mod tea.KeyMod) {
	const steps = 8

	// The messages go to the handlers directly: Update would work the position out
	// from the cell alone.
	run(m, m.handleClick(tea.MouseClickMsg(point(m, fromX, fromY, mod))), 0)

	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		m.handleDrag(tea.MouseMotionMsg(point(m, fromX+(toX-fromX)*t, fromY+(toY-fromY)*t, mod)))
		m.render()
	}
}

// releaseMouse lets the button go, following the commands that come out of it.
func releaseMouse(m *Model) {
	run(m, m.handleRelease(), 0)
	m.Update(flushMsg{})
	m.render()
}

// dragMouse moves what is at a position of the window to another one, with the key held
// that keeps it from being lined up with anything.
func dragMouse(m *Model, fromX, fromY, toX, toY float64) {
	pressMouse(m, fromX, fromY, toX, toY, tea.ModAlt)
	releaseMouse(m)
}

// cardCenter returns the middle of the card or the box of a node, in cells of the window.
func cardCenter(t *testing.T, p *mapPage, node string) (x, y float64) {
	t.Helper()

	if p.free != nil {
		c := p.free.card(node)
		if c == nil {
			t.Fatalf("%s is not drawn", node)
		}

		x, y = c.center()

		return x + pixelInset, y + pixelBodyTop
	}

	r, ok := p.layout.NodeRect(node)
	if !ok {
		t.Fatalf("%s is not drawn", node)
	}

	return float64(r.X-p.offsetX+pixelInset) + float64(r.W)/2,
		float64(r.Y-p.offsetY+pixelBodyTop) + float64(r.H)/2
}

func positions(m *Model, lab string) map[string]clabinternaltuibackend.TopoNode {
	out := map[string]clabinternaltuibackend.TopoNode{}

	topo := m.be.Topology(lab)
	for i := range topo.Nodes {
		out[topo.Nodes[i].Name] = topo.Nodes[i]
	}

	return out
}

func TestMapDragTurnsLayoutIntoDrawing(t *testing.T) {
	m, p := freeMapModel(t)

	if p.free != nil {
		t.Fatal("a lab without a drawing is shown in the free view")
	}

	picture(t, m, "free_0_layout")

	node := p.node
	x, y := cardCenter(t, p, node)

	// Every other node stays where it is on screen.
	before := map[string][2]float64{}

	for i := range p.graph.Nodes {
		id := p.graph.Nodes[i].ID
		cx, cy := cardCenter(t, p, id)
		before[id] = [2]float64{cx, cy}
	}

	dragMouse(m, x, y, x+10.25, y+3.5)

	if p.free == nil {
		t.Fatal("moving a node did not turn the layout into a drawing")
	}

	picture(t, m, "free_1_dragged")

	for id, was := range before {
		cx, cy := cardCenter(t, p, id)
		wantX, wantY := was[0], was[1]

		if id == node {
			wantX, wantY = was[0]+10.25, was[1]+3.5
		}

		// A card starts on a whole pixel, which is a fraction of a cell.
		if math.Abs(cx-wantX) > 0.2 || math.Abs(cy-wantY) > 0.2 {
			t.Errorf("%s is at %.2f,%.2f, want %.2f,%.2f", id, cx, cy, wantX, wantY)
		}
	}

	// The whole drawing is saved, not the moved node alone.
	saved := positions(m, p.lab)
	for i := range p.full.Nodes {
		if !saved[p.full.Nodes[i].ID].HasPos {
			t.Errorf("%s has no saved position", p.full.Nodes[i].ID)
		}
	}

	if len(p.placed) != 0 {
		t.Errorf("%d moves are kept although the backend reports them", len(p.placed))
	}

	if p.layout.Mode() != clabinternaltuitopo.ModeSaved || p.free == nil {
		t.Error("the saved drawing is not what is shown")
	}
}

// drawnMapModel returns a UI that shows the demo lab in the free view: one of its nodes
// was moved, which made the layout the drawing of the lab.
func drawnMapModel(t *testing.T) (*Model, *mapPage) {
	t.Helper()

	m, p := freeMapModel(t)
	x, y := cardCenter(t, p, p.node)
	dragMouse(m, x, y, x+4, y)

	if p.free == nil {
		t.Fatal("the lab is not shown in the free view")
	}

	return m, p
}

func spotOf(t *testing.T, m *Model, p *mapPage, node string) clabinternaltuitopo.Spot {
	t.Helper()

	n, ok := positions(m, p.lab)[node]
	if !ok || !n.HasPos {
		t.Fatalf("%s has no saved position", node)
	}

	return clabinternaltuitopo.Spot{X: n.X, Y: n.Y}
}

func TestMapDragMovesByFractionsOfACell(t *testing.T) {
	m, p := drawnMapModel(t)

	node := "leaf2"
	before := spotOf(t, m, p, node)
	x, y := cardCenter(t, p, node)

	// A tenth of a cell is less than a pixel of the terminal here: the node still
	// moves by as much as the pointer does.
	dragMouse(m, x, y, x+2.1, y+0.6)

	after := spotOf(t, m, p, node)
	wantX := before.X + 2.1/p.free.scale
	wantY := before.Y + 0.6*p.free.aspect/p.free.scale

	if math.Abs(after.X-wantX) > 0.01 || math.Abs(after.Y-wantY) > 0.01 {
		t.Errorf("%s was saved at %.2f,%.2f, want %.2f,%.2f", node, after.X, after.Y, wantX, wantY)
	}

	// The other nodes keep their positions.
	if got := spotOf(t, m, p, "leaf3"); got != p.free.spots["leaf3"] {
		t.Errorf("leaf3 is shown at %v and saved at %v", p.free.spots["leaf3"], got)
	}
}

func TestMapClickDoesNotMoveANode(t *testing.T) {
	m, p := drawnMapModel(t)

	before := spotOf(t, m, p, "leaf2")
	x, y := cardCenter(t, p, "leaf2")

	// The pointer shakes a little between press and release.
	dragMouse(m, x, y, x+0.2, y+0.05)

	if p.node != "leaf2" {
		t.Errorf("the click focused %s, want leaf2", p.node)
	}

	if got := spotOf(t, m, p, "leaf2"); got != before {
		t.Errorf("the click moved the node from %v to %v", before, got)
	}
}

func TestMapDragSnapsIntoLine(t *testing.T) {
	m, p := drawnMapModel(t)

	x, y := cardCenter(t, p, "leaf2")
	other := p.free.spots["leaf3"]

	// Up, and almost level with nothing: then nearly straight under another node.
	tx, _ := p.free.toView(clabinternaltuitopo.Spot{X: other.X})
	pressMouse(m, x, y, tx+pixelInset-0.4, y+6.3, 0)

	if p.free.guideX == nil {
		t.Fatal("no guide is shown for a node moved almost in line with another")
	}

	picture(t, m, "free_2_guide")
	releaseMouse(m)

	if got := spotOf(t, m, p, "leaf2"); math.Abs(got.X-other.X) > 0.01 {
		t.Errorf("leaf2 was saved at x %.2f, want it in line with leaf3 at %.2f", got.X, other.X)
	}

	if p.free.guideX != nil || p.free.guideY != nil {
		t.Error("the guides stay after the node was put down")
	}
}

func TestMapDragOfTheCanvasMovesTheView(t *testing.T) {
	m, p := drawnMapModel(t)

	saved := positions(m, p.lab)
	x, y := cardCenter(t, p, "leaf2")

	// Empty canvas, well under the drawing.
	dragMouse(m, 20, 34, 23.5, 32.25)

	nx, ny := cardCenter(t, p, "leaf2")
	if math.Abs(nx-x-3.5) > 0.2 || math.Abs(ny-y+1.75) > 0.2 {
		t.Errorf("the view moved by %.2f,%.2f, want 3.5,-1.75", nx-x, ny-y)
	}

	for name, n := range positions(m, p.lab) {
		if n.X != saved[name].X || n.Y != saved[name].Y {
			t.Errorf("moving the view moved %s", name)
		}
	}
}

func TestMapMoveWithTheKeyboard(t *testing.T) {
	m, p := drawnMapModel(t)

	press(m, "down")

	node := p.node
	before := spotOf(t, m, p, node)

	press(m, "g", "right", "right", "down", "shift+left")

	if p.moving == nil {
		t.Fatal("g did not pick the node up")
	}

	picture(t, m, "free_3_keyboard")
	pressRun(m, "enter")
	m.Update(flushMsg{})

	if p.moving != nil {
		t.Fatal("enter did not put the node down")
	}

	after := spotOf(t, m, p, node)
	wantX := before.X + (2*freeStep-freeFineStep)/p.free.scale
	wantY := before.Y + freeStep/p.free.scale

	if math.Abs(after.X-wantX) > 0.01 || math.Abs(after.Y-wantY) > 0.01 {
		t.Errorf("%s was saved at %.2f,%.2f, want %.2f,%.2f", node, after.X, after.Y, wantX, wantY)
	}

	// Esc puts a node back where it was picked up.
	press(m, "g", "left", "left", "up", "esc")
	m.render()

	if got := p.free.spots[node]; got != after {
		t.Errorf("esc left %s at %v, want %v", node, got, after)
	}

	// And u takes the move before that back.
	pressRun(m, "u")
	m.Update(flushMsg{})

	if got := spotOf(
		t,
		m,
		p,
		node,
	); math.Abs(got.X-before.X) > 0.01 ||
		math.Abs(got.Y-before.Y) > 0.01 {
		t.Errorf("u left %s at %v, want %v", node, got, before)
	}
}

func TestMapMoveInALayoutCanBeCalledOff(t *testing.T) {
	m, p := freeMapModel(t)

	press(m, "g", "right", "esc")
	m.render()

	if p.free != nil || p.adopted != nil || len(p.placed) != 0 {
		t.Error("calling a move off left the layout turned into a drawing")
	}

	for name, n := range positions(m, p.lab) {
		if n.HasPos {
			t.Errorf("%s was saved although the move was called off", name)
		}
	}
}

func TestMapMoveAsksBeforeReplacingADrawing(t *testing.T) {
	m, p := drawnMapModel(t)
	saved := positions(m, p.lab)

	// The tiers of the lab, although it has a drawing.
	press(m, "v")
	m.render()

	if p.free != nil {
		t.Fatalf("v shows %v in the free view", p.mode)
	}

	x, y := cardCenter(t, p, "leaf2")
	dragMouse(m, x, y, x+6, y+2)

	if len(m.overlays) != 1 {
		t.Fatalf("%d dialogs are open, want the question whether to replace the drawing",
			len(m.overlays))
	}

	picture(t, m, "free_4_replace")

	// Escape calls it off: the drawing is as it was, and so is the layout shown.
	press(m, "esc")
	m.Update(flushMsg{})
	m.render()

	if p.free != nil || p.mode != clabinternaltuitopo.ModeTiers {
		t.Error("the layout is not shown anymore after the move was called off")
	}

	for name, n := range positions(m, p.lab) {
		if n.X != saved[name].X || n.Y != saved[name].Y {
			t.Errorf("%s moved although the move was called off", name)
		}
	}

	// Again, and this time it is meant.
	x, y = cardCenter(t, p, "leaf2")
	dragMouse(m, x, y, x+6, y+2)
	pressRun(m, "y")
	m.Update(flushMsg{})
	m.render()

	if p.free == nil {
		t.Fatal("the layout was not made the drawing")
	}

	changed := false

	for name, n := range positions(m, p.lab) {
		changed = changed || n.X != saved[name].X || n.Y != saved[name].Y
	}

	if !changed {
		t.Error("the drawing was not replaced")
	}
}

func TestMapKeysOfTheFreeView(t *testing.T) {
	m, p := drawnMapModel(t)

	// Every node is reached with the arrows from every other one, wherever the nodes
	// were put.
	focus := p.node

	for i := range p.graph.Nodes {
		start := p.graph.Nodes[i].ID
		seen := map[string]bool{start: true}

		for queue := []string{start}; len(queue) > 0; queue = queue[1:] {
			p.node = queue[0]

			for _, dir := range []clabinternaltuitopo.Dir{
				clabinternaltuitopo.DirLeft, clabinternaltuitopo.DirRight,
				clabinternaltuitopo.DirUp, clabinternaltuitopo.DirDown,
			} {
				if next, ok := p.freeNeighbor(dir); ok && !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}

		if len(seen) != len(p.graph.Nodes) {
			t.Errorf(
				"the arrows reach %d of %d nodes from %s",
				len(seen),
				len(p.graph.Nodes),
				start,
			)
		}
	}

	p.node = focus

	press(m, "down")

	if p.node == focus {
		t.Errorf("down does not leave %s", focus)
	}

	// Tab steps through the links of the node, and back to the node.
	links := len(p.freeEdgesOf(p.node))

	for i := range links {
		press(m, "tab")

		if p.edge < 0 {
			t.Fatalf("tab %d of %d left the links", i+1, links)
		}
	}

	press(m, "tab")

	if p.edge >= 0 {
		t.Error("tab does not come back to the node after its last link")
	}

	// Zooming keeps the middle of the view where it is.
	width, height := p.viewSize()
	middle := p.free.toCanvas(float64(width)/2, float64(height)/2)
	scale := p.free.scale

	press(m, "+")
	m.render()

	if p.free.scale <= scale {
		t.Error("+ did not zoom in")
	}

	if got := p.free.toCanvas(
		float64(width)/2,
		float64(height)/2,
	); math.Abs(
		got.X-middle.X,
	) > 0.01 ||
		math.Abs(got.Y-middle.Y) > 0.01 {
		t.Errorf("zooming moved the middle of the view from %v to %v", middle, got)
	}

	press(m, "f")
	m.render()

	for i := range p.free.cards {
		c := &p.free.cards[i]
		if c.x < 0 || c.y < 0 || c.x+c.w > float64(width) || c.y+c.h > float64(height) {
			t.Errorf("%s is not in view after fitting", c.n.ID)
		}
	}

	picture(t, m, "free_5_fitted")
}

func TestMapWheelZoomsAroundThePointer(t *testing.T) {
	m, p := drawnMapModel(t)

	x, y := cardCenter(t, p, "leaf3")
	spot := p.free.spots["leaf3"]
	scale := p.free.scale

	m.pointer = [2]float64{x, y}
	m.handleWheel(
		tea.MouseWheelMsg{X: int(x), Y: int(y), Button: tea.MouseWheelUp, Mod: tea.ModCtrl},
	)
	m.render()

	if p.free.scale <= scale {
		t.Fatal("the wheel with control held did not zoom in")
	}

	// What is under the pointer stays under it.
	if vx, vy := p.free.toView(
		spot,
	); math.Abs(vx+pixelInset-x) > 0.01 ||
		math.Abs(vy+pixelBodyTop-y) > 0.01 {
		t.Errorf("leaf3 went from %.2f,%.2f to %.2f,%.2f", x, y, vx+pixelInset, vy+pixelBodyTop)
	}

	// Without control the wheel moves the view, as it does in the computed layouts.
	oy := p.free.oy

	m.handleWheel(tea.MouseWheelMsg{X: int(x), Y: int(y), Button: tea.MouseWheelDown})

	if p.free.scale <= scale || p.free.oy <= oy {
		t.Error("the wheel alone does not scroll the drawing")
	}
}

func TestDraggingDrawsNoMoreFramesThanNeeded(t *testing.T) {
	m, p := drawnMapModel(t)

	x, y := cardCenter(t, p, "leaf3")
	run(m, m.handleClick(tea.MouseClickMsg(point(m, x, y, tea.ModAlt))), 0)

	first := m.View().Content
	shown := m.px.drawn

	// A burst of moves, as a pointer reports them: the window is not drawn for each.
	for i := 1; i <= 5; i++ {
		_, cmd := m.Update(
			tea.MouseMotionMsg{X: int(x) + i, Y: int(y), Button: tea.MouseLeft, Mod: tea.ModAlt},
		)
		if (cmd != nil) != (i == 1) {
			t.Errorf("move %d: a later frame was asked for: %v", i, cmd != nil)
		}

		if m.View().Content != first || m.px.drawn != shown {
			t.Fatalf("move %d was drawn right away", i)
		}
	}

	// The frame that follows shows where the node went.
	m.frameAt = m.frameAt.Add(-time.Second)
	m.Update(frameMsg{})
	m.View()

	if m.px.drawn == shown {
		t.Fatal("the moves were never drawn")
	}

	if nx, _ := cardCenter(t, p, "leaf3"); nx-x < 4 {
		t.Errorf("leaf3 moved by %.2f cells, want 5", nx-x)
	}

	releaseMouse(m)
}
