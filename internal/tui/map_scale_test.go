// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuitopo "github.com/srl-labs/containerlab/internal/tui/topo"
)

// leafSpine builds a leaf-spine fabric with a number of hosts behind every leaf.
func leafSpine(spines, leaves, hostsPerLeaf int) clabinternaltuitopo.Graph {
	var g clabinternaltuitopo.Graph

	for s := range spines {
		g.Nodes = append(g.Nodes, clabinternaltuitopo.Node{
			ID: fmt.Sprintf("spine%d", s+1), Role: "spine", Level: 1,
		})
	}

	for l := range leaves {
		leaf := fmt.Sprintf("leaf%d", l+1)
		g.Nodes = append(g.Nodes, clabinternaltuitopo.Node{ID: leaf, Role: "leaf", Level: 2})

		for s := range spines {
			g.Links = append(g.Links, clabinternaltuitopo.Link{
				A: leaf, B: fmt.Sprintf("spine%d", s+1),
				AIface: fmt.Sprintf("e1-%d", 49+s), BIface: fmt.Sprintf("e1-%d", l+1),
			})
		}

		for h := range hostsPerLeaf {
			host := fmt.Sprintf("h%d-%d", l+1, h+1)
			g.Nodes = append(g.Nodes, clabinternaltuitopo.Node{ID: host, Role: "server", Level: 3})
			g.Links = append(g.Links, clabinternaltuitopo.Link{
				A: host, B: leaf, AIface: "eth1", BIface: fmt.Sprintf("e1-%d", h+1),
			})
		}
	}

	return g
}

// TestMapScale reports how large the drawing of big fabrics gets and how long it takes.
func TestMapScale(t *testing.T) {
	for _, size := range []struct{ spines, leaves, hosts int }{
		{2, 4, 1}, {4, 16, 2}, {4, 32, 4}, {8, 64, 4}, {16, 128, 8},
	} {
		g := leafSpine(size.spines, size.leaves, size.hosts)

		for _, zoom := range []clabinternaltuitopo.Zoom{
			clabinternaltuitopo.ZoomNormal, clabinternaltuitopo.ZoomCompact,
		} {
			start := time.Now()
			layout := clabinternaltuitopo.Compute(g, clabinternaltuitopo.Options{
				Zoom: zoom,
			})
			computed := time.Since(start)

			start = time.Now()

			layout.Paint(clabinternaltuitopo.State{}, clabinternaltuitopo.Selection{
				Edge: -1,
			})

			painted := time.Since(start)

			w, h := layout.Size()
			t.Logf("%4d nodes %5d links zoom %d: %5d x %-4d cells, layout %8s, paint %8s",
				len(g.Nodes), len(g.Links), zoom, w, h,
				computed.Round(10*time.Microsecond), painted.Round(10*time.Microsecond))
		}
	}
}

// newFabricModel returns a UI on a demo backend with a large fabric, on its map.
func newFabricModel(t *testing.T, leaves, width, height int) (*Model, *mapPage) {
	t.Helper()

	be := clabinternaltuibackendfake.New(clabinternaltuibackendfake.Options{
		Now: fixedClock, Seed: 1, FabricLeaves: leaves,
	})
	m := New(be, Options{Now: fixedClock, Home: "/home/demo", Lab: "fabric"})
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	press(m, "5")

	page, ok := m.top().(*mapPage)
	if !ok {
		t.Fatalf("expected the map, got %T", m.top())
	}

	return m, page
}

// TestMapOfLargeLabStaysReadable opens the map of labs of growing size: whatever the
// size, the automatic extent must come up with a drawing that fits the window.
func TestMapOfLargeLabStaysReadable(t *testing.T) {
	for _, leaves := range []int{2, 8, 32, 128} {
		m, page := newFabricModel(t, leaves, 120, 35)

		width, height := page.viewSize()
		w, h := page.layout.Size()

		if w > width || h > height {
			t.Errorf("%d leaves: the drawing is %dx%d, the view %dx%d (extent %v)",
				leaves, w, h, width, height, page.shown)
		}

		if _, ok := page.layout.NodeRect(page.node); !ok {
			t.Errorf("%d leaves: the node in focus (%s) is not drawn", leaves, page.node)
		}

		assertScreenFits(t, m, m.render())
		t.Logf("%3d leaves, %4d nodes: extent %-9v draws %3d nodes in %dx%d",
			leaves, len(page.full.Nodes), page.shown, len(page.graph.Nodes), w, h)
	}
}

func TestMapFoldsHostsAndCountsThem(t *testing.T) {
	_, page := newFabricModel(t, 8, 120, 35)

	page.extent = extentCore
	page.compute()

	// Four spines and eight leaves stay, the hosts are folded into their leaf.
	if got := len(page.graph.Nodes); got != 12 {
		t.Fatalf("%d nodes are drawn without hosts, want 12", got)
	}

	for i := range page.graph.Nodes {
		n := &page.graph.Nodes[i]
		if n.Role == "leaf" && n.Label != n.ID+" +4" {
			t.Errorf("%s is labeled %q, want it to count its four hosts", n.ID, n.Label)
		}

		if n.Role == "spine" && n.Label != "" {
			t.Errorf("%s hides nothing but is labeled %q", n.ID, n.Label)
		}
	}
}

// TestMapNeighborhoodFollowsTheFocus moves through a lab too large to draw as a whole:
// the drawing must follow the focus, and any node must be reachable by name.
func TestMapNeighborhoodFollowsTheFocus(t *testing.T) {
	m, page := newFabricModel(t, 128, 120, 35)

	if !page.shown.local() {
		t.Fatalf("a lab of %d nodes is drawn with extent %v", len(page.full.Nodes), page.shown)
	}

	press(m, "/")
	typeText(m, "h100-3")
	press(m, "enter")

	if page.node != "h100-3" {
		t.Fatalf("the focus is on %s after jumping to h100-3", page.node)
	}

	if _, ok := page.layout.NodeRect("leaf100"); !ok {
		t.Error("the leaf of the host in focus is not drawn")
	}

	// Stepping to the leaf redraws around it, which brings its other hosts in.
	press(m, "up")

	if page.node != "leaf100" {
		t.Fatalf("moving up from the host landed on %s", page.node)
	}

	if _, ok := page.layout.NodeRect("h100-1"); !ok {
		t.Error("the hosts of the leaf in focus are not drawn")
	}

	// The whole lab is one key away, and can be panned through.
	page.extent = extentAll
	page.compute()

	if got := len(page.graph.Nodes); got != len(page.full.Nodes) {
		t.Errorf("the extent all draws %d of %d nodes", got, len(page.full.Nodes))
	}

	assertScreenFits(t, m, m.render())
}

func TestMapExtentCycles(t *testing.T) {
	m, page := newFabricModel(t, 8, 120, 35)

	seen := map[mapExtent]bool{page.extent: true}

	for range 5 {
		press(m, "e")

		seen[page.extent] = true

		assertScreenFits(t, m, m.render())
	}

	if len(seen) != 5 {
		t.Errorf("the extent key went through %d extents, want 5", len(seen))
	}
}

// BenchmarkMapRedraw measures drawing the map of a large lab again without changes, as
// happens on every key press.
func BenchmarkMapRedraw(b *testing.B) {
	be := clabinternaltuibackendfake.New(clabinternaltuibackendfake.Options{
		Now: fixedClock, Seed: 1, FabricLeaves: 128,
	})
	m := New(be, Options{Now: fixedClock, Lab: "fabric"})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	press(m, "5")

	page := m.top().(*mapPage)
	page.extent = extentAll
	page.compute()

	b.ResetTimer()

	for range b.N {
		m.render()
	}
}

// TestScreenMapLargeLab draws the map of a lab far too large for the window: it shows the
// surroundings of the node in focus and says how much it leaves out.
func TestScreenMapLargeLab(t *testing.T) {
	m, page := newFabricModel(t, 32, 120, 35)
	snapshot(t, m, "map_fabric_hub_120x35")

	press(m, "/")
	typeText(m, "leaf17")
	press(m, "enter")

	if page.node != "leaf17" {
		t.Fatalf("the focus is on %s", page.node)
	}

	snapshot(t, m, "map_fabric_leaf_120x35")
}

// TestScreenMapExtents draws labs of growing size on a large terminal, in each of the
// extents the map offers.
func TestScreenMapExtents(t *testing.T) {
	// 68 nodes: the whole lab does not fit, without its hosts it does. The extents are
	// stepped through with the key, the way they are reached at the keyboard.
	m, page := newFabricModel(t, 12, 200, 50)
	press(m, "e", "e")

	if page.extent != extentCore {
		t.Fatalf("two steps led to the extent %q", page.extent)
	}

	snapshot(t, m, "map_medium_no_hosts_200x50")

	// 164 nodes, as a whole: a strip to pan through.
	m, page = newFabricModel(t, 32, 200, 50)
	press(m, "e")
	snapshot(t, m, "map_large_all_200x50")

	press(m, "e")
	snapshot(t, m, "map_large_no_hosts_200x50")

	press(m, "/")
	typeText(m, "leaf17")
	press(m, "enter", "e")

	if page.extent != extentNear || page.node != "leaf17" {
		t.Fatalf("the map shows %q around %s", page.extent, page.node)
	}

	snapshot(t, m, "map_large_two_hops_200x50")

	press(m, "e", "e")

	if page.extent != extentAuto {
		t.Fatalf("the extents do not come round to auto: %q", page.extent)
	}

	press(m, "/")
	typeText(m, "h9")
	snapshot(t, m, "map_large_goto_200x50")
}
