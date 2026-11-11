// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package termtext

import "testing"

func TestSanitize(t *testing.T) {
	tests := map[string]struct {
		in, want string
	}{
		"plain text is untouched":   {"deploying lab", "deploying lab"},
		"non ASCII text is kept":    {"link ▪┄┄▪ über", "link ▪┄┄▪ über"},
		"colors":                    {"\x1b[1;31mERRO\x1b[0m failed", "ERRO failed"},
		"cursor movement":           {"\x1b[2K\x1b[1Gprogress", "progress"},
		"private mode sequences":    {"\x1b[?25lhidden\x1b[?25h", "hidden"},
		"window title":              {"\x1b]0;evil title\x07text", "text"},
		"hyperlink":                 {"\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\", "link"},
		"device control string":     {"a\x1bP1$r0m\x1b\\b", "ab"},
		"two character sequence":    {"a\x1bcb", "ab"},
		"charset selection":         {"a\x1b(Bb", "ab"},
		"single byte csi":           {"a\u009b31mb", "ab"},
		"other c1 controls":         {"a\u0085b\u0090c", "abc"},
		"unterminated sequence":     {"text\x1b[31", "text"},
		"lone escape":               {"text\x1b", "text"},
		"control characters":        {"a\x00b\x07c\x08d\x7fe", "abcde"},
		"line break characters":     {"a\vb\fc\nd", "abcd"},
		"windows line ending":       {"line\r", "line"},
		"rewritten line":            {"10%\r50%\r100%", "100%"},
		"rewritten line with crlf":  {"10%\r100%\r\r", "100%"},
		"tab at the start":          {"\tx", "        x"},
		"tab after text":            {"ab\tx", "ab      x"},
		"tab at a tab stop":         {"12345678\tx", "12345678        x"},
		"invalid utf-8":             {"a\xffb", "a�b"},
		"cut multi byte character":  {"a\xe2\x96", "a��"},
		"escape in front of utf-8":  {"\x1b[1m▪\x1b[0m", "▪"},
		"empty":                     {"", ""},
		"only an escape sequence":   {"\x1b[0m", ""},
		"osc without a terminator":  {"a\x1b]0;never ends", "a"},
		"nested escape after bell":  {"\x1b]0;t\x07\x1b[1mb\x1b[0m", "b"},
		"csi with intermediate":     {"a\x1b[0 qb", "ab"},
		"application program cmd":   {"a\x1b_payload\x1b\\b", "ab"},
		"privacy message":           {"a\x1b^payload\x07b", "ab"},
		"start of string":           {"a\x1bXpayload\x1b\\b", "ab"},
		"sequence then carriage":    {"\x1b[1mold\rnew", "new"},
		"tabs count printed runes":  {"é\tx", "é       x"},
		"backspace does not rewind": {"ab\bc", "abc"},
		"bell and return":           {"bell\x07 and\rreturn", "return"},
		"line break":                {"line\nbreak", "linebreak"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Sanitize(tt.in); got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNaturalCompare(t *testing.T) {
	sorted := []string{
		"", "e1-1", "e1-2", "e1-10", "eth0", "eth00", "eth1", "eth9", "eth10", "leaf", "leaf1",
		"leaf1a", "leaf2", "leaf10", "leaf10b", "Leaf11", "leaf12", "mgmt0", "Spine1", "spine02",
		"spine3",
	}

	for i := range sorted {
		for j := range sorted {
			got := NaturalCompare(sorted[i], sorted[j])

			switch {
			case i < j && got >= 0, i > j && got <= 0, i == j && got != 0:
				t.Errorf("NaturalCompare(%q, %q) = %d", sorted[i], sorted[j], got)
			}
		}
	}
}

// TestNaturalCompareIgnoresCase checks that case only decides between strings that are
// the same otherwise.
func TestNaturalCompareIgnoresCase(t *testing.T) {
	if c := NaturalCompare("Lab2", "lab10"); c >= 0 {
		t.Errorf("NaturalCompare(Lab2, lab10) = %d, want < 0", c)
	}

	if c := NaturalCompare("Lab1", "lab1"); c >= 0 {
		t.Errorf("NaturalCompare(Lab1, lab1) = %d, want < 0", c)
	}

	if NaturalLess("leaf1", "leaf1") {
		t.Error("a value must not sort before itself")
	}
}
