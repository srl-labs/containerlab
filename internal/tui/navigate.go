// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"sort"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// siblingPage is implemented by pages that can move on to the item next to the one they
// show without going back to the list first, like the next node.
type siblingPage interface {
	sibling(m *Model, delta int) tea.Cmd
}

// sibling moves to the previous or next item at the current level. A page about one node
// moves on to the node next to it; everywhere else the views move on to the next lab.
func (m *Model) sibling(delta int) tea.Cmd {
	if s, ok := m.top().(siblingPage); ok {
		return s.sibling(m, delta)
	}

	labs := m.deployedLabs()
	if len(labs) == 0 {
		return nil
	}

	// From all labs the first step lands on the first, or the last, lab.
	next := 0
	if delta < 0 {
		next = len(labs) - 1
	}

	for i, name := range labs {
		if name == m.scope {
			next = (i + delta + len(labs)) % len(labs)
		}
	}

	if m.top().tab() == tabLabs {
		// The list of labs covers them all: stepping from there shows the nodes.
		m.scope = labs[next]
		m.goTo(tabNodes)

		return nil
	}

	m.setScope(labs[next])

	return nil
}

// deployedLabs returns the names of the deployed labs in the order the lab list has them.
func (m *Model) deployedLabs() []string {
	labs := make([]*clabinternaltuibackend.Lab, 0, len(m.labs))

	for i := range m.labs {
		if m.labs[i].Deployed {
			labs = append(labs, &m.labs[i])
		}
	}

	sort.SliceStable(labs, func(a, b int) bool {
		if ra, rb := stateRank(labs[a].State()), stateRank(labs[b].State()); ra != rb {
			return ra < rb
		}

		return clabinternaltuiui.NaturalLess(labs[a].Name, labs[b].Name)
	})

	names := make([]string, len(labs))
	for i, lab := range labs {
		names[i] = lab.Name
	}

	return names
}

// neighborNode returns the node next to the given one in its lab, wrapping around.
func (m *Model) neighborNode(
	ref clabinternaltuibackend.NodeRef,
	delta int,
) (clabinternaltuibackend.NodeRef, bool) {
	lab := m.lab(ref.Lab)
	if lab == nil || len(lab.Nodes) == 0 {
		return ref, false
	}

	for i := range lab.Nodes {
		if lab.Nodes[i].Name == ref.Name {
			next := (i + delta + len(lab.Nodes)) % len(lab.Nodes)

			return lab.Nodes[next].NodeRef, next != i
		}
	}

	return lab.Nodes[0].NodeRef, true
}

// openNode shows a node: the views are limited to its lab, the node list has its cursor
// on it, and its page is opened on top. Going back therefore lands where one expects.
func (m *Model) openNode(ref clabinternaltuibackend.NodeRef) {
	m.scope = ref.Lab
	m.goTo(tabNodes)

	if nodes, ok := m.top().(*nodesPage); ok {
		nodes.tbl.SetFilter("")
		nodes.tbl.SetCursorKey(ref.Container)
	}

	m.push(newNodePage(m, ref))
}

// openLab shows the nodes of a lab, or its topology when it is not deployed.
func (m *Model) openLab(lab *clabinternaltuibackend.Lab) {
	if !lab.Deployed {
		m.showMap(lab.Name)

		return
	}

	m.scope = lab.Name
	m.goTo(tabNodes)

	// A filter left from another lab would hide the nodes of this one.
	if t := m.top().table(); t != nil {
		t.SetFilter("")
	}
}

// goToMenu lets the user jump anywhere by typing a few letters of its name: to a view, a
// lab or a node. It is the shortest way to a node whose name is known.
func (m *Model) goToMenu() tea.Cmd {
	var items []menuItem

	for i := range m.labs {
		lab := &m.labs[i]

		for n := range lab.Nodes {
			node := &lab.Nodes[n]
			items = append(items, menuItem{
				label: node.Name,
				desc:  lab.Name + " · " + node.Kind + " · " + node.Readiness().String(),
				run: func() tea.Cmd {
					m.openNode(node.NodeRef)

					return nil
				},
			})
		}
	}

	for i := range m.labs {
		lab := &m.labs[i]
		items = append(items, menuItem{
			label: lab.Name,
			desc:  "lab · " + lab.State().String(),
			run: func() tea.Cmd {
				m.openLab(lab)

				return nil
			},
		})
	}

	for _, t := range tabs {
		items = append(items, menuItem{
			label: t.command,
			desc:  "view",
			run: func() tea.Cmd {
				m.goTo(t.id)

				return nil
			},
		})
	}

	return m.choose("Go to", items)
}

// goToName jumps to the lab or node with exactly the given name and reports whether there
// is one. A node name used in several labs is looked for in the current lab first.
func (m *Model) goToName(name string) bool {
	if lab := m.lab(name); lab != nil {
		m.openLab(lab)

		return true
	}

	var found *clabinternaltuibackend.Node

	for i := range m.labs {
		node := m.labs[i].Node(name)
		if node == nil {
			continue
		}

		if found == nil || m.labs[i].Name == m.scope {
			found = node
		}
	}

	if found == nil {
		return false
	}

	m.openNode(found.NodeRef)

	return true
}

// unknownCommand tells the user that a command does not exist.
func (m *Model) unknownCommand(word string) {
	m.notifyf(clabinternaltuitheme.LevelWarning,
		"%q is neither a command nor the name of a lab or node, see :help", word)
}
