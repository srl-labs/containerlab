// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package ui holds the building blocks the TUI screens are made of: text and number
// formatting, the table, dialogs and the other widgets.
package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
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
// show: escape sequences are removed and control characters replaced, so that it cannot
// move the cursor or change the terminal's state.
func Sanitize(s string) string {
	if !needsSanitizing(s) {
		return s
	}

	s = ansi.Strip(s)

	var b strings.Builder

	b.Grow(len(s))

	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case unicode.IsControl(r):
			// Dropped: a control character has no place in a single line of text.
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func needsSanitizing(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}

	return false
}

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

// NaturalLess compares two strings the way people order names: runs of digits compare by
// their numeric value, so that leaf2 sorts before leaf10. The comparison ignores case.
func NaturalLess(a, b string) bool {
	return naturalCompare(a, b) < 0
}

func naturalCompare(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0

	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			ni, nexti := digitRun(ra, i)
			nj, nextj := digitRun(rb, j)

			if c := compareDigits(ni, nj); c != 0 {
				return c
			}

			i, j = nexti, nextj

			continue
		}

		if ra[i] != rb[j] {
			if ra[i] < rb[j] {
				return -1
			}

			return 1
		}

		i++
		j++
	}

	return (len(ra) - i) - (len(rb) - j)
}

// digitRun returns the run of digits starting at i without its leading zeros, and the
// index following the run.
func digitRun(r []rune, i int) (digits []rune, next int) {
	next = i
	for next < len(r) && unicode.IsDigit(r[next]) {
		next++
	}

	digits = r[i:next]
	for len(digits) > 1 && digits[0] == '0' {
		digits = digits[1:]
	}

	return digits, next
}

func compareDigits(a, b []rune) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}

	for k := range a {
		if a[k] != b[k] {
			return int(a[k]) - int(b[k])
		}
	}

	return 0
}
