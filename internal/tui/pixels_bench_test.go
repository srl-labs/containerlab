// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"testing"
)

// benchPixelModel returns a UI of a large window drawn with the pixel layer, on the list
// of nodes.
func benchPixelModel(b *testing.B) (*Model, *fakeTerminal) {
	b.Helper()

	t := &testing.T{}
	m, term := newPixelModel(t, 200, 50)
	press(m, "enter")
	m.render()

	return m, term
}

// BenchmarkPixelFirstFrame draws a whole window from nothing.
func BenchmarkPixelFirstFrame(b *testing.B) {
	m, term := benchPixelModel(b)

	for b.Loop() {
		m.px.screen.Forget()
		m.render()
	}

	b.ReportMetric(float64(term.Len())/float64(b.N), "bytes/frame")
}

// BenchmarkPixelCursorMove moves the cursor of the table by a row, which changes two
// strips.
func BenchmarkPixelCursorMove(b *testing.B) {
	m, term := benchPixelModel(b)
	term.Reset()

	keys := []string{"down", "up"}
	i := 0

	for b.Loop() {
		press(m, keys[i%len(keys)])
		m.render()

		i++
	}
}

// BenchmarkPixelIdleFrame draws a frame in which nothing changed.
func BenchmarkPixelIdleFrame(b *testing.B) {
	m, _ := benchPixelModel(b)

	for b.Loop() {
		m.render()
	}
}
