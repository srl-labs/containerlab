// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// mapExtent tells how much of a lab the map draws. A drawing of a large lab is far wider
// than a terminal, so the map can leave out what matters least: first the hosts hanging
// off a single node, then everything that is not close to the node in focus.
type mapExtent int

const (
	// extentAuto draws as much as fits the view.
	extentAuto mapExtent = iota
	// extentAll draws every node.
	extentAll
	// extentCore folds the nodes with a single neighbor, typically hosts, into it.
	extentCore
	// extentNear draws the node in focus and what is within two hops of it.
	extentNear
	// extentAdjacent draws the node in focus and its neighbors.
	extentAdjacent
)

const (
	nearHops     = 2
	adjacentHops = 1
	// autoNodeLimit is the number of nodes above which a drawing is not even tried for
	// the automatic extent: it would not fit any terminal.
	autoNodeLimit = 150
	// minNeighbors is the least number of neighbors drawn around the node in focus.
	minNeighbors = 3
)

// mapScope is what a reduced drawing is centered on.
type mapScope struct {
	// focus is the node the drawing is about.
	focus string
	// anchor is the node the focus came from. Of the neighbors of a node that has more
	// than fit, the ones around the anchor are drawn.
	anchor string
	// limit is the number of neighbors drawn around the focus, zero for all of them.
	limit int
}

func (e mapExtent) String() string {
	switch e {
	case extentAll:
		return "all"
	case extentCore:
		return "no hosts"
	case extentNear:
		return "2 hops"
	case extentAdjacent:
		return "neighbors"
	default:
		return "auto"
	}
}

// local reports whether the extent depends on the node in focus.
func (e mapExtent) local() bool {
	return e == extentNear || e == extentAdjacent
}

// neighborSets returns the distinct neighbors of every node.
func neighborSets(g *clabinternaltuitopo.Graph) map[string]map[string]bool {
	sets := make(map[string]map[string]bool, len(g.Nodes))
	for i := range g.Nodes {
		sets[g.Nodes[i].ID] = map[string]bool{}
	}

	for i := range g.Links {
		a, b := g.Links[i].A, g.Links[i].B
		if a == b || sets[a] == nil || sets[b] == nil {
			continue
		}

		sets[a][b] = true
		sets[b][a] = true
	}

	return sets
}

// reduceGraph returns the part of a graph an extent covers. Nodes that stand for more
// than themselves say so in their label: "leaf1 +4" has four neighbors that are not drawn.
func reduceGraph(
	full *clabinternaltuitopo.Graph, extent mapExtent, scope mapScope,
) clabinternaltuitopo.Graph {
	neighbors := neighborSets(full)

	var keep map[string]bool

	switch extent {
	case extentCore:
		keep = withoutHosts(neighbors, scope.focus)
	case extentNear:
		keep = within(neighbors, scope.focus, nearHops)
	case extentAdjacent:
		keep = adjacent(neighbors, scope)
	case extentAuto, extentAll:
		return *full
	}

	var g clabinternaltuitopo.Graph

	for i := range full.Nodes {
		n := full.Nodes[i]
		if !keep[n.ID] {
			continue
		}

		hidden := 0

		for other := range neighbors[n.ID] {
			if !keep[other] {
				hidden++
			}
		}

		if hidden > 0 {
			n.Label = fmt.Sprintf("%s +%d", n.ID, hidden)
		}

		g.Nodes = append(g.Nodes, n)
	}

	for i := range full.Links {
		if keep[full.Links[i].A] && keep[full.Links[i].B] {
			g.Links = append(g.Links, full.Links[i])
		}
	}

	return g
}

// withoutHosts returns the nodes that stay when hosts are folded away: a host is a node
// with a single neighbor that itself has more than one. The node in focus always stays.
func withoutHosts(neighbors map[string]map[string]bool, focus string) map[string]bool {
	keep := make(map[string]bool, len(neighbors))

	for id, set := range neighbors {
		host := false

		if len(set) == 1 {
			for other := range set {
				host = len(neighbors[other]) > 1
			}
		}

		keep[id] = !host || id == focus
	}

	return keep
}

// within returns the nodes at most hops links away from the start.
func within(neighbors map[string]map[string]bool, start string, hops int) map[string]bool {
	keep := map[string]bool{}
	if neighbors[start] == nil {
		return keep
	}

	keep[start] = true
	frontier := []string{start}

	for range hops {
		var next []string

		for _, id := range frontier {
			for other := range neighbors[id] {
				if !keep[other] {
					keep[other] = true

					next = append(next, other)
				}
			}
		}

		frontier = next
	}

	return keep
}

// adjacent returns the node in focus and its neighbors. A node with more neighbors than
// the limit allows, such as a spine of a large fabric, is drawn with the stretch of them
// around the anchor: coming from leaf100, that is the leaves next to it.
func adjacent(neighbors map[string]map[string]bool, scope mapScope) map[string]bool {
	keep := map[string]bool{}
	if neighbors[scope.focus] == nil {
		return keep
	}

	keep[scope.focus] = true

	names := make([]string, 0, len(neighbors[scope.focus]))
	for name := range neighbors[scope.focus] {
		names = append(names, name)
	}

	sort.SliceStable(names, func(a, b int) bool {
		return clabinternaltuiui.NaturalLess(names[a], names[b])
	})

	if scope.limit > 0 && len(names) > scope.limit {
		start := 0

		for i, name := range names {
			if name == scope.anchor {
				start = i - scope.limit/2 //nolint:mnd // the anchor in the middle
			}
		}

		start = max(0, min(start, len(names)-scope.limit))
		names = names[start : start+scope.limit]
	}

	for _, name := range names {
		keep[name] = true
	}

	return keep
}

// busiestNode returns the node with the most neighbors, the natural place to start
// looking at a lab from. Ties go to the name that sorts first.
func busiestNode(g *clabinternaltuitopo.Graph) string {
	neighbors := neighborSets(g)
	best := ""

	for i := range g.Nodes {
		id := g.Nodes[i].ID

		switch {
		case best == "", len(neighbors[id]) > len(neighbors[best]):
			best = id
		case len(neighbors[id]) == len(neighbors[best]) && clabinternaltuiui.NaturalLess(id, best):
			best = id
		}
	}

	return best
}

// autoExtent picks the widest extent whose drawing fits the view at the smallest zoom. A
// lab too large for that falls back to the neighbors of the node in focus, which is the
// one drawing that stays readable whatever the size of the lab.
func (p *mapPage) autoExtent() mapExtent {
	width, height := p.viewSize()
	if width <= 0 || height <= 0 {
		return extentAll
	}

	for _, extent := range []mapExtent{extentAll, extentCore, extentNear} {
		g := reduceGraph(&p.full, extent, mapScope{focus: p.node})
		if len(g.Nodes) > autoNodeLimit {
			continue
		}

		if p.fits(&g, clabinternaltuitopo.ZoomCompact) {
			return extent
		}
	}

	return extentAdjacent
}

// fits reports whether the drawing of a graph fits the view at the given zoom.
func (p *mapPage) fits(g *clabinternaltuitopo.Graph, zoom clabinternaltuitopo.Zoom) bool {
	width, height := p.viewSize()

	w, h := clabinternaltuitopo.Compute(*g, clabinternaltuitopo.Options{
		Mode: p.mode, Zoom: zoom,
	}).Size()

	return w <= width && h <= height
}

// scope returns what the drawing of an extent is centered on. For the neighbors of the
// node in focus it works out how many of them fit the view.
func (p *mapPage) scope(extent mapExtent) mapScope {
	scope := mapScope{focus: p.node, anchor: p.anchor}
	if extent != extentAdjacent {
		return scope
	}

	if width, height := p.viewSize(); width <= 0 || height <= 0 {
		return scope
	}

	scope.limit = len(neighborSets(&p.full)[p.node])

	// Fewer neighbors with their names in full read better than more of them cut
	// short, so the count is fitted to the regular zoom, not the smallest one.
	for scope.limit > minNeighbors {
		g := reduceGraph(&p.full, extentAdjacent, scope)
		if p.fits(&g, clabinternaltuitopo.ZoomNormal) {
			break
		}

		scope.limit = max(minNeighbors, scope.limit*3/4) //nolint:mnd // shrink by a quarter
	}

	return scope
}

// cycleExtent switches to the next extent.
func (p *mapPage) cycleExtent() {
	p.extent = (p.extent + 1) % (extentAdjacent + 1)
	p.fit = true
	p.compute()
	p.center()
}

// focusOn moves the focus to a node. Extents that depend on the focus are drawn anew
// around it, and so is a drawing that does not show the node.
func (p *mapPage) focusOn(node string) {
	if node != p.node {
		p.anchor = p.node
	}

	p.node, p.edge = node, noEdge

	if _, shown := p.layout.NodeRect(node); !shown || p.shown.local() {
		p.compute()
		p.center()

		return
	}

	p.reveal()
}

// jumpMenu lets the user go to any node of the lab by name, drawn or not.
func (p *mapPage) jumpMenu(m *Model) tea.Cmd {
	names := make([]string, 0, len(p.full.Nodes))
	for i := range p.full.Nodes {
		names = append(names, p.full.Nodes[i].ID)
	}

	sort.SliceStable(names, func(a, b int) bool {
		return clabinternaltuiui.NaturalLess(names[a], names[b])
	})

	kinds := map[string]string{}
	for i := range p.full.Nodes {
		kinds[p.full.Nodes[i].ID] = p.full.Nodes[i].Kind
	}

	items := make([]menuItem, 0, len(names))

	for _, name := range names {
		items = append(items, menuItem{
			label: name,
			desc:  kinds[name],
			run: func() tea.Cmd {
				p.focusOn(name)
				p.center()

				return nil
			},
		})
	}

	return m.choose("Go to node", items)
}
