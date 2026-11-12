// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package termtext holds the text handling the backend and the screens of the TUI share:
// making untrusted text safe to show in a terminal, and ordering names the way people do.
package termtext

import (
	"cmp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// tabWidth is the distance between tab stops when tabs are expanded.
	tabWidth = 8

	escape = 0x1b
	bell   = 0x07
	// c1CSI is the single character form of the control sequence introducer.
	c1CSI = 0x9b
)

// Sanitize makes a line of untrusted text, such as command output or a container label,
// safe to show in a terminal: escape sequences and control characters are removed, tabs
// are expanded and invalid UTF-8 is replaced. Of a line that was rewritten with carriage
// returns only the last version is kept, which is what a terminal would show.
func Sanitize(s string) string {
	s = strings.TrimRight(s, "\r")
	if !needsSanitizing(s) {
		return s
	}

	var (
		out strings.Builder
		col int
	)

	out.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])

		switch {
		case r == escape:
			i += escapeLen(s[i:])

			continue
		case r == c1CSI:
			i += size + csiLen(s[i+size:])

			continue
		case r == '\r':
			out.Reset()

			col = 0
		case r == '\t':
			pad := tabWidth - col%tabWidth
			out.WriteString(strings.Repeat(" ", pad))

			col += pad
		case unicode.IsControl(r):
		default:
			// An invalid byte decodes to the replacement character, which is what is kept.
			out.WriteRune(r)

			col++
		}

		i += size
	}

	return out.String()
}

func needsSanitizing(s string) bool {
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return true
		}
	}

	return false
}

// escapeLen returns the length of the escape sequence s starts with.
func escapeLen(s string) int {
	if len(s) < 2 { //nolint:mnd // the escape character and what it introduces
		return len(s)
	}

	switch s[1] {
	case '[':
		return 2 + csiLen(s[2:]) //nolint:mnd // the introducer is two bytes long
	case ']', 'P', 'X', '^', '_':
		return 2 + stringSeqLen(s[2:]) //nolint:mnd // the introducer is two bytes long
	}

	// A plain escape sequence: optional intermediate bytes and one final byte.
	n := 1
	for n < len(s) && s[n] >= 0x20 && s[n] <= 0x2f {
		n++
	}

	if n < len(s) {
		n++
	}

	return n
}

// csiLen returns the length of the control sequence that follows its introducer: parameter
// and intermediate bytes up to the final byte.
func csiLen(s string) int {
	n := 0
	for n < len(s) && s[n] >= 0x20 && s[n] <= 0x3f {
		n++
	}

	if n < len(s) && s[n] >= 0x40 && s[n] <= 0x7e {
		n++
	}

	return n
}

// stringSeqLen returns the length of the payload of an operating system command or a device
// control string, which lasts until a bell or a string terminator.
func stringSeqLen(s string) int {
	for n := range len(s) {
		switch {
		case s[n] == bell:
			return n + 1
		case s[n] == escape && n+1 < len(s) && s[n+1] == '\\':
			return n + 2 //nolint:mnd // the terminator is two bytes long
		}
	}

	return len(s)
}

// NaturalCompare orders strings the way people order names: the numbers in them compare by
// value, so that leaf2 sorts before leaf10, and case is ignored. Strings that only differ
// in case or in how their numbers are written, ie. by leading zeros, compare as plain
// strings, which keeps the order total.
func NaturalCompare(a, b string) int {
	return cmp.Or(compareNumbersByValue(strings.ToLower(a), strings.ToLower(b)),
		strings.Compare(a, b))
}

// NaturalLess reports whether a sorts before b in the order of NaturalCompare.
func NaturalLess(a, b string) bool { return NaturalCompare(a, b) < 0 }

// compareNumbersByValue compares the strings byte by byte, except for runs of digits, which
// compare by their value. Comparing the bytes of UTF-8 orders by code point.
func compareNumbersByValue(a, b string) int {
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			an, bn := digits(a), digits(b)
			a, b = a[len(an):], b[len(bn):]

			// Without leading zeros the longer number is the larger one.
			an, bn = strings.TrimLeft(an, "0"), strings.TrimLeft(bn, "0")
			if c := cmp.Or(cmp.Compare(len(an), len(bn)), strings.Compare(an, bn)); c != 0 {
				return c
			}

			continue
		}

		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}

		a, b = a[1:], b[1:]
	}

	return cmp.Compare(len(a), len(b))
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// digits returns the run of digits s starts with.
func digits(s string) string {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}

	return s[:n]
}
