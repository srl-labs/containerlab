// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "sort"

const (
	// splitStar is the number of leaves from which a star is drawn with leaves on
	// both sides of the hub. Fewer fit in one row under it.
	splitStar = 6
	// minShape is the fewest nodes that can form a ring, a star or a mesh.
	minShape = 3
)

// networkKinds are attachment points rather than devices: a cloud or bridge hanging
// off a core node is upstream of it.
var networkKinds = map[string]bool{
	"host": true, "mgmt-net": true, "macvlan": true, "vxlan": true,
	"vxlan-stitch": true, "dummy": true, "bridge": true, "ovs-bridge": true,
}

// graphRows derives rows from the shape of a component alone. The second result is
// true when the order within the rows is part of the shape and must be kept.
//
// A ring, star or mesh is still one with stubs hanging off it, such as the customer
// routers on a metro ring. So the shape is looked for in what remains without the
// nodes that have a single link, and those are put back on the outer side of the
// node they attach to.
func (l *Layout) graphRows(comp []int) (rows [][]int, fixed bool) {
	var body []int

	for _, n := range comp {
		if len(l.nodes[n].adj) > 1 {
			body = append(body, n)
		}
	}

	if len(body) >= minShape && len(body) < len(comp) {
		if rows, fixed, ok := l.shapeRows(body); ok {
			return l.addStubs(rows, comp), fixed
		}
	}

	if rows, fixed, ok := l.shapeRows(comp); ok {
		return rows, fixed
	}

	return l.layerRows(comp), false
}

// shapeRows recognizes a ring, a mesh or a star among the given nodes, looking only
// at the links between them.
func (l *Layout) shapeRows(nodes []int) (rows [][]int, fixed, ok bool) {
	n := len(nodes)
	if n < minShape {
		return nil, false, false
	}

	in := make(map[int]bool, n)
	for _, i := range nodes {
		in[i] = true
	}

	minDeg, maxDeg, sum := n, 0, 0

	for _, i := range nodes {
		d := len(l.within(i, in))
		minDeg, maxDeg = min(minDeg, d), max(maxDeg, d)
		sum += d
	}

	edges := sum / 2 //nolint:mnd // every edge is counted at both ends

	switch {
	case minDeg == 2 && maxDeg == 2 && edges == n:
		return l.ringRows(nodes, in), true, true
	case edges == n*(n-1)/2:
		// A clique has no hierarchy: two rows keep it compact.
		return [][]int{nodes[:(n+1)/2], nodes[(n+1)/2:]}, false, true
	case minDeg == maxDeg && edges >= n:
		// A uniform mesh has no natural root either, so any node will do.
		return l.bfsRows(nodes, nodes[:1], in, false), false, true
	case n > minShape && minDeg == 1 && maxDeg == n-1 && edges == n-1:
		// With two leaves it would be a chain, which reads better in layers.
		return l.starRows(nodes, in), false, true
	}

	return nil, false, false
}

// within returns the neighbors of a node that belong to a set.
func (l *Layout) within(n int, in map[int]bool) []int {
	var out []int

	for _, nb := range l.nodes[n].adj {
		if in[nb] {
			out = append(out, nb)
		}
	}

	return out
}

// ringRows walks the ring and folds it in two rows, so every link stays between
// neighbors and only the two at the ends run between the rows.
func (l *Layout) ringRows(nodes []int, in map[int]bool) [][]int {
	order := []int{nodes[0]}
	seen := map[int]bool{nodes[0]: true}

	for len(order) < len(nodes) {
		next := -1

		for _, nb := range l.within(order[len(order)-1], in) {
			if !seen[nb] && (next < 0 || l.rank[nb] < l.rank[next]) {
				next = nb
			}
		}

		if next < 0 {
			break
		}

		seen[next] = true
		order = append(order, next)
	}

	half := (len(order) + 1) / 2 //nolint:mnd // top row takes the larger half
	top := order[:half]
	bottom := make([]int, 0, len(order)-half)

	for i := len(order) - 1; i >= half; i-- {
		bottom = append(bottom, order[i])
	}

	return [][]int{top, bottom}
}

func (l *Layout) starRows(nodes []int, in map[int]bool) [][]int {
	var (
		hub    int
		leaves []int
	)

	for _, n := range nodes {
		if len(l.within(n, in)) > 1 {
			hub = n
		} else {
			leaves = append(leaves, n)
		}
	}

	if len(leaves) < splitStar {
		return [][]int{{hub}, leaves}
	}

	half := (len(leaves) + 1) / 2 //nolint:mnd // upper row takes the larger half

	return [][]int{leaves[:half], {hub}, leaves[half:]}
}

// addStubs puts the nodes left out of a shape back, each on the outer side of the
// row its only neighbor is in: above for the upper rows, below for the others.
// Within a row they follow the order of the nodes they attach to.
func (l *Layout) addStubs(rows [][]int, comp []int) [][]int {
	type place struct{ row, pos int }

	at := map[int]place{}

	for r, row := range rows {
		for i, n := range row {
			at[n] = place{r, i}
		}
	}

	// One spare row at each end; the empty ones are dropped afterwards.
	out := make([][]int, len(rows)+2) //nolint:mnd // above and below
	copy(out[1:], rows)

	var stubs []int

	for _, n := range comp {
		if _, placed := at[n]; !placed {
			stubs = append(stubs, n)
		}
	}

	parent := func(n int) place { return at[l.nodes[n].adj[0]] }

	sort.SliceStable(
		stubs,
		func(i, j int) bool { return parent(stubs[i]).pos < parent(stubs[j]).pos },
	)

	for _, n := range stubs {
		row := parent(n).row + 2 //nolint:mnd // below, in the shifted numbering
		if parent(n).row < len(rows)/2 {
			row = parent(n).row
		}

		out[row] = append(out[row], n)
	}

	kept := out[:0]

	for _, row := range out {
		if len(row) > 0 {
			kept = append(kept, row)
		}
	}

	return kept
}

// layerRows handles everything with a top and a bottom: trees, chains, spine-leaf.
// The best connected nodes are the roots, unless an attachment point hangs off one.
func (l *Layout) layerRows(comp []int) [][]int {
	var cores, upstream []int

	maxDeg := 0
	for _, n := range comp {
		maxDeg = max(maxDeg, len(l.nodes[n].adj))
	}

	isCore := map[int]bool{}
	in := map[int]bool{}

	for _, n := range comp {
		in[n] = true

		if len(l.nodes[n].adj) == maxDeg {
			cores = append(cores, n)
			isCore[n] = true
		}
	}

	for _, n := range comp {
		if isCore[n] || !networkKinds[l.nodes[n].Kind] {
			continue
		}

		for _, nb := range l.nodes[n].adj {
			if isCore[nb] {
				upstream = append(upstream, n)
				break
			}
		}
	}

	if len(upstream) > 0 {
		return l.bfsRows(comp, upstream, in, true)
	}

	return l.bfsRows(comp, cores, in, true)
}

// bfsRows layers the nodes by distance from the roots, following only the links
// among them.
func (l *Layout) bfsRows(nodes, roots []int, in map[int]bool, sink bool) [][]int {
	layer := make(map[int]int, len(nodes))

	queue := append([]int(nil), roots...)

	for _, r := range roots {
		layer[r] = 0
	}

	for head := 0; head < len(queue); head++ {
		cur := queue[head]

		for _, nb := range l.within(cur, in) {
			if _, ok := layer[nb]; !ok {
				layer[nb] = layer[cur] + 1
				queue = append(queue, nb)
			}
		}
	}

	if sink {
		l.sinkLeaves(nodes, layer)
	}

	depth := 0
	for _, d := range layer {
		depth = max(depth, d)
	}

	byLayer := make([][]int, depth+1)
	for _, n := range nodes {
		byLayer[layer[n]] = append(byLayer[layer[n]], n)
	}

	rows := make([][]int, 0, len(byLayer))

	for _, row := range byLayer {
		if len(row) > 0 {
			rows = append(rows, l.byGroup(row))
		}
	}

	return rows
}

// sinkLeaves moves a node with nothing but shallower neighbors one layer down when
// it shares its layer with nodes that do lead further down: a host attached straight
// to a spine then sits under the leaves instead of among them. Layers made only of
// such nodes stay put, which avoids stretching every link of the last row.
func (l *Layout) sinkLeaves(comp []int, layer map[int]int) {
	leaf := map[int]bool{}
	parents := map[int]bool{}

	for _, n := range comp {
		leaf[n] = true

		for _, nb := range l.nodes[n].adj {
			if layer[nb] > layer[n] {
				parents[layer[n]] = true
			}

			if layer[nb] >= layer[n] {
				leaf[n] = false
			}
		}
	}

	for _, n := range comp {
		if leaf[n] && parents[layer[n]] {
			layer[n]++
		}
	}
}
