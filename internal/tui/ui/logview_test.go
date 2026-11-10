// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

func numberedLines(from, to int) []LogLine {
	lines := make([]LogLine, 0, to-from)
	for i := from; i < to; i++ {
		lines = append(lines, LogLine{Text: fmt.Sprintf("line %d", i)})
	}

	return lines
}

func newTestLog(width, height int) *LogView {
	v := NewLogView(clabinternaltuitheme.Default(true))
	v.SetSize(width, height)

	return v
}

func visibleText(v *LogView) []string {
	lines := strings.Split(ansi.Strip(v.View()), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	return lines
}

func TestLogViewFollowsTheEnd(t *testing.T) {
	v := newTestLog(40, 5)
	v.Append(numberedLines(0, 20)...)

	if got := visibleText(v); got[0] != "line 15" || got[4] != "line 19" {
		t.Errorf("expected the last five lines, got %v", got)
	}

	v.Append(numberedLines(20, 22)...)

	if got := visibleText(v); got[4] != "line 21" {
		t.Errorf("new lines did not scroll the view, the last one shown is %q", got[4])
	}
}

// TestLogViewStaysPutWhenScrolledUp reads further up while lines keep arriving: the view
// must not move, and going back to the end resumes following.
func TestLogViewStaysPutWhenScrolledUp(t *testing.T) {
	v := newTestLog(40, 5)
	v.Append(numberedLines(0, 20)...)
	v.Scroll(-3)

	if v.Follow {
		t.Fatal("scrolling up should stop following")
	}

	before := visibleText(v)
	v.Append(numberedLines(20, 30)...)

	if got := visibleText(v); got[0] != before[0] {
		t.Errorf("the view moved from %q to %q while paused", before[0], got[0])
	}

	v.Scroll(100)

	if !v.Follow {
		t.Error("scrolling to the end should resume following")
	}

	if got := visibleText(v); got[4] != "line 29" {
		t.Errorf("after resuming the last line shown is %q", got[4])
	}
}

func TestLogViewAlwaysFillsItsSize(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		for width := 1; width <= 60; width += 7 {
			v := newTestLog(width, 6)
			v.Wrap = wrap
			v.Timestamps = true
			v.Append(LogLine{
				Time: time.Unix(0, 0),
				Text: strings.Repeat("a long line ", 12),
			},
				LogLine{Stderr: true, Level: "ERRO", Text: "short"})

			lines := strings.Split(v.View(), "\n")
			if len(lines) != 6 {
				t.Fatalf("wrap=%v width %d: got %d lines", wrap, width, len(lines))
			}

			for i, line := range lines {
				if w := ansi.StringWidth(line); w != width {
					t.Errorf("wrap=%v width %d: line %d is %d cells wide", wrap, width, i, w)
				}
			}
		}
	}
}

func TestLogViewWrapShowsWholeLines(t *testing.T) {
	v := newTestLog(21, 10)
	v.Append(LogLine{Text: "0123456789abcdefghijklmnopqrstuvwxyz"})

	if got := visibleText(v); strings.Contains(strings.Join(got, ""), "z") {
		t.Errorf("an unwrapped line should be cut, got %v", got)
	}

	v.Wrap = true
	joined := strings.Join(visibleText(v), "")

	if !strings.Contains(joined, "z") || !strings.HasPrefix(joined, "0123") {
		t.Errorf("a wrapped line should be shown in full, got %q", joined)
	}
}

func TestLogViewFind(t *testing.T) {
	v := newTestLog(60, 4)
	v.Append(numberedLines(0, 50)...)
	v.Append(LogLine{Text: "BGP neighbor up"})
	v.Append(numberedLines(51, 80)...)
	v.Append(LogLine{Text: "bgp neighbor down"})

	v.SetQuery("BGP")

	status := strings.Join(v.Status(), " | ")
	if !strings.Contains(status, "find: bgp 2/2") {
		t.Errorf("expected two hits ignoring case, the status is %q", status)
	}

	if got := strings.Join(visibleText(v), "\n"); !strings.Contains(got, "bgp neighbor down") {
		t.Errorf("the search should start at the hit closest to the end:\n%s", got)
	}

	v.NextHit(-1)

	if got := strings.Join(visibleText(v), "\n"); !strings.Contains(got, "BGP neighbor up") {
		t.Errorf("the previous hit is not shown:\n%s", got)
	}

	// The search wraps around.
	v.NextHit(-1)

	if !strings.Contains(strings.Join(v.Status(), " "), "2/2") {
		t.Errorf("the search did not wrap around: %v", v.Status())
	}

	v.SetQuery("nothing like this")

	if !strings.Contains(strings.Join(v.Status(), " "), "(none)") {
		t.Errorf("a search without hits should say so: %v", v.Status())
	}
}

func TestLogViewSanitizesAndCaps(t *testing.T) {
	v := newTestLog(60, 4)
	v.Append(LogLine{Text: "before\x1b[2J\x1b[Hafter\x07"})

	if got := visibleText(v)[0]; got != "beforeafter" {
		t.Errorf("control sequences reached the screen: %q", got)
	}

	v.Reset()
	v.Append(numberedLines(0, logLimit+25)...)

	if v.Len() != logLimit {
		t.Errorf("the view holds %d lines, the limit is %d", v.Len(), logLimit)
	}

	if !strings.Contains(strings.Join(v.Status(), " "), "25 dropped") {
		t.Errorf("dropped lines are not reported: %v", v.Status())
	}
}

func TestLogViewWriteTo(t *testing.T) {
	v := newTestLog(60, 4)
	v.Append(
		LogLine{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Level: "INFO", Text: "started"},
		LogLine{Text: "plain"},
	)

	var b strings.Builder

	if _, err := v.WriteTo(&b); err != nil {
		t.Fatal(err)
	}

	if want := "2026-01-02T03:04:05Z INFO started\nplain\n"; b.String() != want {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}
