// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

// The tests in this file walk through what users come to do, key by key, starting from
// the screen the UI opens with. They pin down the keyboard model: enter and the right
// arrow go in, escape and the left arrow go out, the brackets move sideways, and a menu
// entry is one key away.

// journey returns a UI on its first screen, the list of labs, with the welcome put away.
func journey(t *testing.T) (*Model, func() []string) {
	t.Helper()

	m, be := newTestModel(t, 120, 35)

	return m, be.Sessions
}

func nodeOnPage(t *testing.T, m *Model) string {
	t.Helper()

	page, ok := m.top().(*nodePage)
	if !ok {
		t.Fatalf("expected the page of a node, got %T", m.top())
	}

	return page.ref.Lab + "/" + page.ref.Name
}

// TestBeginnerPath uses nothing but the arrows, enter, escape and the action menu: that
// must be enough to get a shell on a node.
func TestBeginnerPath(t *testing.T) {
	m, sessions := journey(t)

	if !strings.Contains(ansi.Strip(m.render()), "a what can I do here") {
		t.Error("the first screen does not tell the basics")
	}

	// Into the lab under the cursor, down to the second node, into it.
	press(m, "enter", "down", "enter")

	if got := nodeOnPage(t, m); got != "dc1/spine2" {
		t.Fatalf("the arrows and enter led to %s", got)
	}

	if strings.Contains(ansi.Strip(m.render()), "what can I do here") {
		t.Error("the welcome line stayed after the first key presses")
	}

	// The action menu lists what can be done; enter on its first entry opens a shell.
	press(m, "a")

	if len(m.overlays) != 1 {
		t.Fatal("a did not open the action menu")
	}

	screen := ansi.Strip(m.render())
	for _, want := range []string{"shell", "open the command line of the node", "logs", "restart"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the action menu does not offer %q", want)
		}
	}

	press(m, "enter")

	if got := sessions(); len(got) != 1 || !strings.Contains(got[0], "clab-dc1-spine2") {
		t.Errorf("enter on the first action opened %v, want a shell on spine2", got)
	}

	// Escape leads all the way back out, one level at a time.
	press(m, "esc")

	if _, ok := m.top().(*nodesPage); !ok {
		t.Fatalf("escape from a node shows %T", m.top())
	}

	press(m, "esc")

	if _, ok := m.top().(*labsPage); !ok || m.scope != "" {
		t.Errorf("escape from the nodes shows %T with the scope %q", m.top(), m.scope)
	}
}

// TestShellInFourKeys reaches a node by name with the finder and opens a shell.
func TestShellInFourKeys(t *testing.T) {
	m, sessions := journey(t)

	press(m, "ctrl+k")
	typeText(m, "leaf3")
	press(m, "enter", "s")

	if got := sessions(); len(got) != 1 || !strings.Contains(got[0], "clab-dc1-leaf3") {
		t.Fatalf("the finder and s opened %v, want a shell on leaf3", got)
	}

	if !strings.Contains(sessions()[0], "sr_cli") {
		t.Errorf("s should use the shell of the node's kind, got %q", sessions()[0])
	}

	// Going back from a node reached through the finder lands on the node list of
	// its lab, with the cursor on the node.
	press(m, "esc")

	nodes, ok := m.top().(*nodesPage)
	if !ok || m.scope != "dc1" || nodes.current() == nil || nodes.current().Name != "leaf3" {
		t.Errorf("going back shows %T scoped to %q", m.top(), m.scope)
	}
}

func TestCommandLineTakesNames(t *testing.T) {
	m, _ := journey(t)

	press(m, ":")
	typeText(m, "pe2")
	press(m, "enter")

	if got := nodeOnPage(t, m); got != "sp-core/pe2" {
		t.Errorf(":pe2 led to %s", got)
	}

	press(m, ":")
	typeText(m, "metro")
	press(m, "enter")

	if _, ok := m.top().(*nodesPage); !ok || m.scope != "metro" {
		t.Errorf(":metro shows %T scoped to %q", m.top(), m.scope)
	}

	press(m, ":")
	typeText(m, "nothing-like-this")
	press(m, "enter")

	if !strings.Contains(m.toast.text, "neither a command nor the name") {
		t.Errorf("an unknown name is not reported: %q", m.toast.text)
	}
}

// TestArrowsWalkTheHierarchy checks that right and left do what enter and escape do.
func TestArrowsWalkTheHierarchy(t *testing.T) {
	m, _ := journey(t)

	press(m, "right")

	if _, ok := m.top().(*nodesPage); !ok || m.scope != "dc1" {
		t.Fatalf("right on a lab shows %T scoped to %q", m.top(), m.scope)
	}

	press(m, "right")

	if got := nodeOnPage(t, m); got != "dc1/spine1" {
		t.Fatalf("right on a node led to %s", got)
	}

	press(m, "left")

	if _, ok := m.top().(*nodesPage); !ok {
		t.Fatalf("left from a node shows %T", m.top())
	}

	press(m, "left")

	if _, ok := m.top().(*labsPage); !ok || m.scope != "" {
		t.Errorf("left from the nodes shows %T scoped to %q", m.top(), m.scope)
	}

	// The lab list finds its cursor where it was left.
	if lab := m.top().(*labsPage).current(m); lab == nil || lab.Name != "dc1" {
		t.Errorf("the cursor of the lab list moved to %+v", lab)
	}
}

// TestBracketsMoveSideways steps through labs and nodes without going back to a list.
func TestBracketsMoveSideways(t *testing.T) {
	m, _ := journey(t)
	labs := m.deployedLabs()

	press(m, "enter")

	at := slices.Index(labs, "dc1")

	press(m, "]")

	if want := labs[(at+1)%len(labs)]; m.scope != want {
		t.Errorf("] moved to lab %q, want %q", m.scope, want)
	}

	press(m, "[")

	if m.scope != "dc1" {
		t.Errorf("[ moved to lab %q, want dc1 again", m.scope)
	}

	// With a node open the brackets step through the nodes of the lab.
	press(m, "enter", "]")

	if got := nodeOnPage(t, m); got != "dc1/spine2" {
		t.Errorf("] from spine1 led to %s", got)
	}

	press(m, "[", "[")

	if got := nodeOnPage(t, m); got != "dc1/client4" {
		t.Errorf("[ wraps around to %s, want the last node", got)
	}
}

// TestViewsKeepTheirPlace switches views and comes back: the cursor and the filter are
// where they were left.
func TestViewsKeepTheirPlace(t *testing.T) {
	m, _ := journey(t)

	press(m, "enter", "/")
	typeText(m, "leaf")
	press(m, "enter", "down", "down")

	before := m.top().(*nodesPage).current().Name

	press(m, "4", "6", "2")

	nodes, ok := m.top().(*nodesPage)
	if !ok || nodes.tbl.Filter() != "leaf" || nodes.current().Name != before {
		t.Errorf("the node list did not keep its place: filter %q", nodes.tbl.Filter())
	}
}

// TestMenusAreOneKeyAway copies the address of a node and connects over SSH, each with
// the key that opens the menu and the key of the entry.
func TestMenusAreOneKeyAway(t *testing.T) {
	m, sessions := journey(t)
	press(m, "enter", "y")

	screen := ansi.Strip(m.render())
	if !strings.Contains(screen, "IPv4 address") || !strings.Contains(screen, "172.20.20.11") {
		t.Fatalf("the copy menu does not show the values:\n%s", screen)
	}

	_, cmd := m.Update(keyMsg("4"))

	if cmd == nil || len(m.overlays) != 0 {
		t.Error("4 did not copy the address and close the menu")
	}

	if !strings.Contains(m.toast.text, "copied IPv4 address: 172.20.20.11") {
		t.Errorf("the copy is not confirmed: %q", m.toast.text)
	}

	press(m, "c", "2")

	if got := sessions(); len(got) != 1 || got[0] != "ssh clab-dc1-spine1" {
		t.Errorf("c then 2 opened %v, want ssh", got)
	}

	// A menu can still be searched, and escape leaves the search before the menu.
	press(m, "a", "/")
	typeText(m, "rest")

	if !strings.Contains(ansi.Strip(m.render()), "restart") {
		t.Error("the search of the action menu does not find restart")
	}

	press(m, "esc")

	if len(m.overlays) != 1 {
		t.Fatal("escape closed the menu instead of leaving the search")
	}

	press(m, "esc")

	if len(m.overlays) != 0 {
		t.Error("the second escape did not close the menu")
	}
}

// TestDestroyInTwoKeys destroys the lab under the cursor: X and the answer.
func TestDestroyInTwoKeys(t *testing.T) {
	m, be := newTestModel(t, 120, 35)

	press(m, "X", "n")

	if len(m.overlays) != 0 || len(be.Jobs()) != 0 {
		t.Fatal("n did not cancel the destroy")
	}

	press(m, "X", "y")
	advance(m, be, 10)

	if lab := m.lab("dc1"); lab == nil || lab.Deployed {
		t.Error("X and y did not destroy the lab")
	}
}

// TestImpairALink sets a delay on a link from the node it belongs to.
func TestImpairALink(t *testing.T) {
	m, be := newTestModel(t, 120, 35)

	press(m, "ctrl+k")
	typeText(m, "leaf2")
	press(m, "enter", "down", "n")

	if len(m.overlays) != 1 {
		t.Fatal("n did not open the impairment form")
	}

	typeText(m, "20ms")
	press(m, "enter")
	advance(m, be, 3)

	leaf2 := clabinternaltuibackend.NodeRef{Lab: "dc1", Name: "leaf2", Container: "clab-dc1-leaf2"}

	for _, iface := range be.Interfaces(leaf2) {
		if iface.Name == "e1-49" {
			if iface.Netem == nil || iface.Netem.Delay.String() != "20ms" {
				t.Errorf("the delay was not set: %+v", iface.Netem)
			}

			return
		}
	}

	t.Error("the interface was not found")
}

// TestImpairSeveralLinks marks interfaces of a node and sets a delay on all of them. A lab
// runs one job at a time, so they are worked on by a single job, and so is clearing them.
func TestImpairSeveralLinks(t *testing.T) {
	m, be := newTestModel(t, 120, 35)

	press(m, "ctrl+k")
	typeText(m, "leaf2")
	press(m, "enter", "down", "space", "space")

	page, ok := m.top().(*nodePage)
	if !ok {
		t.Fatalf("expected the page of a node, got %T", m.top())
	}

	marked := map[string]bool{}
	for _, iface := range page.selection() {
		marked[iface.Name] = true
	}

	if len(marked) != 2 {
		t.Fatalf("%d interfaces are marked, want 2", len(marked))
	}

	leaf2 := clabinternaltuibackend.NodeRef{Lab: "dc1", Name: "leaf2", Container: "clab-dc1-leaf2"}

	impaired := func() int {
		count := 0

		for _, iface := range be.Interfaces(leaf2) {
			if marked[iface.Name] && iface.Netem != nil && iface.Netem.Delay.String() == "20ms" {
				count++
			}
		}

		return count
	}

	press(m, "n")
	typeText(m, "20ms")
	press(m, "enter")

	jobs := be.Jobs()
	if len(jobs) != 1 || jobs[0].Lab != "dc1" || jobs[0].Title != "netem dc1 (2 interfaces)" {
		t.Fatalf("setting the delay started %+v, want one job for the lab", jobs)
	}

	if m.toast.level == clabinternaltuitheme.LevelError {
		t.Errorf("setting the delay failed: %s", m.toast.text)
	}

	advance(m, be, 3)

	if got := impaired(); got != len(marked) {
		t.Errorf("the delay was set on %d of %d interfaces", got, len(marked))
	}

	press(m, "N", "enter")

	jobs = be.Jobs()
	if len(jobs) != 2 || jobs[0].Title != "netem reset dc1 (2 interfaces)" {
		t.Fatalf("clearing the delay started %+v, want one more job for the lab", jobs)
	}

	if m.toast.level == clabinternaltuitheme.LevelError {
		t.Errorf("clearing the delay failed: %s", m.toast.text)
	}

	advance(m, be, 3)

	if got := impaired(); got != 0 {
		t.Errorf("the delay is left on %d interfaces", got)
	}
}

// TestInterfacesAreGroupedByLab pins down that interfaces of several labs make one request
// per lab, in the order the labs come in.
func TestInterfacesAreGroupedByLab(t *testing.T) {
	iface := func(lab, node, name string) *clabinternaltuibackend.Interface {
		return &clabinternaltuibackend.Interface{
			Node: clabinternaltuibackend.NodeRef{Lab: lab, Name: node}, Name: name,
		}
	}

	groups := interfacesByLab([]*clabinternaltuibackend.Interface{
		iface("dc1", "leaf1", "e1-1"),
		iface("sp-core", "pe1", "eth1"),
		iface("dc1", "leaf2", "e1-1"),
	})

	var got []string

	for _, group := range groups {
		names := make([]string, len(group))
		for i, ref := range group {
			names[i] = ref.Node.Lab + "/" + ref.Node.Name + ":" + ref.Name
		}

		got = append(got, strings.Join(names, " "))
	}

	want := []string{"dc1/leaf1:e1-1 dc1/leaf2:e1-1", "sp-core/pe1:eth1"}
	if !slices.Equal(got, want) {
		t.Errorf("groups = %q, want %q", got, want)
	}
}

// TestUnavailableActionsSayWhy presses the key of an action that does not apply.
func TestUnavailableActionsSayWhy(t *testing.T) {
	m, sessions := journey(t)

	press(m, "ctrl+k")
	typeText(m, "pe4")
	press(m, "enter", "s")

	if len(sessions()) != 0 {
		t.Fatal("a shell was opened on a node that does not run")
	}

	if !strings.Contains(m.toast.text, "pe4 is not running") {
		t.Errorf("the reason is not given: %q", m.toast.text)
	}

	// The action menu shows the entry dimmed, with the same reason.
	press(m, "a")

	if !strings.Contains(ansi.Strip(m.render()), "pe4 is not running") {
		t.Error("the action menu does not say why the shell is unavailable")
	}
}

// TestEveryHintWorks presses the key of every hint on the main screens: a key the screen
// advertises must do something, or say why not.
func TestEveryHintWorks(t *testing.T) {
	screens := map[string][]string{
		"labs": nil, "nodes": {"enter"}, "node": {"enter", "enter"},
		"interfaces": {"enter", "3"}, "links": {"enter", "4"}, "map": {"enter", "5"},
	}

	for name, keys := range screens {
		m, _ := newTestModel(t, 200, 50)
		press(m, keys...)

		for _, h := range m.top().hints(m) {
			key, ok := hintKey(h)
			if !ok || h.key == "?" || h.key == ":" || h.key == "/" || h.key == keyEsc {
				continue
			}

			probe, _ := newTestModel(t, 200, 50)
			press(probe, keys...)

			before := ansi.Strip(probe.render())
			_, cmd := probe.Update(key)

			if cmd == nil && ansi.Strip(probe.render()) == before {
				t.Errorf("%s: the hint %q (%s) does nothing", name, h.key, h.desc)
			}
		}
	}
}

func TestShiftedKeysWithoutText(t *testing.T) {
	m, _ := journey(t)

	// Some terminals report shift+x as the letter and the modifier, without the text.
	m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift})

	if len(m.overlays) != 1 {
		t.Error("shift+x reported without its text did not open the destroy dialog")
	}
}
