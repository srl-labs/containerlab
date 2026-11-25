// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import "strings"

// Marks tell a pixel layer that writes the text itself, in a font whose letters differ in
// width, how a piece of text sits in the cells it takes: text that is centered or flush
// right in its cells stays so. They are control sequences no terminal acts on, and have
// no width.
const (
	markEnd    = "\x1b[0z"
	markRight  = "\x1b[1z"
	markCenter = "\x1b[2z"
	markFlow   = "\x1b[3z"
)

// MarkRight marks text as flush right in the cells it takes.
func MarkRight(s string) string { return markRight + s + markEnd }

// MarkCenter marks text as centered in the cells it takes.
func MarkCenter(s string) string { return markCenter + s + markEnd }

// MarkFlow marks text as one piece that starts where its first cell does.
func MarkFlow(s string) string { return markFlow + s + markEnd }

var markStripper = strings.NewReplacer(markEnd, "", markRight, "", markCenter, "", markFlow, "")

// StripMarks removes the marks from text that goes to the terminal as it is.
func StripMarks(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}

	return markStripper.Replace(s)
}
