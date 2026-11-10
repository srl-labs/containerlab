// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// mapPanelWidth is the width of the details panel next to the map, and
	// mapPanelMinWidth the window width from which the panel is shown.
	mapPanelWidth    = 36
	mapPanelMinWidth = 100
	// mapPanStep is the number of cells the map moves per key press.
	mapPanStep = 8
	// mapMargin is the blank space kept around the focused node when it is scrolled
	// into view.
	mapMargin = 2

	// panelValueWidth and panelSparkWidth size the load figures in the details panel.
	panelValueWidth = 5
	panelSparkWidth = 2 * sparkWidth

	noEdge = -1
)

// mapPage draws the topology of a lab: its nodes as boxes and its links as lines between
// them, colored by their state.
type mapPage struct {
	basePage

	// lab is the lab on display. It follows the scope, or is picked when there is none.
	lab string

	topology clabinternaltuibackend.Topology
	// full is the whole topology, graph the part of it that is drawn.
	full   clabinternaltuitopo.Graph
	graph  clabinternaltuitopo.Graph
	layout *clabinternaltuitopo.Layout
	// signature tells the topology the layout was computed for.
	signature string

	// extent is how much of the lab the user asked for, shown what that came down to.
	extent mapExtent
	shown  mapExtent

	// grid is the drawing as last painted. It is reused until what it shows changes.
	grid     *clabinternaltuitopo.Grid
	gridFor  mapPainted
	revision int

	mode clabinternaltuitopo.Mode
	zoom clabinternaltuitopo.Zoom
	fit  bool
	// labels is what is written on the links.
	labels mapLabels

	node string
	edge int
	// anchor is the node the focus was on before, see mapScope.
	anchor string

	offsetX   int
	offsetY   int
	hidePanel bool

	ifaces map[[2]string]*clabinternaltuibackend.Interface
}

// mapLabels tells what is written on the links of the map. Both are a matter of taste and
// of how busy a drawing is, so each can be switched off; the choice holds for every lab
// the map is opened for.
type mapLabels struct {
	// names tells on which links the name of the interface is written at each end.
	names mapNames
	// rates writes the traffic rate in the middle of a link.
	rates bool
}

// mapNames tells which links of the map have the names of their interfaces written at
// their ends.
type mapNames int

const (
	// namesFocus names the interfaces of the links of the node in focus. It is the
	// default: the names are there for what is looked at, and the drawing stays calm.
	namesFocus mapNames = iota
	// namesAll names the interfaces of every link there is room for.
	namesAll
	// namesOff leaves the names out.
	namesOff
)

func (n mapNames) String() string {
	switch n {
	case namesAll:
		return "on every link"
	case namesOff:
		return "off"
	default:
		return "on the links of the node in focus"
	}
}

// mapPainted identifies what a painted grid shows.
type mapPainted struct {
	layout   *clabinternaltuitopo.Layout
	node     string
	edge     int
	labels   mapLabels
	revision int
}

func newMapPage(m *Model) *mapPage {
	p := &mapPage{edge: noEdge, zoom: clabinternaltuitopo.ZoomNormal, fit: true}
	p.labels = mapLabels{names: namesFocus, rates: true}
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*mapPage) tab() tabID { return tabMap }

func (p *mapPage) crumbs(*Model) []string {
	if p.lab == "" {
		return []string{"map"}
	}

	return []string{"labs", p.lab, "map"}
}

func (p *mapPage) info(*Model) []string {
	if p.layout == nil {
		return nil
	}

	info := []string{"layout: " + modeName(p.layout.Mode()), "zoom: " + zoomName(p.zoom)}

	if len(p.graph.Nodes) == len(p.full.Nodes) {
		return append(info,
			fmt.Sprintf("%d nodes · %d links", len(p.full.Nodes), len(p.full.Links)))
	}

	return append(info, "view: "+p.shown.String(),
		fmt.Sprintf("%d of %d nodes", len(p.graph.Nodes), len(p.full.Nodes)))
}

func modeName(mode clabinternaltuitopo.Mode) string {
	switch mode {
	case clabinternaltuitopo.ModeSaved:
		return "saved"
	case clabinternaltuitopo.ModeTiers:
		return "tiers"
	case clabinternaltuitopo.ModeGraph:
		return "graph"
	default:
		return "auto"
	}
}

func zoomName(zoom clabinternaltuitopo.Zoom) string {
	switch zoom {
	case clabinternaltuitopo.ZoomCompact:
		return "compact"
	case clabinternaltuitopo.ZoomDetail:
		return "detail"
	default:
		return "normal"
	}
}

func (p *mapPage) focus(*Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: p.lab, Rates: true}
}

// pickLab returns the lab to draw: the one in scope, else the one shown before, else the
// first lab that is deployed.
func (p *mapPage) pickLab(m *Model) string {
	if m.scope != "" {
		return m.scope
	}

	if p.lab != "" && m.lab(p.lab) != nil {
		return p.lab
	}

	for i := range m.labs {
		if m.labs[i].Deployed {
			return m.labs[i].Name
		}
	}

	if len(m.labs) > 0 {
		return m.labs[0].Name
	}

	return ""
}

func (p *mapPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicLabs | clabinternaltuibackend.TopicStats |
		clabinternaltuibackend.TopicInterfaces | clabinternaltuibackend.TopicLinks) {
		return
	}

	lab := p.pickLab(m)
	if lab != p.lab {
		p.lab = lab
		p.signature = ""
		p.node, p.edge = "", noEdge
		p.offsetX, p.offsetY = 0, 0
		m.be.SetFocus(p.focus(m))
	}

	if p.lab == "" {
		p.layout = nil

		return
	}

	p.topology = m.be.Topology(p.lab)
	p.revision++

	if current := m.lab(p.lab); current != nil {
		p.ifaces = labInterfaces(m, current)
	}

	if sig := topologySignature(&p.topology); sig != p.signature || p.layout == nil {
		first := p.layout == nil
		p.signature = sig
		p.compute()

		if first {
			p.center()
		}
	}
}

// topologySignature identifies the shape of a topology, so that the layout is computed
// again only when nodes or links change.
func topologySignature(t *clabinternaltuibackend.Topology) string {
	var b strings.Builder

	for i := range t.Nodes {
		n := &t.Nodes[i]
		fmt.Fprintf(&b, "%s|%s|%d|%v|%v;", n.Name, n.Role, n.Level, n.X, n.Y)
	}

	for i := range t.Links {
		b.WriteString(endpointText(&t.Links[i].A) + ">" + endpointText(t.Links[i].Z) + ";")
	}

	return b.String()
}

// compute lays the topology out. Links to something that is not a node of the lab, such
// as the host or a bridge, are left out of the drawing.
func (p *mapPage) compute() {
	full := clabinternaltuitopo.Graph{}
	known := map[string]bool{}

	for i := range p.topology.Nodes {
		n := &p.topology.Nodes[i]
		known[n.Name] = true
		full.Nodes = append(full.Nodes, clabinternaltuitopo.Node{
			ID: n.Name, Kind: n.Kind, Role: n.Role, Group: n.Group, Level: n.Level,
			X: n.X, Y: n.Y, HasPos: n.HasPos,
		})
	}

	for i := range p.topology.Links {
		l := &p.topology.Links[i]
		if l.Z == nil || !known[l.A.Node] || !known[l.Z.Node] {
			continue
		}

		full.Links = append(full.Links, clabinternaltuitopo.Link{
			A: l.A.Node, B: l.Z.Node, AIface: l.A.Interface, BIface: l.Z.Interface,
		})
	}

	p.full = full

	if !known[p.node] {
		p.node, p.edge = busiestNode(&p.full), noEdge
	}

	p.shown = p.extent
	if p.extent == extentAuto {
		p.shown = p.autoExtent()
	}

	p.graph = reduceGraph(&p.full, p.shown, p.scope(p.shown))
	p.layout = p.layoutAt(p.zoom)

	// The link in focus is gone when its far end is no longer drawn.
	if !p.hasEdge(p.edge) {
		p.edge = noEdge
	}

	if p.fit {
		p.fitZoom()
	}

	p.reveal()
}

func (p *mapPage) hasEdge(id int) bool {
	for _, e := range p.layout.Edges() {
		if e.ID == id {
			return true
		}
	}

	return false
}

// fitZoom picks the most detailed zoom at which the whole drawing fits the view.
func (p *mapPage) fitZoom() {
	width, height := p.viewSize()
	if width <= 0 || height <= 0 {
		return
	}

	for _, zoom := range []clabinternaltuitopo.Zoom{
		clabinternaltuitopo.ZoomDetail, clabinternaltuitopo.ZoomNormal, clabinternaltuitopo.ZoomCompact,
	} {
		layout := p.layoutAt(zoom)

		w, h := layout.Size()
		if (w <= width && h <= height) || zoom == clabinternaltuitopo.ZoomCompact {
			p.zoom, p.layout = zoom, layout

			return
		}
	}
}

// The room a link between two neighboring nodes needs for what is written on it.
const (
	// namesRoom goes with two names: a blank on both sides of each, a piece of the
	// line in front of each and some more of it between them.
	namesRoom = 8
	// rateRoom is a rate with a blank on both sides and some line next to it.
	rateRoom = 9
	// badgeRoom is the mark of a link that stands for several.
	badgeRoom = 3
	// namedRowGap is the number of lines kept between two rows of nodes for a link
	// that runs straight down to carry a name at each of its ends.
	namedRowGap = 2
	// rateMargin is the room a rate takes next to a link at the edge of the drawing.
	rateMargin = 4
)

// layoutAt draws the lab at a zoom. Links get the room that what is written on them
// asks for, as much of it as the view has to give: a lab that only fits without is
// better shown whole than with every name in place.
func (p *mapPage) layoutAt(zoom clabinternaltuitopo.Zoom) *clabinternaltuitopo.Layout {
	opts := clabinternaltuitopo.Options{Mode: p.mode, Zoom: zoom}
	plain := clabinternaltuitopo.Compute(p.graph, opts)

	names, rates := p.labels.names != namesOff, p.labels.rates
	if zoom == clabinternaltuitopo.ZoomCompact || (!names && !rates) {
		return plain
	}

	width, height := p.viewSize()
	if w, h := plain.Size(); w > width || h > height {
		return plain
	}

	room, margin := p.linkRoom()

	rowGap := 0
	if names {
		rowGap = namedRowGap
	}

	// From all of the room to the part of it that costs no width.
	for _, try := range []clabinternaltuitopo.Options{
		{Room: room, Margin: margin, RowGap: rowGap},
		{Room: room, RowGap: rowGap},
		{Margin: margin, RowGap: rowGap},
		{RowGap: rowGap},
	} {
		if len(try.Room) == 0 && try.Margin == 0 && try.RowGap == 0 {
			continue
		}

		try.Mode, try.Zoom = p.mode, zoom

		roomy := clabinternaltuitopo.Compute(p.graph, try)
		if w, h := roomy.Size(); w <= width && h <= height {
			return roomy
		}
	}

	return plain
}

// linkRoom tells how long the link between two nodes has to be for what is written on
// it, for every pair of nodes with a link, and how much room a text next to a link at
// the edge of the drawing takes.
func (p *mapPage) linkRoom() (room map[[2]string]int, margin int) {
	pairs := map[[2]string][]int{}

	for i := range p.graph.Links {
		a, b := p.graph.Links[i].A, p.graph.Links[i].B
		if a == b {
			continue
		}

		if a > b {
			a, b = b, a
		}

		pairs[[2]string{a, b}] = append(pairs[[2]string{a, b}], i)
	}

	room = make(map[[2]string]int, len(pairs))

	if p.labels.rates {
		margin = rateMargin
	}

	for pair, links := range pairs {
		n := 0

		if p.labels.names != namesOff {
			ends := p.endNames(&clabinternaltuitopo.Edge{A: pair[0], B: pair[1], Links: links})
			a, b := clabinternaltuiui.Width(ends[0]), clabinternaltuiui.Width(ends[1])
			n += a + b + namesRoom
			margin = max(margin, a+1, b+1)
		}

		if p.labels.rates {
			n += rateRoom
		}

		if len(links) > 1 {
			n += badgeRoom
		}

		room[pair] = n
	}

	return room, margin
}

func (p *mapPage) panelVisible() bool {
	return !p.hidePanel && p.width >= mapPanelMinWidth
}

// viewSize returns the size of the area the map is drawn in.
func (p *mapPage) viewSize() (width, height int) {
	width = p.width
	if p.panelVisible() {
		width -= mapPanelWidth + 1
	}

	return width, p.height
}

func (p *mapPage) resize(width, height int) {
	changed := width != p.width || height != p.height
	first := p.width <= 0 || p.height <= 0
	p.basePage.resize(width, height)

	if p.layout == nil {
		return
	}

	switch {
	case changed && p.extent == extentAuto:
		// How much of the lab fits depends on the room there is.
		p.compute()
	case p.fit:
		p.fitZoom()
		p.reveal()
	case changed:
		// Whether the links get room for their texts depends on the room there is.
		p.layout = p.layoutAt(p.zoom)
		p.reveal()
	default:
		p.reveal()
	}

	if first {
		// The drawing was made before the size of the view was known.
		p.center()
	}
}

// reveal scrolls the map so that the focused node is in view.
func (p *mapPage) reveal() {
	if p.layout == nil {
		return
	}

	width, height := p.viewSize()
	total, totalHeight := p.layout.Size()

	if rect, ok := p.layout.NodeRect(p.node); ok {
		p.offsetX = min(p.offsetX, rect.X-mapMargin)
		p.offsetX = max(p.offsetX, rect.X+rect.W+mapMargin-width)
		p.offsetY = min(p.offsetY, rect.Y-mapMargin)
		p.offsetY = max(p.offsetY, rect.Y+rect.H+mapMargin-height)
	}

	p.offsetX = max(0, min(p.offsetX, total-width))
	p.offsetY = max(0, min(p.offsetY, totalHeight-height))
}

// center scrolls the map so that the focused node sits in the middle of the view. It is
// meant for a drawing that is new to the user; within a drawing, reveal moves the view no
// further than needed.
func (p *mapPage) center() {
	if p.layout == nil {
		return
	}

	if rect, ok := p.layout.NodeRect(p.node); ok {
		width, height := p.viewSize()
		p.offsetX = rect.X + rect.W/2 - width/2  //nolint:mnd // the middle of both
		p.offsetY = rect.Y + rect.H/2 - height/2 //nolint:mnd // the middle of both
	}

	p.pan(0, 0)
}

func (p *mapPage) pan(dx, dy int) {
	if p.layout == nil {
		return
	}

	width, height := p.viewSize()
	total, totalHeight := p.layout.Size()

	p.offsetX = max(0, min(p.offsetX+dx, total-width))
	p.offsetY = max(0, min(p.offsetY+dy, totalHeight-height))
}

func (p *mapPage) scroll(notches int) { p.pan(0, notches*2) } //nolint:mnd // rows per notch

func (p *mapPage) scrollSideways(notches int) { p.pan(notches*mapPanStep, 0) }

func (p *mapPage) click(m *Model, x, y int, double bool) {
	if p.layout == nil {
		return
	}

	width, _ := p.viewSize()
	if x >= width {
		return
	}

	hit := p.layout.Hit(x+p.offsetX, y+p.offsetY)

	switch {
	case hit.Node != "" && double:
		// A double click opens the node, like enter does.
		p.node, p.edge = hit.Node, noEdge

		if n := p.current(m); n != nil {
			m.push(newNodePage(m, n.NodeRef))
		}
	case hit.Node != "":
		p.focusOn(hit.Node)
	case hit.Edge >= 0:
		p.edge = hit.Edge
	}
}

// dragBy moves the map with the pointer: dragging to the left shows what is to the right.
func (p *mapPage) dragBy(dx, dy int) { p.pan(-dx, -dy) }

// current returns the focused node as the lab has it, nil for a node that is defined in
// the topology but does not run.
func (p *mapPage) current(m *Model) *clabinternaltuibackend.Node {
	lab := m.lab(p.lab)
	if lab == nil {
		return nil
	}

	return lab.Node(p.node)
}

// edgeLinks returns the links of the topology that the focused edge bundles.
func (p *mapPage) edgeLinks() []clabinternaltuitopo.Link {
	if p.layout == nil || p.edge < 0 {
		return nil
	}

	for _, e := range p.layout.Edges() {
		if e.ID != p.edge {
			continue
		}

		links := make([]clabinternaltuitopo.Link, 0, len(e.Links))
		for _, i := range e.Links {
			links = append(links, p.graph.Links[i])
		}

		return links
	}

	return nil
}

// edgeInterfaces returns the interfaces at the ends of the focused edge that exist on
// running nodes: the A ends only, or both ends.
func (p *mapPage) edgeInterfaces(both bool) []*clabinternaltuibackend.Interface {
	var list []*clabinternaltuibackend.Interface

	for _, l := range p.edgeLinks() {
		if iface := p.ifaces[[2]string{l.A, l.AIface}]; iface != nil {
			list = append(list, iface)
		}

		if iface := p.ifaces[[2]string{l.B, l.BIface}]; iface != nil && both {
			list = append(list, iface)
		}
	}

	return list
}

func (p *mapPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.layout == nil {
		return false, nil
	}

	if p.setting(m, msg.String()) || p.navigate(msg.String()) {
		return true, nil
	}

	switch msg.String() {
	case keyEnter:
		if n := p.current(m); n != nil && p.edge < 0 {
			m.push(newNodePage(m, n.NodeRef))
		}

		return true, nil
	case "/":
		return true, p.jumpMenu(m)
	case "p":
		p.hidePanel = !p.hidePanel
		p.resize(p.width, p.height)

		if p.extent == extentAuto {
			p.compute()
		}

		return true, nil
	}

	return false, nil
}

// actions implements actor: with a link in focus its actions, else the ones of the node.
func (p *mapPage) actions(m *Model) (string, []action) {
	if p.layout == nil {
		return "", nil
	}

	if p.edge >= 0 {
		links := p.edgeLinks()
		if len(links) == 0 {
			return "", nil
		}

		_, list := m.interfaceActions(p.edgeInterfaces(false))

		return links[0].A + " ↔ " + links[0].B, linkActions(m, list, p.edgeInterfaces(true))
	}

	n := p.current(m)
	if n == nil {
		return "", nil
	}

	return m.nodeActions(n, []*clabinternaltuibackend.Node{n})
}

// navigate handles the keys that move the focus and the view.
func (p *mapPage) navigate(key string) bool {
	switch key {
	// The arrows move the focus. The letters that move the cursor in lists are left to
	// the actions here, where l opens the logs as it does everywhere else.
	case "left":
		p.step(clabinternaltuitopo.DirLeft)
	case "right":
		p.step(clabinternaltuitopo.DirRight)
	case "up":
		p.step(clabinternaltuitopo.DirUp)
	case "down":
		p.step(clabinternaltuitopo.DirDown)
	case keyTab:
		p.cycleEdge(1)
	case "shift+tab":
		p.cycleEdge(-1)
	case "o":
		p.jumpToPeer()
	case "H", "shift+left":
		p.pan(-mapPanStep, 0)
	case "L", "shift+right":
		p.pan(mapPanStep, 0)
	case "K", "shift+up":
		p.pan(0, -mapPanStep/2) //nolint:mnd // rows are taller than columns are wide
	case "J", "shift+down":
		p.pan(0, mapPanStep/2) //nolint:mnd // see above
	default:
		return false
	}

	return true
}

// setting handles the keys that change how the map is drawn. Each of them says what it
// did: a change is not always visible, ie. when a lab fits the window at any zoom.
func (p *mapPage) setting(m *Model, key string) bool {
	info := clabinternaltuitheme.LevelInfo

	switch key {
	case "+", "=":
		if p.zoom == clabinternaltuitopo.ZoomDetail {
			m.notifyf(info, "this is the largest zoom")

			return true
		}

		p.setZoom(p.zoom + 1)
		m.notifyf(info, "zoom: %s", zoomName(p.zoom))
	case "-":
		if p.zoom == clabinternaltuitopo.ZoomCompact {
			m.notifyf(info, "this is the smallest zoom")

			return true
		}

		p.setZoom(p.zoom - 1)
		m.notifyf(info, "zoom: %s", zoomName(p.zoom))
	case "f":
		p.fit = true
		p.fitZoom()
		p.center()
		m.notifyf(info, "fitted to the window, zoom: %s", zoomName(p.zoom))
	case "v":
		p.cycleMode()
		m.notifyf(info, "layout: %s, drawn by %s",
			modeName(p.mode), modeName(p.layout.Mode()))
	case "e":
		p.cycleExtent()
		m.notifyf(info, "showing: %s — %d of %d nodes",
			p.extent, len(p.graph.Nodes), len(p.full.Nodes))
	case "r":
		p.labels.rates = !p.labels.rates
		p.relabel()
		m.notifyf(info, "traffic rates on the links: %s", onOff(p.labels.rates))
	case "i":
		p.labels.names = (p.labels.names + 1) % (namesOff + 1)
		p.relabel()
		m.notifyf(info, "interface names: %s", p.labels.names)
	default:
		return false
	}

	return true
}

func (p *mapPage) step(dir clabinternaltuitopo.Dir) {
	if next, ok := p.layout.Neighbor(p.node, dir); ok {
		p.focusOn(next)
	}
}

// cycleEdge moves the focus through the links of the focused node.
func (p *mapPage) cycleEdge(delta int) {
	edges := p.layout.EdgesOf(p.node)
	if len(edges) == 0 {
		return
	}

	pos := -1

	for i, id := range edges {
		if id == p.edge {
			pos = i
		}
	}

	switch {
	case pos < 0 && delta > 0:
		pos = 0
	case pos < 0:
		pos = len(edges) - 1
	default:
		pos += delta
	}

	if pos < 0 || pos >= len(edges) {
		// Past the last link the focus is back on the node itself.
		p.edge = noEdge

		return
	}

	p.edge = edges[pos]
}

// jumpToPeer moves the focus to the node at the other end of the focused link.
func (p *mapPage) jumpToPeer() {
	for _, e := range p.layout.Edges() {
		if e.ID != p.edge {
			continue
		}

		if e.A == p.node {
			p.focusOn(e.B)
		} else {
			p.focusOn(e.A)
		}

		return
	}
}

func (p *mapPage) setZoom(zoom clabinternaltuitopo.Zoom) {
	zoom = max(clabinternaltuitopo.ZoomCompact, min(zoom, clabinternaltuitopo.ZoomDetail))
	if zoom == p.zoom {
		return
	}

	p.zoom, p.fit = zoom, false
	p.layout = p.layoutAt(zoom)
	p.center()
}

// relabel draws the lab anew after a change of what is written on the links, which
// changes the room the links are given.
func (p *mapPage) relabel() {
	if p.layout == nil {
		return
	}

	p.layout = p.layoutAt(p.zoom)

	if p.fit {
		p.fitZoom()
	}

	if !p.hasEdge(p.edge) {
		p.edge = noEdge
	}

	p.reveal()
}

func (p *mapPage) cycleMode() {
	switch p.mode {
	case clabinternaltuitopo.ModeAuto:
		p.mode = clabinternaltuitopo.ModeTiers
	case clabinternaltuitopo.ModeTiers:
		p.mode = clabinternaltuitopo.ModeGraph
	case clabinternaltuitopo.ModeGraph:
		p.mode = clabinternaltuitopo.ModeSaved
	default:
		p.mode = clabinternaltuitopo.ModeAuto
	}

	p.compute()
	p.center()
}

// state gathers what is known about the nodes and links for the drawing.
func (p *mapPage) state(m *Model) clabinternaltuitopo.State {
	st := clabinternaltuitopo.State{
		Nodes:      map[string]clabinternaltuitopo.NodeState{},
		Links:      map[int]clabinternaltuitopo.LinkState{},
		EdgeLabels: map[int]string{},
		NodeNotes:  map[string]string{},
	}

	st.EndLabels = p.endLabels()

	lab := m.lab(p.lab)
	deployed := lab != nil && lab.Deployed

	for i := range p.graph.Nodes {
		id := p.graph.Nodes[i].ID
		st.Nodes[id] = clabinternaltuitopo.NodeUnknown

		if !deployed {
			continue
		}

		st.Nodes[id] = clabinternaltuitopo.NodeDown

		if n := lab.Node(id); n != nil {
			st.Nodes[id] = nodeMapState(n.Readiness())

			if n.HasStats {
				st.NodeNotes[id] = clabinternaltuiui.Percent(n.CPUPercent) + " " +
					clabinternaltuiui.Bytes(n.MemBytes)
			}
		}
	}

	if !deployed {
		return st
	}

	rates := map[int]float64{}

	for i := range p.graph.Links {
		l := &p.graph.Links[i]
		a, b := p.ifaces[[2]string{l.A, l.AIface}], p.ifaces[[2]string{l.B, l.BIface}]
		st.Links[i] = linkMapState(a, b)

		if a != nil && a.HasStats {
			rates[i] = max(a.RxBps, a.TxBps)
		} else if b != nil && b.HasStats {
			rates[i] = max(b.RxBps, b.TxBps)
		}
	}

	if p.labels.rates {
		for _, e := range p.layout.Edges() {
			var sum float64
			for _, i := range e.Links {
				sum += rates[i]
			}

			if sum >= 1 {
				st.EdgeLabels[e.ID] = clabinternaltuiui.Rate(sum)
			}
		}
	}

	return st
}

// endLabels returns the names of the interfaces to write at the ends of the links.
func (p *mapPage) endLabels() map[int][2]string {
	if p.labels.names == namesOff {
		return nil
	}

	named := map[int]bool{}

	if p.labels.names == namesFocus {
		for _, id := range p.layout.EdgesOf(p.node) {
			named[id] = true
		}
	}

	labels := map[int][2]string{}

	for _, e := range p.layout.Edges() {
		if p.labels.names == namesAll || named[e.ID] || e.ID == p.edge {
			labels[e.ID] = p.endNames(&e)
		}
	}

	return labels
}

// endNames returns the names of the interfaces at the two ends of an edge, the end at
// its first node first. An edge that stands for several links names them all when the
// names are short, and the first of them with their number otherwise.
func (p *mapPage) endNames(e *clabinternaltuitopo.Edge) [2]string {
	var a, b []string

	for _, i := range e.Links {
		l := &p.graph.Links[i]
		if l.A == e.A {
			a, b = append(a, l.AIface), append(b, l.BIface)
		} else {
			a, b = append(a, l.BIface), append(b, l.AIface)
		}
	}

	return [2]string{joinNames(a), joinNames(b)}
}

// joinedNamesWidth is the room a list of interface names may take at the end of a link.
const joinedNamesWidth = 12

func joinNames(names []string) string {
	if len(names) == 0 || names[0] == "" {
		return ""
	}

	if joined := strings.Join(names, ","); len(joined) <= joinedNamesWidth {
		return joined
	}

	return fmt.Sprintf("%s +%d", names[0], len(names)-1)
}

func onOff(on bool) string {
	if on {
		return "on"
	}

	return "off"
}

func nodeMapState(r clabinternaltuibackend.Readiness) clabinternaltuitopo.NodeState {
	switch r {
	case clabinternaltuibackend.Ready:
		return clabinternaltuitopo.NodeReady
	case clabinternaltuibackend.Starting, clabinternaltuibackend.Unhealthy,
		clabinternaltuibackend.Paused, clabinternaltuibackend.Stopping:
		return clabinternaltuitopo.NodeWarn
	case clabinternaltuibackend.Stopped, clabinternaltuibackend.Failed:
		return clabinternaltuitopo.NodeDown
	default:
		return clabinternaltuitopo.NodeUnknown
	}
}

func linkMapState(a, b *clabinternaltuibackend.Interface) clabinternaltuitopo.LinkState {
	switch {
	case a == nil || b == nil:
		// An end is missing when its node does not run.
		return clabinternaltuitopo.LinkDown
	case a.State == stateDown || b.State == stateDown:
		return clabinternaltuitopo.LinkDown
	case a.State == stateUp || b.State == stateUp:
		return clabinternaltuitopo.LinkUp
	default:
		return clabinternaltuitopo.LinkUnknown
	}
}

func (p *mapPage) view(m *Model) string {
	if p.layout == nil || len(p.graph.Nodes) == 0 {
		return p.empty(m)
	}

	width, height := p.viewSize()
	grid := p.painted(m)

	lines := make([]string, height)
	for y := range height {
		lines[y] = p.renderRow(m, grid, y+p.offsetY, width)
	}

	canvas := strings.Join(lines, "\n")
	if !p.panelVisible() {
		return canvas
	}

	separator := strings.TrimRight(strings.Repeat(m.th.Border.Render("│")+"\n", height), "\n")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		canvas,
		separator,
		p.panel(m, mapPanelWidth, height),
	)
}

// painted returns the drawing, painting it anew only when what it shows has changed:
// the layout, the focus or the state reported by the backend.
func (p *mapPage) painted(m *Model) *clabinternaltuitopo.Grid {
	want := mapPainted{
		layout: p.layout, node: p.node, edge: p.edge, labels: p.labels, revision: p.revision,
	}

	if p.grid == nil || p.gridFor != want {
		p.grid = p.layout.Paint(p.state(m),
			clabinternaltuitopo.Selection{Node: p.node, Edge: p.edge})
		p.gridFor = want
	}

	return p.grid
}

func (p *mapPage) empty(m *Model) string {
	text := "no lab to draw"

	switch {
	case p.topology.Warning != "":
		text = clabinternaltuiui.Sanitize(p.topology.Warning)
	case p.lab != "":
		text = "the topology of " + p.lab + " has no nodes"
	}

	return clabinternaltuiui.FitLines(
		"\n"+m.th.Muted.Render(clabinternaltuiui.Center(text, p.width)),
		p.width,
		p.height,
	)
}

// renderRow draws one row of the grid, joining neighboring cells of the same look into
// one styled run.
func (p *mapPage) renderRow(m *Model, grid *clabinternaltuitopo.Grid, y, width int) string {
	gridWidth, gridHeight := grid.Size()
	if y < 0 || y >= gridHeight {
		return strings.Repeat(" ", width)
	}

	var (
		b       strings.Builder
		run     []rune
		current lipgloss.Style
		started bool
	)

	flush := func() {
		if len(run) > 0 {
			b.WriteString(current.Render(string(run)))
			run = run[:0]
		}
	}

	for x := range width {
		cell := clabinternaltuitopo.Cell{Rune: ' '}
		if gx := x + p.offsetX; gx < gridWidth {
			cell = grid.Cell(gx, y)
		}

		if cell.Rune == 0 {
			cell.Rune = ' '
		}

		style := p.cellStyle(m, cell)
		if !started || !sameStyle(style, current) {
			flush()

			current, started = style, true
		}

		run = append(run, cell.Rune)
	}

	flush()

	return b.String()
}

func sameStyle(a, b lipgloss.Style) bool {
	return a.GetForeground() == b.GetForeground() && a.GetBold() == b.GetBold() &&
		a.GetReverse() == b.GetReverse() && a.GetFaint() == b.GetFaint()
}

// cellStyle maps what a cell is part of to how it looks.
func (*mapPage) cellStyle(m *Model, cell clabinternaltuitopo.Cell) lipgloss.Style {
	var style lipgloss.Style

	switch cell.Class {
	case clabinternaltuitopo.ClassNodeBorder:
		style = m.th.LevelStyle(nodeStateLevel(cell.State))
	case clabinternaltuitopo.ClassNodeLabel:
		style = m.th.Text
	case clabinternaltuitopo.ClassNodeNote:
		style = m.th.Muted
	case clabinternaltuitopo.ClassLinkUp:
		style = m.th.Success
	case clabinternaltuitopo.ClassLinkDown:
		style = m.th.Error
	case clabinternaltuitopo.ClassLinkMixed:
		style = m.th.Warning
	case clabinternaltuitopo.ClassLinkUnknown:
		style = m.th.Faint
	case clabinternaltuitopo.ClassEdgeLabel:
		style = m.th.Muted
	case clabinternaltuitopo.ClassEndLabel:
		style = m.th.Faint
	case clabinternaltuitopo.ClassBadge:
		style = m.th.Warning
	default:
		return lipgloss.NewStyle()
	}

	if cell.Selected {
		style = style.Foreground(m.th.Palette.Accent).Bold(true)
	}

	return style
}

func nodeStateLevel(s clabinternaltuitopo.NodeState) clabinternaltuitheme.Level {
	switch s {
	case clabinternaltuitopo.NodeReady:
		return clabinternaltuitheme.LevelSuccess
	case clabinternaltuitopo.NodeWarn:
		return clabinternaltuitheme.LevelWarning
	case clabinternaltuitopo.NodeDown:
		return clabinternaltuitheme.LevelError
	default:
		return clabinternaltuitheme.LevelMuted
	}
}

// panel describes what is in focus: a link, or a node with its links.
func (p *mapPage) panel(m *Model, width, height int) string {
	var lines []string

	if p.edge >= 0 {
		lines = p.edgePanel(m)
	} else {
		lines = p.nodePanel(m)
	}

	for i := range lines {
		lines[i] = " " + lines[i]
	}

	return clabinternaltuiui.FitLines(strings.Join(lines, "\n"), width, height)
}

func (p *mapPage) nodePanel(m *Model) []string {
	lines := []string{m.th.Title.Render(clabinternaltuiui.Sanitize(p.node))}

	n := p.current(m)
	if n == nil {
		for i := range p.topology.Nodes {
			if p.topology.Nodes[i].Name == p.node {
				lines = append(
					lines,
					m.th.Muted.Render(clabinternaltuiui.Sanitize(p.topology.Nodes[i].Kind)),
				)
			}
		}

		return append(lines, m.th.Muted.Render("not running"))
	}

	glyph, level := readinessLook(n.Readiness())
	lines = append(lines,
		m.th.LevelStyle(level).Render(glyph+" "+n.Readiness().String()),
		m.th.Text.Foreground(m.th.Hue(n.Kind)).Render(clabinternaltuiui.Sanitize(n.Kind)),
		m.th.Muted.Render(stripPrefixLen(n.IPv4)),
	)

	if n.HasStats {
		cpu := clabinternaltuiui.PadLeft(clabinternaltuiui.Percent(n.CPUPercent), panelValueWidth)
		mem := clabinternaltuiui.PadLeft(clabinternaltuiui.Bytes(n.MemBytes), panelValueWidth)
		history := clabinternaltuiui.Sparkline(
			n.CPUSeries, panelSparkWidth, cpuLimit(n.CPUSeries, 1))

		lines = append(lines,
			m.th.Muted.Render("CPU ")+m.th.LevelStyle(cpuLevel(n.CPUPercent)).Render(cpu)+" "+
				m.th.Muted.Render(history),
			m.th.Muted.Render("MEM ")+m.th.Text.Render(mem),
		)
	}

	lines = append(lines, "", m.th.Muted.Render("links"))

	return append(lines, p.nodeLinks(m)...)
}

// nodeLinks lists the links of the focused node with the state and the traffic of the
// node's end of each.
func (p *mapPage) nodeLinks(m *Model) []string {
	type row struct {
		iface, peer string
	}

	var rows []row

	for i := range p.graph.Links {
		l := &p.graph.Links[i]

		switch p.node {
		case l.A:
			rows = append(rows, row{l.AIface, l.B})
		case l.B:
			rows = append(rows, row{l.BIface, l.A})
		}
	}

	sort.SliceStable(rows, func(a, b int) bool {
		return clabinternaltuiui.NaturalLess(rows[a].iface, rows[b].iface)
	})

	const ifaceWidth, peerWidth = 9, 10

	lines := make([]string, 0, len(rows))

	for _, r := range rows {
		iface := p.ifaces[[2]string{p.node, r.iface}]

		state, rate := m.th.Faint.Render(glyphNone), ""

		if iface != nil {
			glyph, level := interfaceLook(iface.State)
			state = m.th.LevelStyle(level).Render(glyph)

			if iface.HasStats {
				rate = arrowRx + clabinternaltuiui.Rate(iface.RxBps) + " " +
					arrowTx + clabinternaltuiui.Rate(iface.TxBps)
			}
		}

		lines = append(lines, state+" "+
			m.th.Text.Render(clabinternaltuiui.PadRight(r.iface, ifaceWidth))+" "+
			m.th.Muted.Render(clabinternaltuiui.PadRight(r.peer, peerWidth))+" "+
			m.th.Muted.Render(rate))
	}

	return lines
}

func (p *mapPage) edgePanel(m *Model) []string {
	links := p.edgeLinks()
	if len(links) == 0 {
		return nil
	}

	lines := []string{
		m.th.Title.Render(clabinternaltuiui.Sanitize(links[0].A + " ↔ " + links[0].B)),
	}
	if len(links) > 1 {
		lines = append(lines, m.th.Muted.Render(fmt.Sprintf("%d parallel links", len(links))))
	}

	for _, l := range links {
		a, b := p.ifaces[[2]string{l.A, l.AIface}], p.ifaces[[2]string{l.B, l.BIface}]

		state, level := "unknown", clabinternaltuitheme.LevelMuted

		switch linkMapState(a, b) {
		case clabinternaltuitopo.LinkUp:
			state, level = stateUp, clabinternaltuitheme.LevelSuccess
		case clabinternaltuitopo.LinkDown:
			state, level = stateDown, clabinternaltuitheme.LevelError
		case clabinternaltuitopo.LinkMixed, clabinternaltuitopo.LinkUnknown:
		}

		lines = append(lines, "",
			m.th.Text.Render(clabinternaltuiui.Sanitize(l.A+":"+l.AIface)),
			m.th.Text.Render(clabinternaltuiui.Sanitize(l.B+":"+l.BIface)),
			m.th.LevelStyle(level).Render(state),
		)

		if a != nil && a.HasStats {
			lines = append(lines, m.th.Muted.Render(
				l.A+" → "+clabinternaltuiui.Rate(a.TxBps)+"   ← "+clabinternaltuiui.Rate(a.RxBps)))
		}

		for _, end := range []*clabinternaltuibackend.Interface{a, b} {
			if end != nil && !end.Netem.IsZero() {
				lines = append(lines, m.th.Warning.Render(
					end.Node.Name+": "+clabinternaltuiui.Netem(end.Netem)))
			}
		}
	}

	return lines
}

func (p *mapPage) hints(m *Model) []hint {
	if p.layout == nil {
		return []hint{{"1", "labs"}, {"?", "help"}}
	}

	hints := []hint{{"←↑↓→", "node"}, {keyTab, "link"}}

	switch {
	case p.edge >= 0:
		hints = append(hints, hint{"o", "peer"})
	case p.current(m) != nil:
		hints = append(hints, hint{"⏎", "open"})
	}

	// The actions of what is in focus come first, then how the map itself is driven.
	const shown = 3

	_, list := p.actions(m)
	if actions := actionHints(list); len(actions) > shown {
		hints = append(hints, actions[:shown]...)
	} else {
		hints = append(hints, actions...)
	}

	// What is written on the links comes before the zoom and the layout: a window too
	// narrow for all of the hints drops the last ones, and those are set less often.
	return append(hints,
		hint{keyActions, "actions"}, hint{"/", "go to"}, hint{"e", "extent"},
		hint{"i", "names"}, hint{"r", "rates"},
		hint{"+", "zoom in"}, hint{"-", "out"}, hint{"f", "fit"}, hint{"v", "layout"},
		hint{"p", "panel"}, hint{"?", "help"})
}
