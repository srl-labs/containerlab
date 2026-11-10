// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// summaryRows is the height of the node summary above the interface list.
	summaryRows = 4
	// chartWidth is the width of the history charts in the node summary, and
	// chartBlock the width of a chart with its label and value.
	chartWidth = 30
	chartBlock = chartWidth + 13
	// The summary shows charts only when the window is wide enough for them.
	chartMinWidth = 110

	stateUp   = "up"
	stateDown = "down"
)

// nodePage shows one node: a summary with its recent load and the list of its interfaces.
type nodePage struct {
	basePage

	ref    clabinternaltuibackend.NodeRef
	tbl    *clabinternaltuiui.Table
	ifaces map[string]*clabinternaltuibackend.Interface
}

//nolint:mnd // the widths of the columns are what they are
func interfaceColumns() []clabinternaltuiui.Column {
	right := clabinternaltuiui.AlignRight

	return []clabinternaltuiui.Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "name", Title: "NAME", Min: 5, Max: 24},
		{ID: "alias", Title: "ALIAS", Max: 20, Priority: 4},
		{ID: "state", Title: "STATE", Priority: 1},
		{ID: "peer", Title: "PEER", Max: 30, Priority: 1},
		{ID: "type", Title: "TYPE", Priority: 5},
		{ID: "mtu", Title: "MTU", Priority: 5, Align: right, Numeric: true, NoFilter: true},
		{ID: "mac", Title: "MAC", Priority: 6},
		{
			ID:       "rx",
			Title:    arrowRx + " RX",
			Min:      5,
			Priority: 2,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{
			ID:       "tx",
			Title:    arrowTx + " TX",
			Min:      5,
			Priority: 2,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{ID: "spark", Title: "", Min: sparkWidth, Priority: 3, NoSort: true, NoFilter: true},
		{ID: "netem", Title: "NETEM", Max: 28, Priority: 3},
	}
}

func newNodePage(m *Model, ref clabinternaltuibackend.NodeRef) *nodePage {
	p := &nodePage{ref: ref, tbl: clabinternaltuiui.NewTable(m.th, interfaceColumns())}
	p.tbl.Empty = "no interfaces — the node is not running"
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*nodePage) tab() tabID                        { return tabNodes }
func (p *nodePage) table() *clabinternaltuiui.Table { return p.tbl }

func (p *nodePage) crumbs(*Model) []string {
	return []string{"labs", p.ref.Lab, "nodes", p.ref.Name}
}

func (p *nodePage) info(m *Model) []string {
	n := p.node(m)
	if n == nil {
		return []string{"gone"}
	}

	info := []string{n.Readiness().String()}
	if up := n.Uptime(); up != "" {
		info = append(info, "up "+up)
	}

	return info
}

func (p *nodePage) focus(*Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: p.ref.Lab, Node: p.ref.Name, Rates: true}
}

// node returns the current state of the node, nil when it no longer exists.
func (p *nodePage) node(m *Model) *clabinternaltuibackend.Node {
	lab := m.lab(p.ref.Lab)
	if lab == nil {
		return nil
	}

	return lab.Node(p.ref.Name)
}

func (p *nodePage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats |
		clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicLinks) {
		return
	}

	list := m.be.Interfaces(p.ref)
	p.ifaces = make(map[string]*clabinternaltuibackend.Interface, len(list))
	rows := make([]clabinternaltuiui.Row, 0, len(list))

	for i := range list {
		p.ifaces[list[i].Name] = &list[i]
		rows = append(rows, interfaceRow(m, &list[i], false))
	}

	p.tbl.SetRows(rows)
}

// interfaceLook returns the glyph and the severity that stand for an interface state.
func interfaceLook(state string) (string, clabinternaltuitheme.Level) {
	switch state {
	case stateUp:
		return glyphReady, clabinternaltuitheme.LevelSuccess
	case stateDown:
		return glyphDown, clabinternaltuitheme.LevelError
	default:
		return glyphNone, clabinternaltuitheme.LevelMuted
	}
}

// interfaceRow builds the table row of an interface. withNode prefixes the row with the
// node the interface belongs to, for lists that span nodes.
func interfaceRow(
	m *Model, iface *clabinternaltuibackend.Interface, withNode bool,
) clabinternaltuiui.Row {
	glyph, level := interfaceLook(iface.State)

	state := m.levelCell(iface.State, level)
	if iface.State == "" {
		state = m.dash()
	}

	peer := m.dash()
	if iface.Peer != nil {
		peer = m.cell(iface.Peer.Node + ":" + iface.Peer.Interface)
	}

	netem := m.dash()
	if !iface.Netem.IsZero() {
		netem = m.levelCell(clabinternaltuiui.Netem(iface.Netem), clabinternaltuitheme.LevelWarning)
	}

	mtu := m.dash()
	if iface.MTU > 0 {
		mtu = clabinternaltuiui.Cell{
			Text: strconv.Itoa(iface.MTU), Style: m.th.Muted, Num: float64(iface.MTU),
		}
	}

	cells := []clabinternaltuiui.Cell{m.levelCell(glyph, level), m.cell(iface.Name)}
	key := iface.Name

	if withNode {
		cells = append(cells, m.mutedCell(iface.Node.Name))
		key = iface.Node.Container + "\x00" + iface.Name
	}

	cells = append(cells,
		m.mutedCell(iface.Alias),
		state,
		peer,
		m.mutedCell(iface.Type),
		mtu,
		m.mutedCell(iface.MAC),
		m.rateCell(iface.RxBps, iface.HasStats),
		m.rateCell(iface.TxBps, iface.HasStats),
		m.sparkCell(sumSeries(iface.RxSeries, iface.TxSeries), 0, m.th.Muted),
		netem,
	)

	return clabinternaltuiui.Row{Key: key, Cells: cells}
}

func (p *nodePage) selection() []*clabinternaltuibackend.Interface {
	keys := p.tbl.Selection()
	list := make([]*clabinternaltuibackend.Interface, 0, len(keys))

	for _, key := range keys {
		if iface := p.ifaces[key]; iface != nil {
			list = append(list, iface)
		}
	}

	return list
}

func (p *nodePage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.tbl.SetSize(width, height-summaryRows-1)
}

func (p *nodePage) click(_ *Model, x, y int, _ bool) {
	p.tbl.Click(x, y-summaryRows-1)
}

func (p *nodePage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	return p.tbl.HandleKey(msg), nil
}

// sibling shows the node next to this one in the lab.
//
//nolint:unparam // the result is part of siblingPage, other pages have a command to run
func (p *nodePage) sibling(m *Model, delta int) tea.Cmd {
	next, ok := m.neighborNode(p.ref, delta)
	if !ok {
		return nil
	}

	p.ref = next
	p.tbl.SetFilter("")
	p.tbl.ClearMarks()
	p.refresh(m, clabinternaltuibackend.TopicAll)
	p.tbl.Move(-len(p.ifaces))
	m.be.SetFocus(p.focus(m))

	return nil
}

// actions implements actor: what can be done with the node, followed by what can be done
// with the interfaces that are selected.
func (p *nodePage) actions(m *Model) (string, []action) {
	n := p.node(m)
	if n == nil {
		return "", nil
	}

	title, list := m.nodeActions(n, []*clabinternaltuibackend.Node{n})
	_, ifaceActions := m.interfaceActions(p.selection())

	return title, append(list, ifaceActions...)
}

func (p *nodePage) view(m *Model) string {
	n := p.node(m)
	if n == nil {
		return clabinternaltuiui.FitLines(
			"\n "+m.th.Muted.Render("this node no longer exists"), p.width, p.height)
	}

	return p.summary(m, n) + "\n" + p.rule(m) + "\n" + p.tbl.View()
}

// summary shows what the node is on the left and how busy it has been on the right.
func (p *nodePage) summary(m *Model, n *clabinternaltuibackend.Node) string {
	_, level := readinessLook(n.Readiness())

	kind := n.Kind
	if n.Type != "" {
		kind += " (" + n.Type + ")"
	}

	shell := ""
	if methods := m.be.ConnectMethods(n); len(methods) > 0 {
		shell = methods[0].Label
	}

	fields := []clabinternaltuiui.Field{
		{Label: "name", Value: n.Container},
		{Label: "image", Value: clabinternaltuiui.Sanitize(n.Image)},
		{Label: "ipv4", Value: n.IPv4},
		{Label: "shell", Value: shell},
		{Label: "kind", Value: clabinternaltuiui.Sanitize(kind)},
		{Label: "status", Value: n.Status, Style: m.th.LevelStyle(level), Styled: true},
		{Label: "ipv6", Value: n.IPv6},
		{Label: "ports", Value: clabinternaltuiui.Ports(n.Ports)},
	}

	if p.width < chartMinWidth || !n.HasStats {
		return clabinternaltuiui.Fields(m.th, fields, p.width, summaryRows)
	}

	left := clabinternaltuiui.Fields(m.th, fields, p.width-chartBlock, summaryRows)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, p.charts(m, n))
}

// charts draws the recent CPU load, memory use and traffic of the node.
func (*nodePage) charts(m *Model, n *clabinternaltuibackend.Node) string {
	const labelWidth, valueWidth = 4, 5

	line := func(label, value, chart string, style lipgloss.Style) string {
		return m.th.Muted.Render(clabinternaltuiui.PadRight(label, labelWidth)) + " " +
			m.th.Text.Render(clabinternaltuiui.PadLeft(value, valueWidth)) + "  " +
			style.Render(clabinternaltuiui.PadRight(chart, chartWidth)) + " "
	}

	spark := func(series []float64, limit float64) string {
		return clabinternaltuiui.Sparkline(series, chartWidth, limit)
	}

	// Memory is shown as a share of its limit, when the node has one.
	memory := ""
	if n.MemLimit > 0 {
		memory = clabinternaltuiui.Bar(float64(n.MemBytes)/float64(n.MemLimit), chartWidth, '░')
	}

	return strings.Join([]string{
		line("CPU", clabinternaltuiui.Percent(n.CPUPercent),
			spark(n.CPUSeries, cpuLimit(n.CPUSeries, 1)), m.th.LevelStyle(cpuLevel(n.CPUPercent))),
		line("MEM", clabinternaltuiui.Bytes(n.MemBytes), memory, m.th.Muted),
		line(arrowRx+" RX", clabinternaltuiui.Rate(n.RxBps), spark(n.RxSeries, 0), m.th.Info),
		line(arrowTx+" TX", clabinternaltuiui.Rate(n.TxBps), spark(n.TxSeries, 0), m.th.Info),
	}, "\n")
}

// rule separates the summary from the interface list and tells the state of the list.
func (p *nodePage) rule(m *Model) string {
	rule := m.th.Border.Render
	left := rule(ruleChar+ruleChar) + " " + m.th.Title.Render("interfaces "+tableCount(p.tbl)) + " "

	var right strings.Builder

	for _, seg := range tableInfo(p.tbl) {
		right.WriteString(" " + m.th.Text.Render(seg) + " " + rule(ruleChar+ruleChar))
	}

	fill := p.width - clabinternaltuiui.Width(left) - clabinternaltuiui.Width(right.String())
	if fill < 1 {
		return clabinternaltuiui.PadRight(left, p.width)
	}

	return left + rule(strings.Repeat(ruleChar, fill)) + right.String()
}

func (p *nodePage) hints(m *Model) []hint {
	_, list := p.actions(m)

	hints := append([]hint{{keyActions, "actions"}}, actionHints(list)...)

	return append(hints, hint{"[ ]", "other node"}, hint{keyEsc, "back"}, hint{"?", "help"})
}
