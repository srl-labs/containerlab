// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
)

// fakeTerminal stands in for the terminal the pixel layer is sent to.
type fakeTerminal struct{ bytes.Buffer }

func (*fakeTerminal) Close() error { return nil }
func (*fakeTerminal) Fd() uintptr  { return 0 }

// Cells of 8.43 by 19 pixels, as a terminal with a 14 pixel font reports them.
const (
	testCellWidth  = 8.43
	testCellHeight = 19
)

// newPixelModel returns a UI on top of the demo backend whose terminal shows images.
func newPixelModel(t *testing.T, width, height int) (*Model, *fakeTerminal) {
	t.Helper()

	term := &fakeTerminal{}
	screen := clabinternaltuigfx.NewScreen()
	be := clabinternaltuibackendfake.New(
		clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
	)
	m := New(be, Options{
		Now: fixedClock, Home: "/home/demo", Graphics: GraphicsOn,
		pixels: clabinternaltuigfx.NewOutput(term, screen), screen: screen,
	})
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.Update(uv.PixelSizeEvent{
		Width: int(testCellWidth*float64(width) + 0.5), Height: testCellHeight * height,
	})
	selectLab(t, m, "dc1")

	return m, term
}

// pixelSnapshot compares the text of a screen drawn with the pixel layer with its golden
// file, and writes the layers next to the colored text when asked to, see ansiDirEnv.
func pixelSnapshot(t *testing.T, m *Model, name string) {
	t.Helper()

	snapshot(t, m, name)

	dir := os.Getenv(ansiDirEnv)
	if dir == "" {
		return
	}

	for layer, suffix := range map[clabinternaltuigfx.Layer]string{
		clabinternaltuigfx.Under: ".under.png", clabinternaltuigfx.Over: ".over.png",
	} {
		var buf bytes.Buffer

		if err := png.Encode(&buf, m.px.drawn.Image(layer)); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name+suffix), buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPixelScreenNodes(t *testing.T) {
	m, _ := newPixelModel(t, 120, 35)
	press(m, "enter", "down", "down", "down", "down", "space", "down", "space", "up", "up")

	if !m.graphical() {
		t.Fatal("the window is not drawn with the pixel layer")
	}

	pixelSnapshot(t, m, "pixel_nodes_120x35")
}

func TestPixelScreenMap(t *testing.T) {
	m, _ := newPixelModel(t, 120, 35)
	press(m, "enter", "5")
	pixelSnapshot(t, m, "pixel_map_120x35")
}

func TestPixelScreenLabs(t *testing.T) {
	m, _ := newPixelModel(t, 120, 35)
	pixelSnapshot(t, m, "pixel_labs_120x35")
}

// TestPixelScreensFitAnySize renders the main screens with the pixel layer at many window
// sizes: every line must be exactly as wide as the window.
func TestPixelScreensFitAnySize(t *testing.T) {
	steps := [][]string{
		nil,
		{"enter"},
		{"enter", "enter"},
		{"enter", "5"},
		{"enter", "3"},
		{"?"},
		{"enter", "a"},
	}

	for width := minWidth; width <= 200; width += 13 {
		for _, height := range []int{minHeight, pixelMinHeight, 35, 60} {
			for _, keys := range steps {
				m, _ := newPixelModel(t, width, height)
				press(m, keys...)
				assertScreenFits(t, m, m.render())
			}
		}
	}
}

func TestPixelLayerNeedsTheTerminalToAgree(t *testing.T) {
	term := &fakeTerminal{}
	screen := clabinternaltuigfx.NewScreen()
	be := clabinternaltuibackendfake.New(
		clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
	)
	m := New(be, Options{
		Now: fixedClock, Home: "/home/demo", Graphics: GraphicsAuto,
		pixels: clabinternaltuigfx.NewOutput(term, screen), screen: screen,
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m.Update(uv.PixelSizeEvent{Width: 1012, Height: 665})

	if m.graphical() {
		t.Fatal("the pixel layer is drawn before the terminal said it shows images")
	}

	plain := m.render()

	m.Update(uv.KittyGraphicsEvent{Payload: []byte("OK")})

	if !m.graphical() {
		t.Fatal("the pixel layer is not drawn although the terminal shows images")
	}

	if m.render() == plain {
		t.Error("the window looks the same with the pixel layer")
	}
}

func TestPixelLayerLeavesSmallWindowsPlain(t *testing.T) {
	m, _ := newPixelModel(t, 120, 35)
	m.render()

	if len(m.px.screen.All()) == 0 {
		t.Fatal("nothing was drawn")
	}

	m.Update(tea.WindowSizeMsg{Width: pixelMinWidth - 1, Height: 35})

	if m.graphical() {
		t.Fatal("a window too narrow for it is drawn with the pixel layer")
	}

	screen := m.render()
	assertScreenFits(t, m, screen)

	if len(m.px.screen.All()) != 0 || m.px.shown {
		t.Error("the pixel layer is still on screen")
	}

	// The plain look is the one of a UI without a pixel layer.
	plain, _ := newTestModel(t, pixelMinWidth-1, 35)
	if want := plain.render(); screen != want {
		t.Errorf("the window is drawn as\n%s\nwant\n%s", screen, want)
	}
}

func TestPixelClicksLandWhereTheyShow(t *testing.T) {
	m, _ := newPixelModel(t, 120, 35)
	press(m, "enter")
	m.render()

	page, ok := m.top().(*nodesPage)
	if !ok {
		t.Fatalf("expected the list of nodes, got %T", m.top())
	}

	// The third row of the table: below the header of the card the body starts with.
	m.Update(tea.MouseClickMsg{X: 10, Y: pixelBodyTop + 3, Button: tea.MouseLeft})

	if n := page.current(); n == nil || n.Name != "leaf1" {
		t.Fatalf("the click selected %v, want leaf1", n)
	}

	m.render()

	// A button of the details pane does what its key does.
	var logs *pixelZone

	for i := range m.px.zones {
		if m.px.zones[i].key.Text == "l" {
			logs = &m.px.zones[i]
		}
	}

	if logs == nil {
		t.Fatal("the pane has no button for the logs")
	}

	_, cmd := m.Update(tea.MouseClickMsg{X: logs.x + 1, Y: logs.y, Button: tea.MouseLeft})
	run(m, cmd, 0)

	if _, ok := m.top().(*logsPage); !ok {
		t.Errorf("the button opened %T, want the logs", m.top())
	}

	// A tab of the header is clicked on any of its three lines.
	m.Update(tea.MouseClickMsg{X: m.tabZones[4].from + 1, Y: 2, Button: tea.MouseLeft})

	if m.top().tab() != tabMap {
		t.Errorf("the click on the tab went to %T", m.top())
	}
}

// TestPixelLayerStaysWithinBudget draws a very large window: the layer is drawn with
// fewer pixels there, so that it fits what terminals keep of images.
func TestPixelLayerStaysWithinBudget(t *testing.T) {
	m, _ := newPixelModel(t, 440, 110)

	small, _ := newPixelModel(t, 120, 35)
	if got := small.metrics(); got.Width != 2*1012 {
		t.Errorf(
			"a window of 120 columns is drawn %d pixels wide, want twice its %d",
			got.Width,
			1012,
		)
	}

	got := m.metrics()
	if pixels := got.Width * got.Height; pixels > pixelBudget+pixelBudget/100 {
		t.Errorf("the layer has %d pixels, want at most %d", pixels, pixelBudget)
	}

	if got.Width < got.Cols || got.Height < got.Rows {
		t.Errorf(
			"the layer of %dx%d pixels is smaller than the %dx%d cells",
			got.Width,
			got.Height,
			got.Cols,
			got.Rows,
		)
	}

	assertScreenFits(t, m, m.render())
}
