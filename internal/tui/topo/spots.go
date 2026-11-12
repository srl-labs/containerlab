// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"sort"
)

// Spot is a position on the canvas of a drawing, in its pixels: the unit the containerlab
// application saves the positions of nodes in.
type Spot struct{ X, Y float64 }

const (
	// SpotCol is the number of canvas pixels a column of a computed drawing stands
	// for when it is turned into spots. A box and the gap next to it then come to
	// about the distance the containerlab application keeps between nodes.
	SpotCol = 7.0

	// spotStep is how far a node that was never placed is put from the ones it is
	// linked to, spotClear how close to another node it may end up, and spotPitch the
	// distance between the nodes that have no placed neighbor at all.
	spotStep  = 120
	spotClear = 60
	spotPitch = 140
	// spotTries bounds the search for a free place next to the neighbors of a node.
	spotTries = 64
)

// Spots returns the centers of the boxes of a computed drawing as spots of a canvas.
// aspect is the height of a cell over its width: a canvas has square pixels, and with it
// the drawing keeps its proportions.
func (l *Layout) Spots(aspect float64) map[string]Spot {
	out := make(map[string]Spot, len(l.nodes))

	for _, b := range l.nodes {
		out[b.ID] = Spot{
			X: (float64(b.rect.X) + float64(b.rect.W)/2) * SpotCol,
			Y: (float64(b.rect.Y) + float64(b.rect.H)/2) * SpotCol * aspect,
		}
	}

	return out
}

// Seed returns a spot for every node of a graph. The nodes in known keep theirs. A graph
// none of whose nodes is known is laid out by its tiers or its structure; else the missing
// nodes are put next to the ones they are linked to, and the ones without a placed
// neighbor in a row below the drawing.
//
// The result depends only on the content of the graph and of known.
func Seed(g Graph, known map[string]Spot, aspect float64) map[string]Spot {
	l := newLayout(g, Options{})
	out := make(map[string]Spot, len(l.nodes))

	for _, b := range l.nodes {
		if s, ok := known[b.ID]; ok {
			out[b.ID] = s
		}
	}

	if len(out) == len(l.nodes) {
		return out
	}

	if len(out) == 0 {
		// Without saved positions a drawing follows the tiers or the structure.
		bare := Graph{Nodes: make([]Node, len(g.Nodes)), Links: g.Links}
		copy(bare.Nodes, g.Nodes)

		for i := range bare.Nodes {
			bare.Nodes[i].HasPos = false
		}

		return Compute(bare, Options{Zoom: ZoomNormal}).Spots(aspect)
	}

	order := make([]int, len(l.nodes))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(i, j int) bool {
		return l.rank[order[i]] < l.rank[order[j]]
	})

	// Nodes with a placed neighbor first, each new one helping to place the next.
	for progress := true; progress; {
		progress = false

		for _, n := range order {
			b := l.nodes[n]
			if _, done := out[b.ID]; done {
				continue
			}

			var sum Spot

			count := 0

			for _, nb := range b.adj {
				if s, ok := out[l.nodes[nb].ID]; ok {
					sum.X += s.X
					sum.Y += s.Y
					count++
				}
			}

			if count == 0 {
				continue
			}

			at := Spot{X: sum.X / float64(count), Y: sum.Y/float64(count) + spotStep}
			out[b.ID] = clearSpot(out, at)
			progress = true
		}
	}

	left, bottom := math.Inf(1), math.Inf(-1)
	for _, s := range out {
		left, bottom = math.Min(left, s.X), math.Max(bottom, s.Y)
	}

	stray := 0

	for _, n := range order {
		b := l.nodes[n]
		if _, done := out[b.ID]; !done {
			out[b.ID] = Spot{X: left + float64(stray*spotPitch), Y: bottom + spotStep}
			stray++
		}
	}

	return out
}

// clearSpot moves a spot sideways, to the right and to the left in turns, until no other
// one is close to it.
func clearSpot(taken map[string]Spot, at Spot) Spot {
	free := func(s Spot) bool {
		for _, other := range taken {
			if math.Abs(other.X-s.X) < spotClear && math.Abs(other.Y-s.Y) < spotClear {
				return false
			}
		}

		return true
	}

	for try := range spotTries {
		shift := float64((try+1)/2) * spotStep //nolint:mnd // a step every other try
		if try%2 == 1 {
			shift = -shift
		}

		if s := (Spot{X: at.X + shift, Y: at.Y}); free(s) {
			return s
		}
	}

	return at
}
