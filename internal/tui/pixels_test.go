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
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// fakeTerminal stands in for the terminal the pixel layer is sent to. The output writes
// to it from its own timer as well, hence the lock.
type fakeTerminal struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (t *fakeTerminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.Write(p)
}

func (t *fakeTerminal) Read(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.Read(p)
}

func (t *fakeTerminal) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.String()
}

func (t *fakeTerminal) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.Len()
}

func (t *fakeTerminal) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf.Reset()
}

func (*fakeTerminal) Close() error { return nil }
func (*fakeTerminal) Fd() uintptr  { return 0 }

// Cells of 8.43 by 19 pixels, as a terminal with a 14 pixel font reports them.
const (
	testCellWidth  = 8.43
	testCellHeight = 19
)

// newPixelModel returns a UI on top of the demo backend whose terminal shows images. The
// text is left to the terminal, which is what the golden files of the screens hold.
func newPixelModel(t *testing.T, width, height int) (*Model, *fakeTerminal) {
	t.Helper()

	m, term := newPixelTextModel(t, width, height)
	m.px.text = false

	return m, term
}

// newPixelTextModel returns a UI whose pixel layer writes the text as well.
func newPixelTextModel(t *testing.T, width, height int) (*Model, *fakeTerminal) {
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

				// And with the text written by the layer.
				m, _ = newPixelTextModel(t, width, height)
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

// TestPixelLayerWritesTheText draws a screen with the font of the pixel layer: the
// terminal is left with blank lines, and the layer holds the text.
func TestPixelLayerWritesTheText(t *testing.T) {
	m, _ := newPixelTextModel(t, 120, 35)
	press(m, "enter")

	if text := strings.TrimSpace(ansi.Strip(m.render())); text != "" {
		t.Errorf("the terminal still gets text:\n%s", text)
	}

	written := strings.Join(m.px.drawn.Texts(clabinternaltuigfx.Under), "\n")
	for _, want := range []string{"Nodes", "NAME", "spine1", "nokia_srlinux", "172.20.20.11", "ready", "actions"} {
		if !strings.Contains(written, want) {
			t.Errorf("the layer does not write %q:\n%s", want, written)
		}
	}

	// A dialog is written by the layer as well.
	press(m, "a")
	m.render()

	if written := strings.Join(
		m.px.drawn.Texts(clabinternaltuigfx.Under),
		"\n",
	); !strings.Contains(
		written,
		"open the command line of the node",
	) {
		t.Errorf("the layer does not write the menu:\n%s", written)
	}
}

// TestPixelLayerLeavesLiteralTextToTheTerminal opens the logs of a node: what a node
// printed keeps the fixed width of the terminal, and can be selected there.
func TestPixelLayerLeavesLiteralTextToTheTerminal(t *testing.T) {
	m, _ := newPixelTextModel(t, 120, 35)
	press(m, "enter")
	pressRun(m, "l")

	if _, ok := m.top().(*logsPage); !ok {
		t.Fatalf("expected the logs, got %T", m.top())
	}

	lines := strings.Split(ansi.Strip(m.render()), "\n")
	body := strings.TrimSpace(strings.Join(lines[pixelBodyTop:pixelBodyTop+m.bodyHeight()], ""))
	chrome := strings.TrimSpace(strings.Join(lines[:pixelBodyTop], ""))

	if body == "" {
		t.Error("the terminal does not get the log lines")
	}

	if chrome != "" {
		t.Errorf("the terminal gets the header as text: %q", chrome)
	}
}

func TestParseLineKeepsColumnsAndMarks(t *testing.T) {
	line := "  name   " + clabinternaltuiui.MarkRight(
		"  42%",
	) + "  \x1b[1;38;2;1;2;3mbold text\x1b[m  x"
	cells := parseLine(line)

	if got := len(cells); got != 28 { // the cells of the line above
		t.Fatalf("the line has %d cells, want 28", got)
	}

	// A plain word ends at the gap after it.
	if end := fieldEnd(cells, 2); end != 6 {
		t.Errorf("the word ends at %d, want 6", end)
	}

	// A marked piece is one field, blanks included.
	if cells[9].mark != 1 || fieldEnd(cells, 9) != 14 {
		t.Errorf(
			"the marked piece ends at %d with mark %d, want 14 and 1",
			fieldEnd(cells, 9),
			cells[9].mark,
		)
	}

	// Words of one look that a single space parts stay together.
	if end := fieldEnd(cells, 16); end != 25 || !cells[16].st.bold {
		t.Errorf("the bold text ends at %d (bold: %v), want 25", end, cells[16].st.bold)
	}

	if c := cells[16].st.fg; c.R != 1 || c.G != 2 || c.B != 3 {
		t.Errorf("the color of the bold text is %v", c)
	}

	if clabinternaltuiui.StripMarks(line) == line ||
		strings.Contains(clabinternaltuiui.StripMarks(line), "z") {
		t.Error("the marks are not stripped")
	}
}

func TestNodeSymbolFollowsRoleThenKind(t *testing.T) {
	for _, tc := range []struct {
		role, kind string
		want       clabinternaltuigfx.Symbol
	}{
		{"spine", "nokia_srlinux", clabinternaltuigfx.SymbolSpine},
		{"Leaf", "nokia_srlinux", clabinternaltuigfx.SymbolLeaf},
		{"pe", "nokia_srsim", clabinternaltuigfx.SymbolRouter},
		{"", "linux", clabinternaltuigfx.SymbolClient},
		{"", "bridge", clabinternaltuigfx.SymbolSwitch},
		{"", "fortinet_fortigate", clabinternaltuigfx.SymbolFirewall},
		{"something else", "nokia_srlinux", clabinternaltuigfx.SymbolRouter},
		{"", "", clabinternaltuigfx.SymbolRouter},
	} {
		if got := nodeSymbol(tc.role, tc.kind); got != tc.want {
			t.Errorf("nodeSymbol(%q, %q) is %q, want %q", tc.role, tc.kind, got, tc.want)
		}
	}
}

// TestPixelLayerForTerminalsThatShowImagesOverText draws for the terminal of VS Code,
// which shows one image a cell on top of its text: the layer writes all of the text
// there, also what a node printed, and nothing is placed below the text.
func TestPixelLayerForTerminalsThatShowImagesOverText(t *testing.T) {
	m, term := newPixelTextModel(t, 120, 35)

	if m.px.flat {
		t.Fatal("the terminal is taken for one built on xterm.js without it saying so")
	}

	m.Update(tea.TerminalVersionMsg{Name: "xterm.js(6.1.0-beta.304)"})

	if !m.px.flat || !m.graphical() {
		t.Fatal("the terminal that says it is xterm.js is not drawn for as one")
	}

	press(m, "enter")
	pressRun(m, "l")

	if _, ok := m.top().(*logsPage); !ok {
		t.Fatalf("expected the logs, got %T", m.top())
	}

	screen := m.render()
	assertScreenFits(t, m, screen)

	if text := strings.TrimSpace(ansi.Strip(screen)); text != "" {
		t.Errorf("the terminal gets text, which its images hide:\n%s", text)
	}

	if written := strings.Join(
		m.px.drawn.Texts(clabinternaltuigfx.Under),
		"\n",
	); !strings.Contains(
		written,
		"sr_",
	) {
		t.Errorf("the layer does not write the log:\n%s", written)
	}

	// Sent with a frame of text, no image goes below the text.
	_, _ = m.px.out.Write([]byte("\x1b[?1049h"))

	if sent := term.String(); !strings.Contains(sent, "a=T") || strings.Contains(sent, "z=-") {
		t.Errorf("the images are missing or placed below the text (%d bytes sent)", len(sent))
	}

	// Without a font of its own the layer cannot do without the terminal's text, and
	// the window is drawn plain.
	m.px.text = false

	if m.graphical() {
		t.Error("the pixel layer is drawn although the terminal would hide the text under it")
	}
}

func TestPixelMouseIsAskedForWhenTheTerminalKnowsIt(t *testing.T) {
	// A terminal that says so when asked.
	m, term := newPixelTextModel(t, 120, 35)

	if m.px.mouseOn || strings.Contains(term.String(), "\x1b[?1016h") {
		t.Fatal("the pointer is asked for in pixels before the terminal said that it can")
	}

	m.Update(tea.ModeReportMsg{Mode: ansi.ModeMouseExtSgrPixel, Value: ansi.ModeReset})

	if !m.px.mouseOn || !strings.Contains(term.String(), "\x1b[?1016h") {
		t.Fatal("the pointer is not asked for in pixels although the terminal can")
	}

	// One that does not know the mode.
	m, term = newPixelTextModel(t, 120, 35)
	m.Update(tea.ModeReportMsg{Mode: ansi.ModeMouseExtSgrPixel, Value: ansi.ModeNotRecognized})

	if m.px.mouseOn || strings.Contains(term.String(), "\x1b[?1016h") {
		t.Fatal("the pointer is asked for in pixels although the terminal cannot")
	}

	// And one that is known to, although it does not say so.
	m.Update(tea.TerminalVersionMsg{Name: "par-term(0.31.0)"})

	if !m.px.mouseOn {
		t.Fatal("the pointer is not asked for in pixels in a terminal known to report them")
	}
}

func TestPixelMouseIsTurnedIntoCells(t *testing.T) {
	m, _ := newPixelTextModel(t, 120, 35)
	m.Update(tea.ModeReportMsg{Mode: ansi.ModeMouseExtSgrPixel, Value: ansi.ModeSet})

	// The second tab of the header, a third into its first cell and half way down
	// the second line: the decoder takes one off whatever it is sent.
	m.render()

	zone := m.tabZones[1]
	x := int((float64(zone.from)+0.33)*testCellWidth) - 1
	y := int(math.Round(1.5*testCellHeight)) - 1

	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})

	if m.top().tab() != zone.tab {
		t.Errorf("the click at pixel %d,%d did not open the tab at column %d", x, y, zone.from)
	}

	if math.Abs(m.pointer[0]-float64(zone.from)-0.33) > 0.15 || math.Abs(m.pointer[1]-1.5) > 0.06 {
		t.Errorf("the pointer is taken to be at %.2f,%.2f, want %.2f,1.50",
			m.pointer[0], m.pointer[1], float64(zone.from)+0.33)
	}

	// A position far out of the window is in another unit: back to cells.
	m.Update(tea.MouseMotionMsg{X: 40_000, Y: 12, Button: tea.MouseLeft})

	if m.px.mouseOn {
		t.Fatal("positions that cannot be pixels of the window are still taken for pixels")
	}

	m.Update(tea.MouseClickMsg{X: 7, Y: 3, Button: tea.MouseLeft})

	if m.pointer != [2]float64{7.5, 3.5} {
		t.Errorf("the pointer is taken to be at %v, want the middle of cell 7,3", m.pointer)
	}
}

func TestPixelFramesWaitForTheTerminal(t *testing.T) {
	m, term := newPixelTextModel(t, 120, 35)
	press(m, "enter", "5")

	// A terminal that never said that it answers questions about modes is not asked.
	m.View()

	if m.px.inFlight || strings.Contains(term.String(), pixelAck) && m.px.acks {
		t.Fatal("a frame waits for a terminal that was never heard from")
	}

	m.Update(tea.ModeReportMsg{Mode: ansi.ModeMouseExtSgrPixel, Value: ansi.ModeNotRecognized})

	if !m.px.acks {
		t.Fatal("the answer of the terminal was not taken as a sign that it answers")
	}

	// The next frame that changes pixels goes out with the question, and the one
	// after it waits for the answer.
	press(m, "down")
	m.View()

	if !m.px.inFlight {
		t.Fatal("the frame was sent without asking the terminal to tell when it is done")
	}

	shown := m.px.drawn

	_, cmd := m.Update(keyMsg("down"))
	if cmd == nil {
		t.Error("nothing wakes the UI up should the terminal not answer")
	}

	m.View()

	if m.px.drawn != shown {
		t.Fatal("a frame was drawn while the terminal was busy with the one before")
	}

	// The answer lets the next frame through, with everything that happened since.
	m.Update(tea.ModeReportMsg{Mode: ansi.ModeMouseExtSgrPixel, Value: ansi.ModeNotRecognized})

	if m.px.inFlight {
		t.Fatal("the answer of the terminal did not end the wait")
	}

	m.View()

	if m.px.drawn == shown {
		t.Fatal("no frame was drawn once the terminal was ready")
	}

	// A terminal that stops answering is waited for a few times, and then no more.
	for i := range ackMisses {
		// Each of these keys leaves the focus somewhere else than the frame before.
		press(m, []string{"up", "up", "down"}[i%ackMisses])
		m.View()

		if !m.px.inFlight {
			t.Fatal("the frame was sent without the question")
		}

		m.px.sentAt = m.px.sentAt.Add(-2 * ackTimeout)
		m.Update(frameMsg{})
	}

	if m.px.acks || m.px.inFlight {
		t.Error("a terminal that does not answer anymore is still waited for")
	}
}

func TestPixelScaleFollowsTheTerminal(t *testing.T) {
	// Cells of 19 pixels, and a terminal nothing is known about: twice the pixels.
	m, _ := newPixelTextModel(t, 120, 35)
	cells := int(math.Round(testCellWidth * 120))

	if got := m.metrics().Width; got != 2*cells {
		t.Errorf("the layer is %d pixels wide, want twice the %d the terminal reports", got, cells)
	}

	// One that reports the pixels of the display gets what it reports.
	m.Update(tea.TerminalVersionMsg{Name: "ghostty 1.2.3"})

	if got := m.metrics().Width; got != cells {
		t.Errorf(
			"the layer is %d pixels wide in a terminal that reports %d device pixels",
			got,
			cells,
		)
	}

	// So does one built on xterm.js, which draws images with the pixels it reports.
	m, _ = newPixelTextModel(t, 120, 35)
	m.Update(tea.TerminalVersionMsg{Name: "xterm.js(5.6.0)"})

	if got := m.metrics().Width; got != cells || !m.px.flat {
		t.Errorf("the layer is %d pixels wide in a terminal built on xterm.js, want %d", got, cells)
	}

	// Large cells are those of a high resolution display counted in its own pixels.
	m, _ = newPixelTextModel(t, 120, 35)
	m.Update(uv.PixelSizeEvent{Width: 120 * 14, Height: 35 * 32})

	if got := m.metrics().Height; got != 35*32 {
		t.Errorf("the layer is %d pixels tall with cells of 32 pixels, want %d", got, 35*32)
	}

	// Unless the terminal is known to count without the scale of the display.
	m.Update(tea.TerminalVersionMsg{Name: "par-term(0.31.0)"})

	if got := m.metrics().Height; got != 2*35*32 {
		t.Errorf("the layer is %d pixels tall in such a terminal, want %d", got, 2*35*32)
	}
}
