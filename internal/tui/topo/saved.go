// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"sort"
)

const (
	// rowBreak is the vertical distance in pixels beyond which a node starts a
	// new row. Nodes of one visual row are rarely aligned to the pixel.
	rowBreak = 50
	// strayStep spaces nodes that have neither a position nor a placed neighbor.
	strayStep = 100
)

// savedUnit builds the drawing from saved canvas positions. They give the rows and
// the order within them, not coordinates: a box needs far more room in cells than
// the same node takes on a canvas, so a linear mapping would overlap.
func (l *Layout) savedUnit() *unit {
	var placed []int

	for i, b := range l.nodes {
		if b.HasPos {
			placed = append(placed, i)
		}
	}

	sort.SliceStable(placed, func(i, j int) bool {
		a, b := l.nodes[placed[i]], l.nodes[placed[j]]
		if a.Y != b.Y {
			return a.Y < b.Y
		}

		return l.rank[a.idx] < l.rank[b.idx]
	})

	rowOf := map[int]int{}
	keys := map[int]float64{}
	rows, sum, count := 0, 0.0, 0

	for _, n := range placed {
		b := l.nodes[n]

		if count == 0 || b.Y-sum/float64(count) > rowBreak {
			rows++
			sum, count = 0, 0
		}

		sum += b.Y
		count++
		rowOf[n], keys[n] = rows-1, b.X
	}

	rows = l.seedMissing(rowOf, keys, rows)

	out := make([][]int, rows)
	for n := range l.nodes {
		out[rowOf[n]] = append(out[rowOf[n]], n)
	}

	return l.newUnit(out, keys, true)
}

// seedMissing places the nodes without a saved position by their neighbors: in the
// mean row, or one below when the neighbors share a row, at their mean x. Nodes
// with no placed neighbor at all end up in a row of their own at the bottom.
func (l *Layout) seedMissing(rowOf map[int]int, keys map[int]float64, rows int) int {
	for progress := true; progress; {
		progress = false

		for n, b := range l.nodes {
			if _, done := rowOf[n]; done {
				continue
			}

			rowSum, xSum, count, same := 0, 0.0, 0, true

			for _, nb := range b.adj {
				r, ok := rowOf[nb]
				if !ok {
					continue
				}

				if count > 0 && r*count != rowSum {
					same = false
				}

				rowSum += r
				xSum += keys[nb]
				count++
			}

			if count == 0 {
				continue
			}

			row := int(math.Round(float64(rowSum) / float64(count)))
			if same {
				row++
			}

			rowOf[n], keys[n] = row, xSum/float64(count)
			rows = max(rows, row+1)
			progress = true
		}
	}

	left := math.Inf(1)
	for _, x := range keys {
		left = math.Min(left, x)
	}

	if math.IsInf(left, 1) {
		left = 0
	}

	stray := 0

	for n := range l.nodes {
		if _, done := rowOf[n]; !done {
			rowOf[n], keys[n] = rows, left+float64(stray*strayStep)
			stray++
		}
	}

	if stray > 0 {
		rows++
	}

	return rows
}
