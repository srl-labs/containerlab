// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// nodesPage lists the nodes of the scoped lab, or of every lab.
type nodesPage struct {
	basePage

	tbl      *clabinternaltuiui.Table
	hidePane bool
	// allLabs tells whether the table was built with the lab column.
	allLabs bool
	byKey   map[string]*clabinternaltuibackend.Node
}

//nolint:mnd // the widths of the columns are what they are
func nodeColumns(allLabs bool) []clabinternaltuiui.Column {
	right := clabinternaltuiui.AlignRight
	cols := []clabinternaltuiui.Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "name", Title: "NAME", Min: 6, Max: 32},
	}

	if allLabs {
		cols = append(cols, clabinternaltuiui.Column{ID: "lab", Title: "LAB", Max: 20, Priority: 1})
	}

	return append(cols,
		clabinternaltuiui.Column{ID: "kind", Title: "KIND", Max: 18, Priority: 1},
		clabinternaltuiui.Column{ID: "state", Title: "STATE", Priority: 1},
		clabinternaltuiui.Column{ID: "ipv4", Title: "IPV4", Priority: 2},
		clabinternaltuiui.Column{
			ID: "image", Title: "IMAGE", Min: 12, Max: 36, Flex: true, Priority: 3,
		},
		clabinternaltuiui.Column{
			ID:       "cpu",
			Title:    "CPU",
			Min:      4,
			Priority: 2,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		clabinternaltuiui.Column{
			ID: "cpuspark", Title: "", Min: sparkWidth, Priority: 5, NoSort: true, NoFilter: true,
		},
		clabinternaltuiui.Column{
			ID:       "mem",
			Title:    "MEM",
			Min:      4,
			Priority: 3,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		clabinternaltuiui.Column{
			ID: "rx", Title: arrowRx + " RX", Min: 5, Priority: 4, Align: right, Numeric: true,
			NoFilter: true,
		},
		clabinternaltuiui.Column{
			ID: "tx", Title: arrowTx + " TX", Min: 5, Priority: 4, Align: right, Numeric: true,
			NoFilter: true,
		},
		clabinternaltuiui.Column{ID: "age", Title: "AGE", Min: 3, Priority: 4, NoFilter: true},
		clabinternaltuiui.Column{ID: "ipv6", Title: "IPV6", Priority: 6},
	)
}

func newNodesPage(m *Model) *nodesPage {
	p := &nodesPage{}
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*nodesPage) tab() tabID                        { return tabNodes }
func (p *nodesPage) table() *clabinternaltuiui.Table { return p.tbl }
func (p *nodesPage) info(*Model) []string            { return tableInfo(p.tbl) }

func (p *nodesPage) crumbs(m *Model) []string {
	if m.scope == "" {
		return []string{"nodes " + tableCount(p.tbl)}
	}

	return []string{"labs", m.scope, "nodes " + tableCount(p.tbl)}
}

func (*nodesPage) focus(m *Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: m.scope, Rates: true}
}

func (p *nodesPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats) {
		return
	}

	allLabs := m.scope == ""
	if p.tbl == nil || allLabs != p.allLabs {
		p.rebuildTable(m, allLabs)
	}

	labs := m.scopedLabs()
	p.byKey = map[string]*clabinternaltuibackend.Node{}

	var rows []clabinternaltuiui.Row

	for l := range labs {
		for n := range labs[l].Nodes {
			node := &labs[l].Nodes[n]
			p.byKey[node.Container] = node
			rows = append(rows, p.row(m, node, allLabs))
		}
	}

	p.tbl.SetRows(rows)
}

// rebuildTable creates the table anew when the set of columns changes, carrying the
// filter over.
func (p *nodesPage) rebuildTable(m *Model, allLabs bool) {
	filter := ""
	if p.tbl != nil {
		filter = p.tbl.Filter()
	}

	p.allLabs = allLabs
	p.tbl = clabinternaltuiui.NewTable(m.th, nodeColumns(allLabs))
	p.tbl.Empty = "no nodes — the lab has no running containers"
	p.tbl.SetFilter(filter)
	p.resize(p.width, p.height)
}

func (*nodesPage) row(
	m *Model, n *clabinternaltuibackend.Node, allLabs bool,
) clabinternaltuiui.Row {
	readiness := n.Readiness()
	glyph, level := readinessLook(readiness)

	dot := m.levelCell(glyph, level)
	state := m.levelCell(readiness.String(), level)

	// The chart of the load is drawn in the accent color, and in the color of the
	// load once that is high.
	spark := m.sparkCell(n.CPUSeries, cpuLimit(n.CPUSeries, 1), m.th.Muted)
	if lvl := cpuLevel(n.CPUPercent); n.HasStats && lvl != clabinternaltuitheme.LevelSuccess {
		spark.Deco.Color = m.th.LevelStyle(lvl).GetForeground()
	}

	cells := []clabinternaltuiui.Cell{dot, m.cell(n.Name)}
	if allLabs {
		cells = append(cells, m.mutedCell(n.Lab))
	}

	age := m.mutedCell(n.Uptime())
	if age.Text == "" {
		age = m.dash()
	}

	ipv4 := m.cell(stripPrefixLen(n.IPv4))
	if ipv4.Text == "" {
		ipv4 = m.dash()
	}

	cells = append(cells,
		m.styledCell(n.Kind, m.th.Text.Foreground(m.th.Hue(n.Kind))),
		state,
		ipv4,
		m.mutedCell(clabinternaltuiui.ShortImage(n.Image)),
		m.cpuCell(n.CPUPercent, n.HasStats),
		spark,
		m.memCell(n.MemBytes, n.HasStats),
		m.rateCell(n.RxBps, n.HasStats),
		m.rateCell(n.TxBps, n.HasStats),
		age,
		m.mutedCell(stripPrefixLen(n.IPv6)),
	)

	return clabinternaltuiui.Row{Key: n.Container, Cells: cells}
}

// stripPrefixLen drops the prefix length of an address in CIDR notation.
func stripPrefixLen(cidr string) string {
	for i := len(cidr) - 1; i >= 0; i-- {
		if cidr[i] == '/' {
			return cidr[:i]
		}
	}

	return cidr
}

// current returns the node under the cursor.
func (p *nodesPage) current() *clabinternaltuibackend.Node {
	return p.byKey[p.tbl.CursorKey()]
}

// selection returns the nodes an action applies to: the marked ones, or the one under
// the cursor.
func (p *nodesPage) selection() []*clabinternaltuibackend.Node {
	keys := p.tbl.Selection()
	nodes := make([]*clabinternaltuibackend.Node, 0, len(keys))

	for _, key := range keys {
		if n := p.byKey[key]; n != nil {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (p *nodesPage) paneVisible() bool {
	return !p.hidePane && p.height >= paneMinHeight
}

func (p *nodesPage) resize(width, height int) {
	p.basePage.resize(width, height)

	if p.tbl == nil {
		return
	}

	if p.pix {
		if p.pixelPane() {
			height -= nodePaneLines
		}

		p.tbl.SetSize(width, height)

		return
	}

	if p.paneVisible() {
		height -= paneLines
	}

	p.tbl.SetSize(width, height)
}

func (p *nodesPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.tbl.HandleKey(msg) {
		return true, nil
	}

	switch msg.String() {
	case "p":
		p.hidePane = !p.hidePane
		p.resize(p.width, p.height)

		return true, nil
	case keyEnter:
		if n := p.current(); n != nil {
			m.push(newNodePage(m, n.NodeRef))
		}

		return true, nil
	}

	return false, nil
}

// actions implements actor.
func (p *nodesPage) actions(m *Model) (string, []action) {
	return m.nodeActions(p.current(), p.selection())
}

func (p *nodesPage) view(m *Model) string {
	if p.pix {
		return p.viewPixels(m)
	}

	if !p.paneVisible() {
		return p.tbl.View()
	}

	title := ""

	var fields []clabinternaltuiui.Field

	if n := p.current(); n != nil {
		title = n.Container
		fields = nodeFields(m, n)
	}

	return p.tbl.View() + "\n" + paneRule(m, title, p.width) + "\n" +
		clabinternaltuiui.Fields(m.th, fields, p.width, paneRows)
}

// nodeFields returns the details of a node shown in the pane below the node list.
func nodeFields(m *Model, n *clabinternaltuibackend.Node) []clabinternaltuiui.Field {
	_, level := readinessLook(n.Readiness())

	kind := n.Kind
	if n.Type != "" {
		kind += " (" + n.Type + ")"
	}

	shell := ""
	if methods := m.be.ConnectMethods(n); len(methods) > 0 {
		shell = methods[0].Label
	}

	return []clabinternaltuiui.Field{
		{Label: "image", Value: clabinternaltuiui.Sanitize(n.Image)},
		{Label: "kind", Value: clabinternaltuiui.Sanitize(kind)},
		{Label: "status", Value: n.Status, Style: m.th.LevelStyle(level), Styled: true},
		{Label: "ipv4", Value: n.IPv4},
		{Label: "ipv6", Value: n.IPv6},
		{Label: "ports", Value: clabinternaltuiui.Ports(n.Ports)},
		{Label: "shell", Value: shell},
		{Label: "group", Value: clabinternaltuiui.Sanitize(n.Group)},
		{Label: "id", Value: n.ID},
	}
}

func (p *nodesPage) hints(m *Model) []hint {
	hints := []hint{{"⏎", "open"}, {keyActions, "actions"}}

	_, list := p.actions(m)
	hints = append(hints, actionHints(list)...)

	return append(hints, hint{"␣", "mark"}, hint{"/", "filter"}, hint{keyEsc, "back"},
		hint{"?", "help"})
}
