// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"fmt"
	"math/rand"
	"strings"
)

// builder assembles test graphs.
type builder struct{ g Graph }

func (b *builder) nodes(prefix string, n int) []string {
	ids := make([]string, n)

	for i := range ids {
		ids[i] = fmt.Sprintf("%s%d", prefix, i+1)
		b.g.Nodes = append(b.g.Nodes, Node{ID: ids[i]})
	}

	return ids
}

func (b *builder) link(a, z string) {
	n := len(b.g.Links)
	b.g.Links = append(b.g.Links, Link{
		A: a, B: z, AIface: fmt.Sprintf("e1-%d", n+1), BIface: fmt.Sprintf("e1-%d", n+1),
	})
}

// mesh links every node of one group to every node of the other.
func (b *builder) mesh(upper, lower []string) {
	for _, u := range upper {
		for _, d := range lower {
			b.link(u, d)
		}
	}
}

func pair() Graph {
	var b builder

	n := b.nodes("srl", 2)
	b.link(n[0], n[1])

	return b.g
}

func clos(spines, leaves int) Graph {
	var b builder

	b.mesh(b.nodes("spine", spines), b.nodes("leaf", leaves))

	return b.g
}

// threeTier is a 2x4 fabric with one client per leaf, the second on two links.
func threeTier() Graph {
	var b builder

	leaves := b.nodes("leaf", 4)
	b.mesh(b.nodes("spine", 2), leaves)

	for i, c := range b.nodes("client", 4) {
		b.link(leaves[i], c)
	}

	b.link("leaf2", "client2")

	return b.g
}

func ring(n int) Graph {
	var b builder

	ids := b.nodes("r", n)
	for i := range ids {
		b.link(ids[i], ids[(i+1)%n])
	}

	return b.g
}

func fullMesh(n int) Graph {
	var b builder

	ids := b.nodes("pe", n)
	for i := range ids {
		for j := i + 1; j < n; j++ {
			b.link(ids[i], ids[j])
		}
	}

	return b.g
}

func star(leaves int) Graph {
	var b builder

	b.g.Nodes = append(b.g.Nodes, Node{ID: "hub"})
	for _, n := range b.nodes("n", leaves) {
		b.link("hub", n)
	}

	return b.g
}

func parallel() Graph {
	var b builder

	n := b.nodes("leaf", 2)
	s := b.nodes("spine", 1)
	b.link(s[0], n[0])
	b.link(s[0], n[0])
	b.link(s[0], n[0])
	b.link(s[0], n[1])
	b.link(n[0], n[1])
	b.link(n[0], n[1])

	return b.g
}

// longEdge is a three tier lab where a client also attaches straight to a spine.
func longEdge() Graph {
	g := threeTier()
	g.Links = append(g.Links, Link{A: "spine1", B: "client3"})

	return g
}

func twoComponents() Graph {
	var b builder

	b.mesh(b.nodes("spine", 2), b.nodes("leaf", 3))

	pe := b.nodes("pe", 2)
	ce := b.nodes("ce", 2)
	b.link(pe[0], pe[1])
	b.link(pe[0], ce[0])
	b.link(pe[1], ce[1])
	b.g.Nodes = append(b.g.Nodes, Node{ID: "mgmt"}, Node{ID: "tester"})

	return b.g
}

// links adds one link per pair of names in "a-b" form.
func (b *builder) links(pairs ...string) {
	for _, p := range pairs {
		ends := strings.SplitN(p, "-", 2)
		b.link(ends[0], ends[1])
	}
}

// serviceProvider is a meshed P core with route reflectors, PEs and CEs. Nodes
// carry nothing but their names.
func serviceProvider() Graph {
	var b builder

	b.nodes("p", 4)
	b.nodes("rr", 2)
	b.nodes("pe", 4)
	b.nodes("ce", 4)
	b.links("p1-p2", "p1-p3", "p2-p4", "p3-p4", "p1-p4", "p2-p3", "rr1-p1", "rr2-p4",
		"pe1-p1", "pe1-p2", "pe2-p1", "pe2-p2", "pe3-p3", "pe3-p4", "pe4-p3", "pe4-p4",
		"ce1-pe1", "ce2-pe1", "ce2-pe2", "ce3-pe3", "ce4-pe4")

	return b.g
}

// metroRing is a ring of eight routers, with a customer router on every other one
// when stubs is set.
func metroRing(stubs bool) Graph {
	var b builder

	r := b.nodes("r", 8)
	for i := range r {
		b.link(r[i], r[(i+1)%len(r)])
	}

	if stubs {
		b.nodes("cpe", 4)
		b.links("cpe1-r2", "cpe2-r4", "cpe3-r6", "cpe4-r8")
	}

	return b.g
}

// campus is a firewall in front of two cores, four distribution and six access
// switches.
func campus() Graph {
	var b builder

	b.g.Nodes = append(b.g.Nodes, Node{ID: "internet"})
	b.nodes("fw", 1)

	cores := b.nodes("core", 2)
	dists := b.nodes("dist", 4)
	accs := b.nodes("acc", 6)

	b.links("internet-fw1", "fw1-core1", "fw1-core2", "core1-core2")
	b.mesh(cores, dists)
	b.mesh(dists[:2], accs[:3])
	b.mesh(dists[2:], accs[3:])

	return b.g
}

// leafSpine is a fabric with explicit levels where every leaf connects to every
// spine and leaf labels carry a count, as a view that folds hosts into their leaf
// would show them.
func leafSpine(spines, leaves int) Graph {
	var b builder

	b.mesh(b.nodes("spine", spines), b.nodes("leaf", leaves))

	for i := range b.g.Nodes {
		n := &b.g.Nodes[i]
		n.Level = 1

		if strings.HasPrefix(n.ID, "leaf") {
			n.Level = 2
			n.Label = n.ID + " +4"
		}
	}

	return b.g
}

// leafHosts is a fabric with hosts spread over the leaves.
func leafHosts(spines, leaves, hosts int) Graph {
	var b builder

	l := b.nodes("leaf", leaves)
	b.mesh(b.nodes("spine", spines), l)

	for i, h := range b.nodes("host", hosts) {
		b.link(l[i%leaves], h)
	}

	return b.g
}

// fatTree has 4 cores, 8 pods of 2 aggregation and 2 edge switches, and 4 hosts per
// edge switch: 100 nodes.
func fatTree() Graph {
	var b builder

	cores := b.nodes("core", 4)
	aggs := b.nodes("agg", 16)
	edges := b.nodes("tor", 16)
	hosts := b.nodes("host", 64)

	for i, a := range aggs {
		half := cores[:2]
		if i%2 == 1 {
			half = cores[2:]
		}

		for _, c := range half {
			b.link(c, a)
		}
	}

	for pod := range 8 {
		b.mesh(aggs[2*pod:2*pod+2], edges[2*pod:2*pod+2])
	}

	for i, h := range hosts {
		b.link(edges[i/4], h)
	}

	return b.g
}

// fanOut has more links on one side of a node than a box has room for.
func fanOut(n int) Graph {
	var b builder

	b.mesh(b.nodes("spine", 2), b.nodes("leaf", n))

	return b.g
}

// random builds a graph with opaque node names, so no layout rule is favored.
func random(r *rand.Rand, nodes, links int) Graph {
	var b builder

	ids := b.nodes("n", nodes)
	for range links {
		b.link(ids[r.Intn(nodes)], ids[r.Intn(nodes)])
	}

	return b.g
}

// shuffled returns the same graph with nodes and links in another order.
func shuffled(g Graph, r *rand.Rand) Graph {
	out := Graph{
		Nodes: append([]Node(nil), g.Nodes...),
		Links: append([]Link(nil), g.Links...),
	}

	r.Shuffle(
		len(out.Nodes),
		func(i, j int) { out.Nodes[i], out.Nodes[j] = out.Nodes[j], out.Nodes[i] },
	)
	r.Shuffle(
		len(out.Links),
		func(i, j int) { out.Links[i], out.Links[j] = out.Links[j], out.Links[i] },
	)

	for i := range out.Links {
		if r.Intn(2) == 0 {
			l := &out.Links[i]
			l.A, l.B, l.AIface, l.BIface = l.B, l.A, l.BIface, l.AIface
		}
	}

	return out
}

// withKinds sets the kind of every node: a network OS, or linux for the clients.
func withKinds(g Graph) Graph {
	for i := range g.Nodes {
		g.Nodes[i].Kind = "nokia_srlinux"
		if strings.HasPrefix(g.Nodes[i].ID, "client") {
			g.Nodes[i].Kind = "linux"
		}
	}

	return g
}

// allUp marks every link as up, which draws solid lines.
func allUp(g Graph) State {
	st := State{Links: map[int]LinkState{}}
	for i := range g.Links {
		st.Links[i] = LinkUp
	}

	return st
}
