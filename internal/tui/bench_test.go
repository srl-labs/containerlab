// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
)

// benchModel returns a UI of a large window on a lab of 644 nodes, on the given view.
func benchModel(b *testing.B, keys ...string) (*Model, *clabinternaltuibackendfake.Backend) {
	b.Helper()

	be := clabinternaltuibackendfake.New(clabinternaltuibackendfake.Options{
		Now: fixedClock, Seed: 1, FabricLeaves: 128,
	})
	m := New(be, Options{Now: fixedClock, Lab: "fabric"})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	press(m, keys...)

	return m, be
}

// The benchmarks below measure what the user waits for: drawing a screen, moving the
// cursor, and taking in an update of every traffic rate, on a lab of 644 nodes and
// about 2,300 interfaces.

func BenchmarkDrawNodes(b *testing.B) {
	m, _ := benchModel(b, "2")

	for b.Loop() {
		m.render()
	}
}

func BenchmarkDrawInterfaces(b *testing.B) {
	m, _ := benchModel(b, "3")

	for b.Loop() {
		m.render()
	}
}

func BenchmarkDrawMap(b *testing.B) {
	m, _ := benchModel(b, "5")

	for b.Loop() {
		m.render()
	}
}

func BenchmarkCursorDownNodes(b *testing.B) {
	m, _ := benchModel(b, "2")
	down, up := keyMsg("down"), keyMsg("up")

	for b.Loop() {
		m.Update(down)
		m.render()
		m.Update(up)
		m.render()
	}
}

func BenchmarkFilterNodes(b *testing.B) {
	m, _ := benchModel(b, "2")
	table := m.top().table()

	for b.Loop() {
		table.SetFilter("leaf12")
		table.SetFilter("")
	}
}

func BenchmarkUpdateNodes(b *testing.B) {
	m, be := benchModel(b, "2")

	for b.Loop() {
		b.StopTimer()
		be.Tick()
		b.StartTimer()

		m.Update(flushMsg{})
		m.render()
	}
}

func BenchmarkUpdateInterfaces(b *testing.B) {
	m, be := benchModel(b, "3")

	for b.Loop() {
		b.StopTimer()
		be.Tick()
		b.StartTimer()

		m.Update(flushMsg{})
		m.render()
	}
}

func BenchmarkUpdateMap(b *testing.B) {
	m, be := benchModel(b, "5")

	for b.Loop() {
		b.StopTimer()
		be.Tick()
		b.StartTimer()

		m.Update(flushMsg{})
		m.render()
	}
}

func BenchmarkSwitchView(b *testing.B) {
	m, _ := benchModel(b, "2")

	for b.Loop() {
		press(m, "3")
		m.render()
		press(m, "2")
		m.render()
	}
}
