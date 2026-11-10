// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// tabID identifies one of the top-level views.
type tabID int

const (
	tabLabs tabID = iota
	tabNodes
	tabInterfaces
	tabLinks
	tabMap
	tabJobs
)

type tabDef struct {
	id    tabID
	key   string
	title string
	// short is the title used when the header runs out of room.
	short string
	// command is the name the view answers to on the command line.
	command string
}

var tabs = []tabDef{
	{tabLabs, "1", "Labs", "Labs", "labs"},
	{tabNodes, "2", "Nodes", "Nodes", "nodes"},
	{tabInterfaces, "3", "Interfaces", "Ifaces", "interfaces"},
	{tabLinks, "4", "Links", "Links", "links"},
	{tabMap, "5", "Map", "Map", "map"},
	{tabJobs, "6", "Jobs", "Jobs", "jobs"},
}

// hint is a key and what it does, shown in the line of hints at the bottom.
type hint struct {
	key  string
	desc string
}

// page is a screen: it fills the body of the window and handles the keys of its context.
// Pages form a stack, the one on top is shown.
type page interface {
	// tab returns the top-level view the page belongs to.
	tab() tabID
	// crumbs returns the path to the page, shown in the title rule.
	crumbs(m *Model) []string
	// info returns short status segments shown at the right of the title rule.
	info(m *Model) []string
	// refresh re-reads the data the page shows after the given topics changed.
	refresh(m *Model, topics clabinternaltuibackend.Topic)
	resize(width, height int)
	// key handles a key press and reports whether it used it.
	key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd)
	// update handles any other message.
	update(m *Model, msg tea.Msg) tea.Cmd
	view(m *Model) string
	hints(m *Model) []hint
	// table returns the table the filter and the mouse act on, nil if there is none.
	table() *clabinternaltuiui.Table
	// focus tells the backend what the page needs sampled.
	focus(m *Model) clabinternaltuibackend.Focus
	// close releases what the page holds, such as a log stream.
	close()
}

// basePage provides the page methods most pages do not need.
type basePage struct {
	width  int
	height int
}

func (p *basePage) resize(width, height int)      { p.width, p.height = width, height }
func (*basePage) update(*Model, tea.Msg) tea.Cmd  { return nil }
func (*basePage) table() *clabinternaltuiui.Table { return nil }
func (*basePage) close()                          {}
func (*basePage) focus(m *Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: m.scope}
}

// tableInfo describes the state of a table for the title rule: the filter, the marks and
// the sort order.
func tableInfo(t *clabinternaltuiui.Table) []string {
	var info []string

	if f := t.Filter(); f != "" {
		info = append(info, "/"+f)
	}

	if n := t.MarkCount(); n > 0 {
		info = append(info, fmt.Sprintf("%d marked", n))
	}

	if title, desc, ok := t.SortState(); ok && title != "" {
		arrow := "▲"
		if desc {
			arrow = "▼"
		}

		info = append(info, "sort: "+title+" "+arrow)
	}

	return info
}

// tableCount formats the number of rows of a table for the title rule.
func tableCount(t *clabinternaltuiui.Table) string {
	shown, total := t.Counts()
	if shown == total {
		return fmt.Sprintf("[%d]", total)
	}

	return fmt.Sprintf("[%d/%d]", shown, total)
}
