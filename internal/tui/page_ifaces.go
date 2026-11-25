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

// interfacesPage lists the interfaces of every node in scope.
type interfacesPage struct {
	basePage

	tbl    *clabinternaltuiui.Table
	ifaces map[string]*clabinternaltuibackend.Interface
}

//nolint:mnd // the widths of the columns are what they are
func allInterfaceColumns() []clabinternaltuiui.Column {
	cols := interfaceColumns()
	node := clabinternaltuiui.Column{ID: "node", Title: "NODE", Max: 24}

	// The node comes right after the interface name.
	return append(cols[:2:2], append([]clabinternaltuiui.Column{node}, cols[2:]...)...)
}

func newInterfacesPage(m *Model) *interfacesPage {
	p := &interfacesPage{tbl: clabinternaltuiui.NewTable(m.th, allInterfaceColumns())}
	p.tbl.Empty = "no interfaces — no node is running"
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*interfacesPage) tab() tabID                        { return tabInterfaces }
func (p *interfacesPage) table() *clabinternaltuiui.Table { return p.tbl }
func (p *interfacesPage) info(*Model) []string            { return tableInfo(p.tbl) }

func (p *interfacesPage) crumbs(m *Model) []string {
	if m.scope == "" {
		return []string{"interfaces " + tableCount(p.tbl)}
	}

	return []string{"labs", m.scope, "interfaces " + tableCount(p.tbl)}
}

func (*interfacesPage) focus(m *Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: m.scope, Rates: true}
}

func (p *interfacesPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.tbl.SetSize(width, height)
}

func (p *interfacesPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats |
		clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicLinks) {
		return
	}

	p.ifaces = map[string]*clabinternaltuibackend.Interface{}

	var rows []clabinternaltuiui.Row

	labs := m.scopedLabs()
	for l := range labs {
		for n := range labs[l].Nodes {
			list := m.be.Interfaces(labs[l].Nodes[n].NodeRef)

			for i := range list {
				row := interfaceRow(m, &list[i], true)
				p.ifaces[row.Key] = &list[i]
				rows = append(rows, row)
			}
		}
	}

	p.tbl.SetRows(rows)
}

func (p *interfacesPage) selection() []*clabinternaltuibackend.Interface {
	keys := p.tbl.Selection()
	list := make([]*clabinternaltuibackend.Interface, 0, len(keys))

	for _, key := range keys {
		if iface := p.ifaces[key]; iface != nil {
			list = append(list, iface)
		}
	}

	return list
}

func (p *interfacesPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.tbl.HandleKey(msg) {
		return true, nil
	}

	if selected := p.selection(); msg.String() == keyEnter && len(selected) > 0 {
		m.push(newNodePage(m, selected[0].Node))

		return true, nil
	}

	return false, nil
}

// actions implements actor.
func (p *interfacesPage) actions(m *Model) (string, []action) {
	return m.interfaceActions(p.selection())
}

func (p *interfacesPage) view(*Model) string { return p.tbl.View() }

func (p *interfacesPage) hints(m *Model) []hint {
	_, list := p.actions(m)
	hints := append([]hint{{"⏎", "node"}, {keyActions, "actions"}}, actionHints(list)...)

	return append(hints, hint{"␣", "mark"}, hint{"/", "filter"}, hint{keyEsc, "back"},
		hint{"?", "help"})
}

// hiddenPriority makes a column the first to go when room is short.
const hiddenPriority = 9

// linkRow is what the links page knows about a link.
type linkRow struct {
	lab  string
	link clabinternaltuibackend.Link
	a, z *clabinternaltuibackend.Interface
}

// linksPage lists the links of the labs in scope, with the state and the traffic of both
// their ends.
type linksPage struct {
	basePage

	tbl   *clabinternaltuiui.Table
	links map[string]*linkRow
	// allLabs tells whether the table was built with the lab column.
	allLabs bool
}

//nolint:mnd // the widths of the columns are what they are
func linkColumns(allLabs bool) []clabinternaltuiui.Column {
	right := clabinternaltuiui.AlignRight

	lab := clabinternaltuiui.Column{ID: "lab", Title: "LAB", Max: 20, Priority: 4}
	if !allLabs {
		// Scoped to one lab the column would repeat its name on every row.
		lab.Max, lab.Priority, lab.Title = 1, hiddenPriority, ""
	}

	return []clabinternaltuiui.Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "a", Title: "A", Min: 8, Max: 34},
		{ID: "z", Title: "Z", Min: 8, Max: 34},
		lab,
		{ID: "state", Title: "STATE", Priority: 1},
		{
			ID:       "az",
			Title:    "A → Z",
			Min:      5,
			Priority: 2,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{
			ID:       "za",
			Title:    "Z → A",
			Min:      5,
			Priority: 2,
			Align:    right,
			Numeric:  true,
			NoFilter: true,
		},
		{ID: "spark", Title: "", Min: sparkWidth, Priority: 3, NoSort: true, NoFilter: true},
		{ID: "netem", Title: "NETEM", Max: 30, Priority: 3},
	}
}

func newLinksPage(m *Model) *linksPage {
	p := &linksPage{}
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

// rebuildTable creates the table anew when the set of columns changes, carrying the
// filter over.
func (p *linksPage) rebuildTable(m *Model, allLabs bool) {
	filter := ""
	if p.tbl != nil {
		filter = p.tbl.Filter()
	}

	p.allLabs = allLabs
	p.tbl = clabinternaltuiui.NewTable(m.th, linkColumns(allLabs))
	p.tbl.Empty = "no links — the labs in view have no topology data"
	p.tbl.SetFilter(filter)
	p.tbl.SetSize(p.width, p.height)
}

func (*linksPage) tab() tabID                        { return tabLinks }
func (p *linksPage) table() *clabinternaltuiui.Table { return p.tbl }
func (p *linksPage) info(*Model) []string            { return tableInfo(p.tbl) }

func (p *linksPage) crumbs(m *Model) []string {
	if m.scope == "" {
		return []string{"links " + tableCount(p.tbl)}
	}

	return []string{"labs", m.scope, "links " + tableCount(p.tbl)}
}

func (*linksPage) focus(m *Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: m.scope, Rates: true}
}

func (p *linksPage) resize(width, height int) {
	p.basePage.resize(width, height)

	if p.tbl != nil {
		p.tbl.SetSize(width, height)
	}
}

// labInterfaces returns the interfaces of the running nodes of a lab, keyed by node and
// interface name.
func labInterfaces(
	m *Model, lab *clabinternaltuibackend.Lab,
) map[[2]string]*clabinternaltuibackend.Interface {
	index := map[[2]string]*clabinternaltuibackend.Interface{}

	for n := range lab.Nodes {
		list := m.be.Interfaces(lab.Nodes[n].NodeRef)
		for i := range list {
			index[[2]string{lab.Nodes[n].Name, list[i].Name}] = &list[i]
		}
	}

	return index
}

func (p *linksPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats |
		clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicLinks) {
		return
	}

	if allLabs := m.scope == ""; p.tbl == nil || allLabs != p.allLabs {
		p.rebuildTable(m, allLabs)
	}

	p.links = map[string]*linkRow{}

	var rows []clabinternaltuiui.Row

	labs := m.scopedLabs()
	for l := range labs {
		lab := &labs[l]
		if !lab.Deployed && m.scope == "" {
			continue
		}

		ifaces := labInterfaces(m, lab)
		topo := m.be.Topology(lab.Name)

		for i := range topo.Links {
			link := topo.Links[i]
			entry := &linkRow{lab: lab.Name, link: link}
			entry.a = ifaces[[2]string{link.A.Node, link.A.Interface}]

			if link.Z != nil {
				entry.z = ifaces[[2]string{link.Z.Node, link.Z.Interface}]
			}

			row := p.row(m, lab, entry, i)
			p.links[row.Key] = entry
			rows = append(rows, row)
		}
	}

	p.tbl.SetRows(rows)
}

// linkState combines the state of both ends of a link.
func linkState(lab *clabinternaltuibackend.Lab, l *linkRow) (string, clabinternaltuitheme.Level) {
	if !lab.Deployed {
		return "defined", clabinternaltuitheme.LevelMuted
	}

	states := []string{"", ""}
	if l.a != nil {
		states[0] = l.a.State
	}

	if l.z != nil {
		states[1] = l.z.State
	}

	switch {
	case l.a == nil && l.z == nil:
		return stateDown, clabinternaltuitheme.LevelError
	case states[0] == stateDown || states[1] == stateDown:
		return stateDown, clabinternaltuitheme.LevelError
	case l.a == nil || (l.z == nil && l.link.Z != nil && !l.link.Z.External):
		// One end is missing: its node is not running.
		return stateDown, clabinternaltuitheme.LevelError
	case states[0] == stateUp || states[1] == stateUp:
		return stateUp, clabinternaltuitheme.LevelSuccess
	default:
		return "", clabinternaltuitheme.LevelMuted
	}
}

func endpointText(e *clabinternaltuibackend.Endpoint) string {
	if e == nil {
		return ""
	}

	return e.Node + ":" + e.Interface
}

func (*linksPage) row(
	m *Model, lab *clabinternaltuibackend.Lab, l *linkRow, index int,
) clabinternaltuiui.Row {
	state, level := linkState(lab, l)
	glyph := glyphNone

	switch state {
	case stateUp:
		glyph = glyphReady
	case stateDown:
		glyph = glyphDown
	}

	stateCell := m.levelCell(state, level)
	if state == "" {
		stateCell = m.dash()
	}

	az, za, spark := m.dash(), m.dash(), clabinternaltuiui.Cell{}
	if l.a != nil && l.a.HasStats {
		az = m.rateCell(l.a.TxBps, true)
		za = m.rateCell(l.a.RxBps, true)
		spark = m.sparkCell(sumSeries(l.a.RxSeries, l.a.TxSeries), 0, m.th.Muted)
	} else if l.z != nil && l.z.HasStats {
		az = m.rateCell(l.z.RxBps, true)
		za = m.rateCell(l.z.TxBps, true)
		spark = m.sparkCell(sumSeries(l.z.RxSeries, l.z.TxSeries), 0, m.th.Muted)
	}

	netem := m.dash()

	for _, end := range []*clabinternaltuibackend.Interface{l.a, l.z} {
		if end != nil && !end.Netem.IsZero() {
			netem = m.levelCell(end.Node.Name+": "+clabinternaltuiui.Netem(end.Netem),
				clabinternaltuitheme.LevelWarning)

			break
		}
	}

	z := m.cell(endpointText(l.link.Z))
	if l.link.Z == nil {
		z = m.dash()
	}

	labCell := clabinternaltuiui.Cell{}
	if m.scope == "" {
		labCell = m.mutedCell(lab.Name)
	}

	return clabinternaltuiui.Row{
		Key: lab.Name + "\x00" + itoa(index),
		Cells: []clabinternaltuiui.Cell{
			m.levelCell(glyph, level),
			m.cell(endpointText(&l.link.A)),
			z,
			labCell,
			stateCell,
			az,
			za,
			spark,
			netem,
		},
	}
}

// ends returns the interfaces of the selected links that exist on running nodes.
func (p *linksPage) ends(first bool) []*clabinternaltuibackend.Interface {
	var list []*clabinternaltuibackend.Interface

	for _, key := range p.tbl.Selection() {
		l := p.links[key]
		if l == nil {
			continue
		}

		if l.a != nil {
			list = append(list, l.a)
		}

		if !first && l.z != nil {
			list = append(list, l.z)
		}
	}

	return list
}

func (p *linksPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.tbl.HandleKey(msg) {
		return true, nil
	}

	l := p.links[p.tbl.CursorKey()]
	if l == nil {
		return false, nil
	}

	if msg.String() != keyEnter {
		return false, nil
	}

	if lab := m.lab(l.lab); lab != nil {
		if n := lab.Node(l.link.A.Node); n != nil {
			m.push(newNodePage(m, n.NodeRef))
		}
	}

	return true, nil
}

// actions implements actor. Impairments shape the traffic leaving an interface, so they
// are set on the A end of a link, while a capture listens on both of its ends.
func (p *linksPage) actions(m *Model) (string, []action) {
	l := p.links[p.tbl.CursorKey()]
	if l == nil {
		return "", nil
	}

	_, list := m.interfaceActions(p.ends(true))
	list = linkActions(m, list, p.ends(false))

	title := endpointText(&l.link.A) + " ↔ " + endpointText(l.link.Z)

	return title, append(list, action{
		key: "t", name: "map", desc: "show the link in the topology",
		run: func() tea.Cmd { return m.showMap(l.lab) },
	})
}

// linkActions turns the actions of an interface into the ones of a link: the capture
// covers both ends, and copying the address of one end makes little sense.
func linkActions(
	m *Model, list []action, both []*clabinternaltuibackend.Interface,
) []action {
	out := make([]action, 0, len(list))

	for _, a := range list {
		switch a.key {
		case "C":
			a.run = func() tea.Cmd { return m.captureForm(both) }
		case "m":
			continue
		}

		out = append(out, a)
	}

	return out
}

func (p *linksPage) view(*Model) string { return p.tbl.View() }

func (p *linksPage) hints(m *Model) []hint {
	_, list := p.actions(m)
	hints := append([]hint{{"⏎", "node"}, {keyActions, "actions"}}, actionHints(list)...)

	return append(hints, hint{"/", "filter"}, hint{keyEsc, "back"}, hint{"?", "help"})
}
