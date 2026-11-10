// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"spine1", 10, "spine1"},
		{"spine1", 6, "spine1"},
		{"spine1", 4, "spi…"},
		{"spine1", 1, "…"},
		{"spine1", 0, ""},
		{"köln-leaf", 5, "köln…"},
	}

	for _, tt := range tests {
		if got := Truncate(tt.in, tt.width); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}

func TestTruncateLeftKeepsTheEnd(t *testing.T) {
	got := TruncateLeft("/home/user/labs/dc1/dc1.clab.yml", 16)
	if got != "…c1/dc1.clab.yml" {
		t.Errorf("got %q", got)
	}

	if w := Width(got); w != 16 {
		t.Errorf("width is %d, want 16", w)
	}

	if got := TruncateLeft("short", 16); got != "short" {
		t.Errorf("a fitting value was changed to %q", got)
	}
}

func TestPadFitsExactly(t *testing.T) {
	for _, s := range []string{"", "a", "leaf1", "a much longer value than fits"} {
		for width := 1; width < 12; width++ {
			if w := Width(PadRight(s, width)); w != width {
				t.Errorf("PadRight(%q, %d) is %d cells wide", s, width, w)
			}

			if w := Width(PadLeft(s, width)); w != width {
				t.Errorf("PadLeft(%q, %d) is %d cells wide", s, width, w)
			}
		}
	}
}

func TestSanitizeRemovesControlSequences(t *testing.T) {
	tests := map[string]string{
		"plain":                      "plain",
		"\x1b[31mred\x1b[0m":         "red",
		"move\x1b[2Aup":              "moveup",
		"title\x1b]0;owned\x07after": "titleafter",
		"bell\x07 and\rreturn":       "bell andreturn",
		"tab\there":                  "tab    here",
		"line\nbreak":                "linebreak",
	}

	for in, want := range tests {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFitLinesReturnsTheRequestedSize(t *testing.T) {
	got := FitLines("one\na line that is much too long\nthree\nfour", 8, 3)
	lines := strings.Split(got, "\n")

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	for i, line := range lines {
		if w := Width(line); w != 8 {
			t.Errorf("line %d is %d cells wide, want 8: %q", i, w, line)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	ordered := []string{"leaf1", "leaf2", "leaf10", "leaf10a", "Spine1", "spine02", "spine3"}

	for i := 0; i+1 < len(ordered); i++ {
		if !NaturalLess(ordered[i], ordered[i+1]) {
			t.Errorf("%q should sort before %q", ordered[i], ordered[i+1])
		}

		if NaturalLess(ordered[i+1], ordered[i]) {
			t.Errorf("%q should not sort before %q", ordered[i+1], ordered[i])
		}
	}

	if NaturalLess("leaf1", "leaf1") {
		t.Error("a value must not sort before itself")
	}
}

func TestRateAndBytes(t *testing.T) {
	rates := map[float64]string{
		0: "0", 0.4: "0", 532: "532", 1000: "1.0k", 9_600: "9.6k", 96_000: "96k",
		1_200_000: "1.2M", 48_200_000: "48M", 2_400_000_000: "2.4G",
	}

	for in, want := range rates {
		if got := Rate(in); got != want {
			t.Errorf("Rate(%v) = %q, want %q", in, got, want)
		}

		if w := Width(Rate(in)); w > 4 {
			t.Errorf("Rate(%v) = %q is wider than 4 cells", in, Rate(in))
		}
	}

	sizes := map[uint64]string{
		0: "0", 900: "900", 24 << 20: "24M", 712 << 20: "712M", 9783 << 20: "9.6G",
	}

	for in, want := range sizes {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	tests := map[time.Duration]string{
		42 * time.Second:               "42s",
		3*time.Minute + 12*time.Second: "3m12s",
		2*time.Hour + 5*time.Minute:    "2h05m",
		4*24*time.Hour + 3*time.Hour:   "4d",
	}

	for in, want := range tests {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPorts(t *testing.T) {
	ports := []clabinternaltuibackend.Port{
		{ContainerPort: 443, Protocol: "tcp"},
		{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{HostIP: "::", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{ContainerPort: 22, Protocol: "tcp"},
	}

	if got, want := Ports(ports), "22/tcp, 8080→80/tcp, 443/tcp"; got != want {
		t.Errorf("Ports() = %q, want %q", got, want)
	}

	if got := Ports(nil); got != "" {
		t.Errorf("Ports(nil) = %q, want it empty", got)
	}
}

func TestShortImage(t *testing.T) {
	tests := map[string]string{
		"alpine:3":                                  "alpine:3",
		"ghcr.io/nokia/srlinux:25.10.1":             "nokia/srlinux:25.10.1",
		"registry.example.com/a/b/c/image@sha256:x": "c/image",
	}

	for in, want := range tests {
		if got := ShortImage(in); got != want {
			t.Errorf("ShortImage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNetem(t *testing.T) {
	if got := Netem(nil); got != Dash {
		t.Errorf("no impairment renders as %q", got)
	}

	got := Netem(&clabinternaltuibackend.Impairment{
		Delay: 50 * time.Millisecond, Jitter: 5 * time.Millisecond, LossPct: 1.5, RateKbit: 1000,
	})

	if want := "50ms ±5ms 1.5% loss 1.0Mbit"; got != want {
		t.Errorf("Netem() = %q, want %q", got, want)
	}
}

func TestHomePath(t *testing.T) {
	tests := []struct{ path, home, want string }{
		{"/home/u/labs/a.clab.yml", "/home/u", "~/labs/a.clab.yml"},
		{"/home/u", "/home/u", "~"},
		{"/home/user2/a", "/home/u", "/home/user2/a"},
		{"/srv/labs/a", "/home/u", "/srv/labs/a"},
		{"/srv/labs/a", "", "/srv/labs/a"},
	}

	for _, tt := range tests {
		if got := HomePath(tt.path, tt.home); got != tt.want {
			t.Errorf("HomePath(%q, %q) = %q, want %q", tt.path, tt.home, got, tt.want)
		}
	}
}

func TestSparkline(t *testing.T) {
	if got, want := Sparkline([]float64{0, 50, 100}, 3, 100), "▁▅█"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Missing history stays blank on the left, the newest value on the right.
	if got, want := Sparkline([]float64{100}, 4, 100), "   █"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Only the most recent values are shown.
	if got, want := Sparkline([]float64{100, 100, 0, 0}, 2, 100), "▁▁"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Without a limit the bars scale to the peak.
	if got, want := Sparkline([]float64{1, 2}, 2, 0), "▅█"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBarIsAlwaysAsWideAsAsked(t *testing.T) {
	for _, fraction := range []float64{-1, 0, 0.01, 0.33, 0.5, 0.99, 1, 7} {
		for width := 1; width <= 12; width++ {
			if w := Width(Bar(fraction, width, '░')); w != width {
				t.Errorf("Bar(%v, %d) is %d cells wide", fraction, width, w)
			}
		}
	}

	if got, want := Bar(0.5, 4, '░'), "██░░"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFuzzyFindRanksAndReportsRunePositions(t *testing.T) {
	items := []string{"/bin/bash", "ssh", "/bin/ash", "/bin/sh"}

	// An item containing the query as typed beats one that only matches loosely.
	matches := FuzzyFind("bash", items)
	if len(matches) != 2 || matches[0].Index != 0 || matches[1].Index != 2 {
		t.Fatalf("expected /bin/bash before /bin/ash, got %+v", matches)
	}

	if got := matches[0].Runes; len(got) != 4 || got[0] != 5 {
		t.Errorf("the typed text should be the highlighted part, got positions %v", got)
	}

	// Among items containing the query the earlier occurrence wins.
	if got := FuzzyFind("sh", items); got[0].Index != 1 {
		t.Errorf("expected ssh first for %q, got %+v", "sh", got)
	}

	if got := FuzzyFind("", items); len(got) != len(items) || got[2].Index != 2 {
		t.Errorf("an empty query should keep every item in order, got %+v", got)
	}

	// Positions count runes: the byte offset of the match in "köln" is one further.
	matches = FuzzyFind("ln", []string{"köln"})
	if len(matches) != 1 || len(matches[0].Runes) != 2 || matches[0].Runes[0] != 2 {
		t.Errorf("unexpected positions for a non-ASCII match: %+v", matches)
	}
}

func TestHighlightRunes(t *testing.T) {
	match := lipgloss.NewStyle().Bold(true)
	got := HighlightRunes("köln", []int{2, 3, 9}, lipgloss.NewStyle(), match)

	if ansi.Strip(got) != "köln" {
		t.Errorf("highlighting changed the text: %q", ansi.Strip(got))
	}

	if !strings.Contains(got, match.Render("ln")) {
		t.Errorf("the matched runes are not highlighted together: %q", got)
	}
}

func TestFieldsFillTheirBlock(t *testing.T) {
	th := clabinternaltuitheme.Default(true)
	fields := []Field{
		{Label: "image", Value: "ghcr.io/nokia/srlinux:25.10.1"},
		{Label: "kind", Value: "nokia_srlinux"},
		{Label: "status", Value: ""},
		{Label: "ipv4", Value: "172.20.20.11/24"},
	}

	for width := 20; width <= 140; width += 9 {
		block := Fields(th, fields, width, 3)
		lines := strings.Split(block, "\n")

		if len(lines) != 3 {
			t.Fatalf("width %d: got %d lines, want 3", width, len(lines))
		}

		for i, line := range lines {
			if w := Width(line); w != width {
				t.Errorf("width %d: line %d is %d cells wide", width, i, w)
			}
		}
	}

	if !strings.Contains(ansi.Strip(Fields(th, fields, 80, 3)), "status "+Dash) {
		t.Error("an empty value should be shown as a dash")
	}
}

func TestFrameIsRectangular(t *testing.T) {
	plain := lipgloss.NewStyle()
	frame := Frame("Destroy dc1", "first line\nsecond, longer line", 40, plain, plain)
	lines := strings.Split(frame, "\n")

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	for i, line := range lines {
		if w := Width(line); w != Width(lines[0]) {
			t.Errorf("line %d is %d cells wide, the first one %d", i, w, Width(lines[0]))
		}
	}

	if !strings.Contains(lines[0], "Destroy dc1") {
		t.Errorf("the title is missing from the top edge: %q", lines[0])
	}

	// Content wider than the frame may be is cut.
	narrow := Frame("t", strings.Repeat("x", 80), 20, plain, plain)
	for line := range strings.SplitSeq(narrow, "\n") {
		if w := Width(line); w > 20 {
			t.Errorf("a line of the frame is %d cells wide, the limit is 20", w)
		}
	}
}
