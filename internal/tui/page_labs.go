// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// paneRows is the number of lines of the details pane below a table, and paneLines
	// its height including the rule on top of it.
	paneRows  = 3
	paneLines = paneRows + 1
	// The details pane needs a window at least this high to be worth its space.
	paneMinHeight = 16
	// topoMinWidth is the least width of the topology path in the lab list.
	topoMinWidth = 30
)

// labsPage lists the deployed labs and the topology files that are not deployed.
type labsPage struct {
	basePage

	tbl      *clabinternaltuiui.Table
	hidePane bool
	// byKey finds the lab a table row stands for.
	byKey map[string]int
}

//nolint:mnd // the widths of the columns are what they are
func labColumns() []clabinternaltuiui.Column {
	right := clabinternaltuiui.AlignRight

	// The path of the topology is what tells labs apart that are not deployed, so it
	// keeps a readable width: the columns of lesser interest give way before it does.
	return []clabinternaltuiui.Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "name", Title: "NAME", Min: 8, Max: 32},
		{ID: "state", Title: "STATE", Priority: 1},
		{ID: "nodes", Title: "NODES", Priority: 1, NoFilter: true},
		{ID: "cpu", Title: "CPU", Priority: 3, Align: right, Numeric: true, NoFilter: true},
		{ID: "cpuspark", Title: "", Min: sparkWidth, Priority: 6, NoSort: true, NoFilter: true},
		{ID: "mem", Title: "MEM", Priority: 3, Align: right, Numeric: true, NoFilter: true},
		{
			ID:       "rx",
			Title:    arrowRx + " RX",
			Min:      5,
			Priority: 4,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{
			ID:       "tx",
			Title:    arrowTx + " TX",
			Min:      5,
			Priority: 4,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{ID: "age", Title: "AGE", Min: 4, Priority: 3, NoFilter: true},
		{ID: "owner", Title: "OWNER", Max: 12, Priority: 5},
		{
			ID:        "topo",
			Title:     "TOPOLOGY",
			Min:       topoMinWidth,
			Flex:      true,
			Priority:  2,
			TruncLeft: true,
		},
	}
}

func newLabsPage(m *Model) *labsPage {
	p := &labsPage{tbl: clabinternaltuiui.NewTable(m.th, labColumns())}
	p.tbl.Empty = "no labs found — deploy one with `containerlab deploy -t <topology file>`"
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*labsPage) tab() tabID                        { return tabLabs }
func (p *labsPage) table() *clabinternaltuiui.Table { return p.tbl }

func (p *labsPage) crumbs(*Model) []string {
	return []string{"labs " + tableCount(p.tbl)}
}

func (p *labsPage) info(*Model) []string { return tableInfo(p.tbl) }

// focus asks for no lab in particular: the list shows the totals of every lab.
func (*labsPage) focus(*Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Rates: true}
}

func labKey(lab *clabinternaltuibackend.Lab) string {
	return lab.Name + "\x00" + lab.TopoFile
}

// stateRank orders the labs: what runs comes first, what is not deployed last.
func stateRank(s clabinternaltuibackend.LabState) int {
	switch s {
	case clabinternaltuibackend.LabRunning:
		return 0
	case clabinternaltuibackend.LabPartial:
		return 1
	case clabinternaltuibackend.LabFailed:
		return 2 //nolint:mnd // rank
	case clabinternaltuibackend.LabStopped:
		return 3 //nolint:mnd // rank
	default:
		return 4 //nolint:mnd // rank
	}
}

func (p *labsPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats |
		clabinternaltuibackend.TopicJobs) {
		return
	}

	order := make([]int, len(m.labs))
	for i := range m.labs {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		la, lb := &m.labs[order[a]], &m.labs[order[b]]
		if ra, rb := stateRank(la.State()), stateRank(lb.State()); ra != rb {
			return ra < rb
		}

		return clabinternaltuiui.NaturalLess(la.Name, lb.Name)
	})

	p.byKey = make(map[string]int, len(m.labs))
	rows := make([]clabinternaltuiui.Row, 0, len(m.labs))

	for _, i := range order {
		lab := &m.labs[i]
		p.byKey[labKey(lab)] = i
		rows = append(rows, p.row(m, lab))
	}

	p.tbl.SetRows(rows)
}

// cpuCell shows the load of a whole lab. It adds up the nodes, so unlike the load of a
// single node it is not colored: several busy cores are normal for a lab.
func (*labsPage) cpuCell(m *Model, percent float64, has bool) clabinternaltuiui.Cell {
	c := m.cpuCell(percent, has)
	if has {
		c.Style = m.th.Text
	}

	return c
}

func (p *labsPage) row(m *Model, lab *clabinternaltuibackend.Lab) clabinternaltuiui.Row {
	state := lab.State()
	glyph, level := labLook(state)

	stateText := state.String()
	if lab.Busy {
		stateText, level = "busy", clabinternaltuitheme.LevelInfo
	}

	row := clabinternaltuiui.Row{Key: labKey(lab)}

	if !lab.Deployed {
		nodes := clabinternaltuiui.Dash
		if lab.DefinedNodes > 0 {
			nodes = fmt.Sprintf("%s/%d", clabinternaltuiui.Dash, lab.DefinedNodes)
		}

		row.Cells = []clabinternaltuiui.Cell{
			m.levelCell(glyph, level), m.mutedCell(lab.Name), m.levelCell(stateText, level),
			m.mutedCell(nodes), m.dash(), {}, m.dash(), m.dash(), m.dash(), m.dash(), m.dash(),
			m.mutedCell(clabinternaltuiui.HomePath(lab.TopoFile, m.opts.Home)),
		}

		return row
	}

	var (
		cpu, rx, tx float64
		mem         uint64
		has         bool
		series      [][]float64
	)

	for i := range lab.Nodes {
		n := &lab.Nodes[i]
		if !n.HasStats {
			continue
		}

		has = true
		cpu += n.CPUPercent
		mem += n.MemBytes
		rx += n.RxBps
		tx += n.TxBps
		series = append(series, n.CPUSeries)
	}

	age := m.mutedCell(labAge(lab))
	if age.Text == "" {
		age = m.dash()
	}

	cpuSeries := sumSeries(series...)

	row.Cells = []clabinternaltuiui.Cell{
		m.levelCell(glyph, level),
		m.cell(lab.Name),
		m.levelCell(stateText, level),
		m.cell(fmt.Sprintf("%d/%d", lab.ReadyNodes(), len(lab.Nodes))),
		p.cpuCell(m, cpu, has),
		m.sparkCell(cpuSeries, cpuLimit(cpuSeries, len(series)), m.th.Muted),
		m.memCell(mem, has),
		m.rateCell(rx, has),
		m.rateCell(tx, has),
		age,
		m.mutedCell(lab.Owner),
		m.mutedCell(clabinternaltuiui.HomePath(lab.TopoFile, m.opts.Home)),
	}

	return row
}

// current returns the lab under the cursor.
func (p *labsPage) current(m *Model) *clabinternaltuibackend.Lab {
	i, ok := p.byKey[p.tbl.CursorKey()]
	if !ok || i >= len(m.labs) {
		return nil
	}

	return &m.labs[i]
}

func (p *labsPage) paneVisible() bool {
	return !p.hidePane && p.height >= paneMinHeight
}

func (p *labsPage) resize(width, height int) {
	p.basePage.resize(width, height)

	if p.paneVisible() {
		height -= paneLines
	}

	p.tbl.SetSize(width, height)
}

func (p *labsPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.tbl.HandleKey(msg) {
		return true, nil
	}

	lab := p.current(m)

	switch msg.String() {
	case "p":
		p.hidePane = !p.hidePane
		p.resize(p.width, p.height)

		return true, nil
	case keyEnter:
		p.open(m, lab)

		return true, nil
	}

	return false, nil
}

// actions implements actor.
func (p *labsPage) actions(m *Model) (string, []action) {
	return m.labActions(p.current(m))
}

// open shows the nodes of a lab. A lab that is not deployed has none yet: its topology
// is shown instead, which is what deploying it would bring up.
func (*labsPage) open(m *Model, lab *clabinternaltuibackend.Lab) {
	if lab == nil {
		return
	}

	m.openLab(lab)

	if !lab.Deployed {
		m.notifyf(clabinternaltuitheme.LevelInfo,
			"%s is not deployed: this is its topology — esc and D to deploy it", lab.Name)
	}
}

func (p *labsPage) view(m *Model) string {
	if !p.paneVisible() {
		return p.tbl.View()
	}

	return p.tbl.View() + "\n" + p.pane(m)
}

// pane describes the lab under the cursor.
func (p *labsPage) pane(m *Model) string {
	lab := p.current(m)
	if lab == nil {
		return paneRule(m, "", p.width) + "\n" + clabinternaltuiui.FitLines("", p.width, paneRows)
	}

	state := lab.State()
	_, level := labLook(state)

	stateText := state.String()
	if lab.Deployed {
		stateText += fmt.Sprintf(" · %d/%d nodes ready", lab.ReadyNodes(), len(lab.Nodes))
	}

	nodes := kindSummary(lab.Nodes)
	if !lab.Deployed && lab.DefinedNodes > 0 {
		nodes = fmt.Sprintf("%d defined", lab.DefinedNodes)
	}

	shares := make([]string, 0, len(lab.Shares))
	for _, s := range lab.Shares {
		shares = append(shares, string(s.Tool)+" "+s.Link)
	}

	fields := []clabinternaltuiui.Field{
		{Label: "topology", Value: clabinternaltuiui.HomePath(lab.TopoFile, m.opts.Home)},
		{Label: "lab dir", Value: clabinternaltuiui.HomePath(lab.LabDir, m.opts.Home)},
		{Label: "sharing", Value: strings.Join(shares, ", ")},
		{Label: "state", Value: stateText, Style: m.th.LevelStyle(level), Styled: true},
		{Label: "nodes", Value: nodes},
		{Label: "owner", Value: lab.Owner},
	}

	return paneRule(m, lab.Name, p.width) + "\n" +
		clabinternaltuiui.Fields(m.th, fields, p.width, paneRows)
}

// paneRule renders the rule that separates a table from the details pane below it.
func paneRule(m *Model, title string, width int) string {
	rule := m.th.Border.Render
	left := rule(ruleChar+ruleChar) + " "

	if title != "" {
		left += m.th.Title.Render(clabinternaltuiui.Sanitize(title)) + " "
	}

	right := " " + m.th.Key.Render(
		"p",
	) + " " + m.th.KeyDesc.Render(
		"hide",
	) + " " + rule(
		ruleChar+ruleChar,
	)

	fill := width - clabinternaltuiui.Width(left) - clabinternaltuiui.Width(right)
	if fill < 1 {
		return clabinternaltuiui.PadRight(left, width)
	}

	return left + rule(strings.Repeat(ruleChar, fill)) + right
}

func (p *labsPage) hints(m *Model) []hint {
	hints := []hint{{"⏎", "open"}, {keyActions, "actions"}}

	_, list := p.actions(m)
	hints = append(hints, actionHints(list)...)

	return append(hints, hint{"/", "filter"}, hint{":", "command"}, hint{"?", "help"})
}
