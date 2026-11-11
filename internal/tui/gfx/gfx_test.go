// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"bytes"
	"fmt"
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

	seq := screen.Update(first)
	if got := images(seq); len(got) != 4 {
		t.Fatalf("the first frame sent the strips %v, want one for each of the 4 rows", got)
	}

	if !bytes.Contains(seq, []byte(fmt.Sprintf("z=%d", underBackgrounds))) {
		t.Error("the strips are not placed under the cell backgrounds")
	}

	same := NewScene(testMetrics())
	same.Fill(0, 0, 10, 4, 0, white)

	if seq := screen.Update(same); len(seq) != 0 {
		t.Errorf("a frame that looks the same sent %q", seq)
	}

	// A dot in the third row changes that strip alone.
	dotted := NewScene(testMetrics())
	dotted.Fill(0, 0, 10, 4, 0, white)
	dotted.Dot(5, 2.5, 3, red)

	if got, want := images(screen.Update(dotted)), fmt.Sprint(imageID(Under, 2)); len(got) != 1 ||
		got[0] != want {
		t.Errorf("the dot sent the strips %v, want only %s", got, want)
	}

	// A strip that empties is taken away by a blank image under its number.
	if seq := string(screen.Update(NewScene(testMetrics()))); strings.Count(seq, "a=t") != 4 ||
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

	seq := string(screen.Update(wider))
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

	out.Queue([]byte("<pixels>"))

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

	out.Queue([]byte("<pixels>"))

	deadline := time.Now().Add(2 * time.Second)
	for term.Len() == 0 && time.Now().Before(deadline) {
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

	if got := term.take(); got != hide+altScreenOff+syncEnd {
		t.Errorf("leaving sent %q, want the strip hidden first", got)
	}

	// Outside nothing is drawn.
	out.Queue([]byte("<pixels>"))
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

	if got := term.take(); got != hide+syncEnd {
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
