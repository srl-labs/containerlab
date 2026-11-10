// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package picker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func testNodes() []Node {
	return []Node{
		{
			Name: "clab-lab1-srl1", Kind: "nokia_srlinux", Lab: "lab1",
			Image: "nokia/srlinux:25.10", State: "running",
			IPv4: "172.20.20.2/24", IPv6: "3fff:172:20:20::2/64",
		},
		{
			Name: "clab-lab1-client1", Kind: "linux", Lab: "lab1",
			Image: "srl-labs/network-multitool:latest", State: "running",
		},
		{
			Name: "clab-lab1-client2", Kind: "linux", Lab: "lab1",
			Image: "srl-labs/network-multitool:latest", State: "exited",
		},
	}
}

func testMethods() []clabinternaltuibackend.ConnectMethod {
	return clabinternaltuibackend.DefaultConnectMethods([]string{"/bin/sh"}, "linux")
}

func typed(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func pressed(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func TestNodePickerRenders(t *testing.T) {
	m := newNodeModel(testNodes())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	view := ansi.Strip(m.render())

	for _, want := range []string{
		"clab-lab1-srl1", "clab-lab1-client1", "nokia_srlinux", "node details",
		"172.20.20.2/24", "esc quit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the view does not show %q:\n%s", want, view)
		}
	}
}

// TestPickersTreatLettersAsQuery types letters that move the cursor in other lists: in a
// picker they belong to the query.
func TestPickersTreatLettersAsQuery(t *testing.T) {
	node := newNodeModel(testNodes())
	node.Update(typed('j'))
	node.Update(typed('k'))

	if got := node.input.Value(); got != "jk" {
		t.Errorf("the node picker query is %q, want %q", got, "jk")
	}

	method := newMethodModel(nil, testMethods())
	method.Update(typed('j'))

	if got := method.input.Value(); got != "j" {
		t.Errorf("the method picker query is %q, want %q", got, "j")
	}
}

// TestNodePickerRowsDoNotWrap renders the node picker at several widths: a row must stay
// on one line, whatever has to give.
func TestNodePickerRowsDoNotWrap(t *testing.T) {
	nodes := []Node{
		{
			Name: "clab-a-rather-long-lab-name-client1", Kind: "linux",
			Image: "srl-labs/network-multitool:latest", IPv6: "3fff:172:20:20::8/64",
		},
		{
			Name: "clab-a-rather-long-lab-name-srl1", Kind: "nokia_srlinux",
			Image: "nokia/srlinux:25.10",
		},
	}

	for _, width := range []int{40, 60, 80, 100, 120, 140, 200} {
		m := newNodeModel(nodes)
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})

		if got := m.layout().width(); got > width {
			t.Errorf("width %d: the list is %d wide", width, got)
		}

		view := m.render()
		if got := lipgloss.Width(view); got > width {
			t.Errorf("width %d: the view is %d wide:\n%s", width, got, ansi.Strip(view))
		}

		for _, node := range nodes {
			if !sharesLine(ansi.Strip(view), node.Name[:12], node.Kind) {
				t.Errorf(
					"width %d: the row of %s is wrapped:\n%s",
					width,
					node.Kind,
					ansi.Strip(view),
				)
			}
		}
	}
}

// sharesLine reports whether both texts appear on one line of the view.
func sharesLine(view, a, b string) bool {
	for line := range strings.Lines(view) {
		if strings.Contains(line, a) && strings.Contains(line, b) {
			return true
		}
	}

	return false
}

func TestPickersClearTheScreenOnceDone(t *testing.T) {
	node := newNodeModel(testNodes())
	node.Update(pressed(tea.KeyEnter))

	if node.chosen != 0 {
		t.Errorf("enter chose %d, want the first node", node.chosen)
	}

	if got := node.render(); got != "" {
		t.Errorf("the node picker left %q on the screen", got)
	}

	method := newMethodModel(nil, testMethods())
	method.Update(pressed(tea.KeyEscape))

	if !method.cancelled {
		t.Error("escape did not cancel the method picker")
	}

	if got := method.render(); got != "" {
		t.Errorf("the method picker left %q on the screen", got)
	}
}

func TestNodeDetailWrapsLongPortLists(t *testing.T) {
	m := newNodeModel(testNodes())

	short := Node{Name: "clab-lab1-client1"}
	long := short
	long.Ports = "22/tcp, 80/tcp, 443/tcp, 1180/tcp, 8080/tcp, 11443/tcp"

	detail := m.nodeDetail(&long)
	limit := lipgloss.Width(m.nodeDetail(&short)) + portsWidth

	if got := lipgloss.Width(detail); got > limit {
		t.Errorf("the details panel is %d wide, want at most %d:\n%s", got, limit, detail)
	}

	for _, port := range []string{"22/tcp", "11443/tcp"} {
		if !strings.Contains(ansi.Strip(detail), port) {
			t.Errorf("port %q is missing:\n%s", port, ansi.Strip(detail))
		}
	}
}

func TestPickersResetCursorOnNewQuery(t *testing.T) {
	m := newNodeModel(testNodes())
	m.Update(pressed(tea.KeyDown))
	m.Update(pressed(tea.KeyDown))

	if m.cursor != 2 {
		t.Fatalf("the cursor is at %d, want 2", m.cursor)
	}

	// Messages that leave the query alone, ie. the blinking of the input's cursor, keep
	// the position.
	m.Update(struct{}{})

	if m.cursor != 2 {
		t.Errorf("the cursor moved to %d without a new query", m.cursor)
	}

	m.Update(typed('c'))

	if m.cursor != 0 {
		t.Errorf("a new query left the cursor at %d, want it on the best match", m.cursor)
	}
}

func TestNodePickerFiltersAndChoosesTheOriginalIndex(t *testing.T) {
	m := newNodeModel(testNodes())

	for _, r := range "client2" {
		m.Update(typed(r))
	}

	if len(m.matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(m.matches))
	}

	m.Update(pressed(tea.KeyEnter))

	if m.chosen != 2 {
		t.Errorf("chose index %d, want 2: the position in the list that was given", m.chosen)
	}
}

func TestMethodPickerShowsTarget(t *testing.T) {
	target := testNodes()[0]
	view := ansi.Strip(newMethodModel(&target, testMethods()).render())

	for _, want := range []string{
		"/bin/sh", "ssh", "/bin/bash", "target: clab-lab1-srl1 (nokia_srlinux)",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the view does not show %q:\n%s", want, view)
		}
	}
}

// TestPickerFollowsTheHeightOfTheTerminal checks that a short terminal gets a shorter
// list rather than a picker that scrolls off the screen.
func TestPickerFollowsTheHeightOfTheTerminal(t *testing.T) {
	nodes := make([]Node, 0, 30)
	for i := range 30 {
		nodes = append(nodes, Node{Name: "clab-lab1-node" + string(rune('a'+i%26)), Kind: "linux"})
	}

	m := newNodeModel(nodes)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if got := m.visible(); got != maxRows {
		t.Errorf("a tall terminal shows %d rows, want %d", got, maxRows)
	}

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})

	if got := m.visible(); got != 10-chromeRows {
		t.Errorf("a short terminal shows %d rows, want %d", got, 10-chromeRows)
	}

	if got := lipgloss.Height(m.render()); got > 10 {
		t.Errorf(
			"the picker is %d lines high in a terminal of 10:\n%s",
			got,
			ansi.Strip(m.render()),
		)
	}
}
