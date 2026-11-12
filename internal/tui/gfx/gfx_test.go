// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	white = Color{R: 255, G: 255, B: 255, A: 255}
	red   = Color{R: 255, A: 255}
)

// A window of 10 by 4 cells of 10 by 20 pixels.
func testMetrics() Metrics { return Metrics{Cols: 10, Rows: 4, Width: 100, Height: 80} }

func TestBoxIsRoundAndSharp(t *testing.T) {
	sc := NewScene(testMetrics())
	sc.Box(1, 1, 4, 2, 8, white, red, 1)

	img := sc.Image(Under)

	for _, tc := range []struct {
		name string
		x, y int
		want Color
	}{
		{"outside", 5, 30, Color{}},
		{"the corner is cut off", 10, 20, Color{}},
		{"the edge is the stroke", 30, 20, red},
		{"the middle is the fill", 30, 40, white},
		{"past the right edge", 50, 40, Color{}},
	} {
		got := img.NRGBAAt(tc.x, tc.y)
		if (Color{R: got.R, G: got.G, B: got.B, A: got.A}) != tc.want {
			t.Errorf("%s: pixel %d,%d is %v, want %v", tc.name, tc.x, tc.y, got, tc.want)
		}
	}
}

func TestShapesStayInsideTheirClip(t *testing.T) {
	sc := NewScene(testMetrics())
	sc.Clip(2, 1, 2, 1)
	sc.Fill(0, 0, 10, 4, 0, white)
	sc.Route([]Point{{X: 0, Y: 0}, {X: 10, Y: 4}}, 4, 0, 0, red)
	sc.Unclip()

	img := sc.Image(Under)

	for y := range 80 {
		for x := range 100 {
			inside := x >= 20 && x < 40 && y >= 20 && y < 40
			if painted := img.NRGBAAt(x, y).A != 0; painted != inside {
				t.Fatalf("pixel %d,%d: painted is %v, inside the clip is %v", x, y, painted, inside)
			}
		}
	}
}

func TestPunchMakesAHole(t *testing.T) {
	sc := NewScene(testMetrics())
	sc.Rect(0, 0, 10, 4, white.Alpha(0.5))
	sc.Punch(2, 1, 4, 2, 0)

	img := sc.Image(Under)

	if a := img.NRGBAAt(40, 40).A; a != 0 {
		t.Errorf("the hole has an opacity of %d", a)
	}

	if a := img.NRGBAAt(5, 5).A; a < 120 || a > 135 {
		t.Errorf("around the hole the opacity is %d, want about half", a)
	}
}

// drawn returns what an update draws, for the tests that take nothing away.
func drawn(_, after []byte) []byte { return after }

// images returns the numbers of the images a sequence transmits.
func images(seq []byte) []string {
	var ids []string

	for _, part := range strings.Split(string(seq), apcStart)[1:] {
		for key := range strings.SplitSeq(strings.SplitN(part, ";", 2)[0], ",") {
			if id, ok := strings.CutPrefix(key, "i="); ok {
				ids = append(ids, id)
			}
		}
	}

	return ids
}

func TestScreenSendsOnlyWhatChanged(t *testing.T) {
	screen := NewScreen()

	first := NewScene(testMetrics())
	first.Fill(0, 0, 10, 4, 0, white)

	seq := drawn(screen.Update(first))
	if got := images(seq); len(got) != 4 {
		t.Fatalf("the first frame sent the strips %v, want one for each of the 4 rows", got)
	}

	if !bytes.Contains(seq, []byte(fmt.Sprintf("z=%d", underBackgrounds))) {
		t.Error("the strips are not placed under the cell backgrounds")
	}

	same := NewScene(testMetrics())
	same.Fill(0, 0, 10, 4, 0, white)

	if seq := drawn(screen.Update(same)); len(seq) != 0 {
		t.Errorf("a frame that looks the same sent %q", seq)
	}

	// A dot in the third row changes that strip alone.
	dotted := NewScene(testMetrics())
	dotted.Fill(0, 0, 10, 4, 0, white)
	dotted.Dot(5, 2.5, 3, red)

	if got, want := images(
		drawn(screen.Update(dotted)),
	), fmt.Sprint(
		imageID(Under, 2),
	); len(
		got,
	) != 1 ||
		got[0] != want {
		t.Errorf("the dot sent the strips %v, want only %s", got, want)
	}

	// A strip that empties is taken away by a blank image under its number.
	if seq := string(
		drawn(screen.Update(NewScene(testMetrics()))),
	); strings.Count(
		seq,
		"a=t",
	) != 4 ||
		strings.Contains(seq, "a=T") {
		t.Errorf("an empty frame sent %q, want four blank images", seq)
	}
}

func TestScreenStartsOverAfterAResize(t *testing.T) {
	screen := NewScreen()

	sc := NewScene(testMetrics())
	sc.Fill(0, 0, 10, 4, 0, white)
	screen.Update(sc)

	wider := NewScene(Metrics{Cols: 12, Rows: 2, Width: 120, Height: 40})
	wider.Fill(0, 0, 12, 1, 0, white)

	before, after := screen.Update(wider)

	seq := string(before) + string(after)
	if strings.Count(seq, "a=t") != 4 || strings.Count(seq, "a=T") != 1 {
		t.Errorf("after a resize %q was sent, want the four old strips hidden and one new", seq)
	}

	if got := images(screen.All()); len(got) != 1 {
		t.Errorf("everything on screen is %v, want the one strip", got)
	}
}

// terminal records what is written to it.
type terminal struct {
	mu sync.Mutex
	bytes.Buffer
}

func (t *terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Buffer.Write(p)
}

func (t *terminal) len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Len()
}

func (t *terminal) take() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.String()
	t.Reset()

	return s
}

func (*terminal) Close() error { return nil }
func (*terminal) Fd() uintptr  { return 0 }

func shownOutput(t *testing.T) (*Output, *Screen, *terminal) {
	t.Helper()

	term := &terminal{}
	screen := NewScreen()
	out := NewOutput(term, screen)

	if _, err := out.Write([]byte(altScreenOn)); err != nil {
		t.Fatal(err)
	}

	term.take()

	return out, screen, term
}

func TestOutputAddsThePixelsToAFrame(t *testing.T) {
	out, _, term := shownOutput(t)

	out.Queue(nil, []byte("<pixels>"))

	frame := syncStart + "text" + syncEnd
	if n, err := out.Write([]byte(frame)); err != nil || n != len(frame) {
		t.Fatalf("Write returned %d, %v", n, err)
	}

	want := syncStart + "text" + saveCursor + "<pixels>" + restoreCur + syncEnd
	if got := term.take(); got != want {
		t.Errorf("the terminal got %q, want %q", got, want)
	}

	// Nothing is left to send with the next frame.
	_, _ = out.Write([]byte(frame))

	if got := term.take(); got != frame {
		t.Errorf("the next frame was sent as %q", got)
	}

	// Whatever is written ends with the end of a synchronized update, for the
	// terminals that hold an update back until what they read ends with it.
	_, _ = out.Write([]byte(frame + "\x1b[?25h"))

	if got := term.take(); got != frame+"\x1b[?25h"+syncEnd {
		t.Errorf("a frame followed by more was sent as %q", got)
	}
}

func TestOutputSendsLonelyPixelsOnTheirOwn(t *testing.T) {
	out, _, term := shownOutput(t)

	out.Queue(nil, []byte("<pixels>"))

	deadline := time.Now().Add(2 * time.Second)
	for term.len() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	want := syncStart + saveCursor + "<pixels>" + restoreCur + syncEnd
	if got := term.take(); got != want {
		t.Errorf("the terminal got %q, want %q", got, want)
	}
}

func TestOutputRepairsAfterScrolling(t *testing.T) {
	out, screen, term := shownOutput(t)

	sc := NewScene(testMetrics())
	sc.Fill(0, 0, 10, 1, 0, white)
	out.Queue(screen.Update(sc))

	_, _ = out.Write([]byte("text"))

	term.take()

	// A frame that scrolls takes the images along: all of them are placed again.
	_, _ = out.Write([]byte(syncStart + "\x1b[2;5r\x1b[1S" + syncEnd))

	if got := term.take(); !strings.Contains(got, string(screen.All())) {
		t.Errorf("after scrolling the terminal got %q, without the strips", got)
	}
}

func TestOutputHidesThePixelsOutsideTheUI(t *testing.T) {
	out, screen, term := shownOutput(t)

	sc := NewScene(testMetrics())
	sc.Fill(0, 0, 10, 1, 0, white)
	out.Queue(screen.Update(sc))

	_, _ = out.Write([]byte("text"))

	term.take()

	hide := hideSeq(Under, 0)

	// Leaving the alternate screen, ie. to run a shell, takes the layer away first.
	_, _ = out.Write([]byte(altScreenOff))

	if got := term.take(); got != saveCursor+hide+restoreCur+altScreenOff+syncEnd {
		t.Errorf("leaving sent %q, want the strip hidden first", got)
	}

	// Outside nothing is drawn.
	out.Queue(nil, []byte("<pixels>"))
	_, _ = out.Write([]byte("shell output"))

	if got := term.take(); got != "shell output" {
		t.Errorf("outside the UI the terminal got %q", got)
	}

	// Coming back puts everything on screen again.
	_, _ = out.Write([]byte(altScreenOn + "text"))

	if got := term.take(); !strings.Contains(got, "a=T") {
		t.Errorf("coming back sent %q, without the strips", got)
	}

	out.Stop()

	if got := term.take(); got != saveCursor+hide+restoreCur+syncEnd {
		t.Errorf("stopping sent %q, want the strip hidden", got)
	}
}

func TestDisturbs(t *testing.T) {
	for seq, want := range map[string]bool{
		"plain text":              false,
		"\x1b[1;31mred\x1b[m":     false,
		"\x1b[10;4H\x1b[K":        false,
		"\x1b[?2026h\x1b[?2026l":  false,
		"\x1b[2J":                 true,
		"\x1b[3;20r\x1b[2S":       true,
		"\x1b[2T":                 true,
		"\x1b[3L":                 true,
		"\x1b[M":                  true,
		"text\x1bMmore":           true,
		"\x1b[38;2;10;20;30mtext": false,
	} {
		if got := disturbs([]byte(seq)); got != want {
			t.Errorf("disturbs(%q) is %v, want %v", seq, got, want)
		}
	}
}

func TestEverySymbolHasArt(t *testing.T) {
	frame, arts := symbolArts()

	if len(frame) != 1 || frame[0][0] != frame[0][len(frame[0])-1] {
		t.Errorf("the frame is %d lines, want one closed line", len(frame))
	}

	if len(arts) != 16 {
		t.Errorf("there are %d symbols, want the 16 of the app", len(arts))
	}

	for name, art := range arts {
		if len(art.lines) == 0 {
			t.Errorf("the symbol %q has no lines", name)
		}

		for _, line := range art.lines {
			for _, p := range line {
				if p[0] < 8 || p[0] > 56 || p[1] < 8 || p[1] > 56 {
					t.Errorf("the symbol %q leaves the frame at %v", name, p)
				}
			}
		}
	}

	if !HasSymbol("spine") || HasSymbol("nothing") {
		t.Error("HasSymbol does not tell the symbols apart")
	}
}

func TestPathsAreFlattened(t *testing.T) {
	// A move, lines given as further pairs, a horizontal and a vertical one.
	lines := flattenPath("M1 2 3 4H10V20M0 0L5 5")
	if len(lines) != 2 || len(lines[0]) != 4 || lines[0][3] != [2]float64{10, 20} {
		t.Fatalf("the path became %v", lines)
	}

	// Half a circle from the left to the right of a center, over the top.
	arc := flattenArc([2]float64{0, 10}, [2]float64{20, 10}, 10, 10, false, true)
	end := arc[len(arc)-1]

	if math.Abs(end[0]-20) > 1e-6 || math.Abs(end[1]-10) > 1e-6 {
		t.Errorf("the arc ends at %v, want 20,10", end)
	}

	for _, p := range arc {
		if r := math.Hypot(p[0]-10, p[1]-10); math.Abs(r-10) > 1e-6 {
			t.Errorf("the point %v is %f from the center, want 10", p, r)
		}

		if p[1] > 10+1e-6 {
			t.Errorf("the arc runs below its end points at %v", p)
		}
	}
}

func TestNodeIconDrawsFrameAndSymbol(t *testing.T) {
	sc := NewScene(testMetrics())
	sc.NodeIcon(SymbolLeaf, 5, 2, 40, red, white)

	img := sc.Image(Under)
	reds, whites := 0, 0

	for y := range 80 {
		for x := range 100 {
			switch c := img.NRGBAAt(x, y); {
			case c.A == 0:
			case c.G > 200:
				whites++
			case c.R > 200:
				reds++
			}
		}
	}

	if reds < 20 || whites < 40 {
		t.Errorf("the icon has %d pixels of the symbol and %d of the frame", reds, whites)
	}

	// The middle of the hexagon is not filled.
	if a := img.NRGBAAt(
		50,
		28,
	).A; a != 0 && img.NRGBAAt(51, 29).A != 0 &&
		img.NRGBAAt(49, 27).A != 0 {
		t.Error("the icon is filled")
	}
}

func TestDotsMakeAGrid(t *testing.T) {
	sc := NewScene(testMetrics())
	sc.Dots(0, 0, 10, 4, 10, 1, white)

	// A point is a little more than a pixel here: the dots are 11 pixels apart.
	img := sc.Image(Under)

	if img.NRGBAAt(5, 5).A == 0 || img.NRGBAAt(16, 16).A == 0 || img.NRGBAAt(27, 5).A == 0 {
		t.Error("there is no dot in the middle of a grid cell")
	}

	if img.NRGBAAt(0, 0).A != 0 || img.NRGBAAt(11, 11).A != 0 || img.NRGBAAt(5, 11).A != 0 {
		t.Error("there is a dot between the grid cells")
	}
}

// flatRows returns the rows a sequence sends an image at, in order.
func flatRows(seq []byte) []string {
	sends := regexp.MustCompile(`\x1b\[(\d+);1H\x1b_Ga=T`).FindAllSubmatch(seq, -1)
	rows := make([]string, 0, len(sends))

	for _, m := range sends {
		rows = append(rows, string(m[1]))
	}

	return rows
}

// TestFlatScreenSendsWhatChangedAsOneImage draws for a terminal that shows one image a
// cell, on top of its text, and takes a while for every image: both layers go into one
// image, from the first row that changed down to the last one.
func TestFlatScreenSendsWhatChangedAsOneImage(t *testing.T) {
	screen := NewScreen()

	sc := NewFlatScene(testMetrics())
	sc.Fill(0, 0, 10, 3, 0, white)
	sc.On(Over).Rect(0, 0, 10, 1, red.Alpha(0.5))
	sc.On(Under)

	// The first frame is the whole window. An image cannot both start at the top and
	// end at the bottom: the first rows are one image, the others another.
	before, after := screen.Update(sc)
	if len(before) != 0 {
		t.Errorf("the first frame takes something away: %q", before)
	}

	if got := flatRows(after); strings.Join(got, ",") != "1,3" {
		t.Errorf("the first frame sent images at the rows %v, want 1 and 3", got)
	}

	// The images are sent in their own size, on top of the text.
	for _, key := range []string{"c=", "r=", "z="} {
		if bytes.Contains(after, []byte(","+key)) {
			t.Errorf("an image is sent with %s", key)
		}
	}

	if !bytes.Contains(after, []byte(",s=100,v=40,")) {
		t.Errorf("no image of two rows of the window was sent: %q", after[:60])
	}

	// An image that fits one sequence says nothing about chunks, and none is sent
	// under a number that was used before.
	if bytes.Contains(after, []byte("m=0")) {
		t.Error("an image of one sequence is marked as the last chunk")
	}

	first := images(after)
	if len(first) != 2 || first[0] == first[1] {
		t.Errorf("the images have the numbers %v", first)
	}

	// Nothing changed: nothing is sent.
	same := NewFlatScene(testMetrics())
	same.Fill(0, 0, 10, 3, 0, white)
	same.On(Over).Rect(0, 0, 10, 1, red.Alpha(0.5))
	same.On(Under)

	if before, after = screen.Update(same); len(before)+len(after) != 0 {
		t.Errorf("a frame that changed nothing sent %q and %q", before, after)
	}

	// A change in the third row: one image, from that row down. The region that
	// scrolls is set to the rows above it, so that the image cannot scroll the screen
	// from the last row.
	low := NewFlatScene(testMetrics())
	low.Fill(0, 0, 10, 3, 0, white)
	low.On(Over).Rect(0, 0, 10, 1, red.Alpha(0.5))
	low.On(Under).Dot(5, 2.5, 3, red)

	_, after = screen.Update(low)
	if !bytes.HasPrefix(after, []byte("\x1b[1;2r\x1b[3;1H")) ||
		!bytes.HasSuffix(after, []byte("\x1b[r")) {
		t.Errorf("the change was sent as %q…%q", after[:20], after[len(after)-8:])
	}

	if ids := images(after); len(ids) != 1 || ids[0] == first[0] || ids[0] == first[1] {
		t.Errorf("the change was sent as the images %v, after %v", ids, first)
	}

	if !bytes.Contains(after, []byte(",s=100,v=40,")) {
		t.Errorf("the image does not hold the last two rows: %q", after[:60])
	}

	// A change in the last row alone sends that row.
	last := NewFlatScene(testMetrics())
	last.Fill(0, 0, 10, 3, 0, white)
	last.On(Over).Rect(0, 0, 10, 1, red.Alpha(0.5))
	last.On(Under).Dot(5, 2.5, 3, red)
	last.Dot(5, 3.5, 3, red)

	_, after = screen.Update(last)
	if !bytes.HasPrefix(after, []byte("\x1b[1;3r\x1b[4;1H")) ||
		!bytes.Contains(after, []byte(",v=20,")) {
		t.Errorf("the last row was sent as %q", after[:60])
	}

	// A change in the first row sends everything: its image may take the row below
	// it along, which has to be sent after it.
	top := NewFlatScene(testMetrics())
	top.Fill(0, 0, 10, 3, 0, white)

	before, after = screen.Update(top)
	if got := flatRows(after); strings.Join(got, ",") != "1,3" || len(before) != 0 {
		t.Errorf("a change in the first row sent images at the rows %v, after %q", got, before)
	}

	// The whole layer is taken away by clearing the screen, and put back by sending
	// all of it under new numbers.
	if got := string(screen.Hide()); got != eraseAll {
		t.Errorf("hiding sent %q", got)
	}

	before, after = screen.Repair()
	if string(before) != eraseAll || strings.Join(flatRows(after), ",") != "1,3" {
		t.Errorf("a repair sent %q and images at the rows %v", before, flatRows(after))
	}
}

// TestFlatScreenDrawsBothLayersIntoOneImage looks at the pixels a flat scene is sent
// with: the layer on top of the text is part of them.
func TestFlatScreenDrawsBothLayersIntoOneImage(t *testing.T) {
	screen := NewScreen()

	sc := NewFlatScene(testMetrics())
	sc.Fill(0, 0, 10, 4, 0, white)
	sc.On(Over).Rect(0, 1, 10, 1, red)
	sc.On(Under)

	screen.Update(sc)

	// Twenty rows of pixels a row of text, a hundred pixels each.
	for row, want := range []Color{white, red, white, white} {
		pix := screen.rows[row].pix
		if len(pix) != 100*20*4 {
			t.Fatalf("row %d is kept with %d bytes", row, len(pix))
		}

		at := (10*100 + 50) * 4
		if got := (Color{R: pix[at], G: pix[at+1], B: pix[at+2], A: pix[at+3]}); got != want {
			t.Errorf("row %d is %v in its middle, want %v", row, got, want)
		}
	}
}

func TestOutputTakesAwayBeforeTheFrame(t *testing.T) {
	out, _, term := shownOutput(t)

	out.Queue([]byte("<erase>"), []byte("<pixels>"))

	_, _ = out.Write([]byte(syncStart + "text" + syncEnd))

	want := syncStart + saveCursor + "<erase>" + restoreCur + "text" +
		saveCursor + "<pixels>" + restoreCur + syncEnd
	if got := term.take(); got != want {
		t.Errorf("the terminal got %q, want %q", got, want)
	}
}

func TestTextIsSetOneCharacterACell(t *testing.T) {
	font, err := NewFont(
		testFont(t, "Inter-400.ttf"),
		testFont(t, "Inter-600.ttf"),
		testFont(t, "DejaVuSans.ttf"),
	)
	if err != nil {
		t.Fatal(err)
	}

	sc := NewScene(testMetrics())
	sc.Cells(font, 2, 1, 12, false, "iii", white)

	img := sc.Image(Under)

	// Three narrow letters, each in the middle of its cell of ten pixels.
	for cell := range 3 {
		inked := false

		for x := 20 + cell*10; x < 30+cell*10; x++ {
			for y := 20; y < 40; y++ {
				if img.NRGBAAt(x, y).A > 100 {
					inked = true

					if x < 22+cell*10 || x > 27+cell*10 {
						t.Errorf(
							"the letter of cell %d has ink at the edge of the cell, at %d",
							cell,
							x,
						)
					}
				}
			}
		}

		if !inked {
			t.Errorf("the cell %d holds no letter", cell)
		}
	}
}

func testFont(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "assets", "fonts", name))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestOutputKeepsThePointerReportedInPixels(t *testing.T) {
	out, _, term := shownOutput(t)

	out.PixelMouse(true)

	if got := term.take(); got != mousePixels {
		t.Fatalf("the terminal got %q, want to be asked for pixels", got)
	}

	// Asked a second time, nothing is sent.
	out.PixelMouse(true)

	if got := term.take(); got != "" {
		t.Errorf("the terminal was asked again: %q", got)
	}

	// The renderer turns the mouse on again, ie. after the terminal was lent to a
	// session, and asks for cells as it does: pixels are asked for right after.
	frame := syncStart + "\x1b[?1002h" + mouseCells + "text" + syncEnd
	if n, err := out.Write([]byte(frame)); err != nil || n != len(frame) {
		t.Fatalf("Write returned %d, %v", n, err)
	}

	got := term.take()
	if !strings.HasSuffix(got, mousePixels+syncEnd) ||
		strings.Index(got, mousePixels) < strings.Index(got, mouseCells) {
		t.Errorf("the terminal got %q, want pixels asked for after cells", got)
	}

	// A frame that does not touch the mouse is left alone.
	_, _ = out.Write([]byte(syncStart + "text" + syncEnd))

	if got := term.take(); strings.Contains(got, mousePixels) {
		t.Errorf("the terminal was asked for pixels with a plain frame: %q", got)
	}

	out.PixelMouse(false)

	if got := term.take(); got != mouseNoPixels+mouseCells {
		t.Errorf("the terminal got %q, want to be asked for cells again", got)
	}

	_, _ = out.Write([]byte(frame))

	if got := term.take(); strings.Contains(got, mousePixels) {
		t.Errorf("the terminal was asked for pixels after they were given up: %q", got)
	}
}
