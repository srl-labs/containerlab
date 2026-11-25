// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package ui holds the building blocks the TUI screens are made of: text and number
// formatting, the table, dialogs and the other widgets.
package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
)

// Ellipsis marks text that was cut short.
const Ellipsis = "…"

// Width returns the number of terminal cells the string takes, ignoring escape sequences.
func Width(s string) int {
	// Most of what a table holds is plain ASCII, one cell per byte. Telling that apart
	// is far cheaper than measuring the text grapheme by grapheme.
	for i := range len(s) {
		if c := s[i]; c < ' ' || c >= utf8.RuneSelf {
			return ansi.StringWidth(s)
		}
	}

	return len(s)
}

// Truncate shortens s to at most width cells, marking the cut with an ellipsis.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}

	if Width(s) <= width {
		return s
	}

	return ansi.Truncate(s, width, Ellipsis)
}

// TruncateLeft shortens s to at most width cells by dropping its beginning, which keeps
// the more telling end of values such as file paths.
func TruncateLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}

	w := ansi.StringWidth(s)
	if w <= width {
		return s
	}

	return ansi.TruncateLeft(s, w-width+1, Ellipsis)
}

// PadRight fits s into exactly width cells, truncating it or adding spaces at its end.
func PadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}

	w := Width(s)
	if w > width {
		return Truncate(s, width)
	}

	return s + strings.Repeat(" ", width-w)
}

// PadLeft fits s into exactly width cells, truncating it or adding spaces in front of it.
func PadLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}

	w := Width(s)
	if w > width {
		return Truncate(s, width)
	}

	return strings.Repeat(" ", width-w) + s
}

// Center places s in the middle of width cells.
func Center(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return Truncate(s, width)
	}

	left := (width - w) / 2 //nolint:mnd // half of the free space on each side

	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// Sanitize makes text from outside sources, such as container output and labels, safe to
// show: see clabinternaltuitermtext.Sanitize, which the backend uses as well.
func Sanitize(s string) string { return clabinternaltuitermtext.Sanitize(s) }

// FitLines returns exactly height lines of exactly width cells made from s, cutting or
// padding as needed. Screens use it to hand back a block of the size they were given.
func FitLines(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	out := make([]string, 0, height)

	for _, line := range lines {
		w := ansi.StringWidth(line)

		switch {
		case w > width:
			line = ansi.Truncate(line, width, "")
		case w < width:
			line += strings.Repeat(" ", width-w)
		}

		out = append(out, line)
	}

	blank := strings.Repeat(" ", width)
	for len(out) < height {
		out = append(out, blank)
	}

	return strings.Join(out, "\n")
}

// NaturalLess compares two strings the way people order names: see
// clabinternaltuitermtext.NaturalCompare, which the backend orders by as well.
func NaturalLess(a, b string) bool { return clabinternaltuitermtext.NaturalLess(a, b) }
