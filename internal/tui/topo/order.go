// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import "sort"

// sweeps is the number of barycenter passes. Crossings stop improving well before
// that on labs of realistic size.
const sweeps = 6

const (
	sideTop = iota
	sideBottom
)

// unit is one independently drawn stack of rows: a connected component, or the
// whole graph when rows come from saved positions.
type unit struct {
	l        *Layout
	rows     [][]*item
	boxes    map[int]*item
	chains   map[int][]*item // edge ID to its items from the upper to the lower box
	flat     []int           // edges between two boxes of one row
	direct   map[int]bool    // flat edges drawn as a straight side link
	hops     [][]*hop        // per channel
	edgeHops map[int][]*hop
	nets     [][]*net
	tracks   []int // per channel
	rowY     []int
	chanY    []int
	items    int  // number of items, for their IDs
	span     int  // width of the widest row when packed, 0 when rows are not packed
	keyed    bool // item keys are saved pixel positions
	fixed    bool // keep the given order within rows
	w, h     int
	left     int // first column in use; the columns before it are blank
	ox, oy   int
}

// item is a slot in a row: a node box, or the lane of an edge that passes through
// the row on its way to a box further down.
type item struct {
	id         int
	node, edge int     // one of them is -1
	row, pos   int     //
	key        float64 // initial ordering key
	x, w       int
	cx         float64 // center while placing
	ups, downs []*item
	sides      [2]side
}

func (it *item) lane() bool { return it.node < 0 }

func (it *item) center() float64 { return float64(it.x) + float64(it.w-1)/2 }

func (l *Layout) newUnit(rows [][]int, keys map[int]float64, fixed bool) *unit {
	u := &unit{
		l: l, boxes: map[int]*item{}, chains: map[int][]*item{},
		direct: map[int]bool{}, edgeHops: map[int][]*hop{},
		keyed: keys != nil, fixed: fixed || keys != nil,
	}

	u.build(rows, keys)

	if !u.fixed {
		u.order()
	}

	u.connect()

	if u.keyed {
		u.placeSaved()
	} else {
		u.place()
	}

	u.route()

	return u
}

// build creates the items of every row, inserting a lane wherever an edge crosses a
// row, so each remaining segment connects two adjacent rows.
func (u *unit) build(rows [][]int, keys map[int]float64) {
	u.rows = make([][]*item, len(rows))

	for r, row := range rows {
		for i, n := range row {
			it := &item{id: u.items, node: n, edge: -1, row: r}
			u.items++
			it.key = (float64(i) + 0.5) / float64(len(row)) //nolint:mnd // cell center

			if keys != nil {
				it.key = keys[n]
			}

			u.rows[r] = append(u.rows[r], it)
			u.boxes[n] = it
		}
	}

	for _, e := range u.l.edges {
		a, okA := u.boxes[e.a]
		b, okB := u.boxes[e.b]

		if !okA || !okB {
			continue
		}

		if a.row == b.row {
			u.flat = append(u.flat, e.ID)
			continue
		}

		if a.row > b.row {
			a, b = b, a
		}

		chain := []*item{a}

		for r := a.row + 1; r < b.row; r++ {
			f := float64(r-a.row) / float64(b.row-a.row)
			ln := &item{id: u.items, node: -1, edge: e.ID, row: r, key: a.key + (b.key-a.key)*f}
			u.items++
			u.rows[r] = append(u.rows[r], ln)
			chain = append(chain, ln)
		}

		chain = append(chain, b)

		for i := 0; i+1 < len(chain); i++ {
			chain[i].downs = append(chain[i].downs, chain[i+1])
			chain[i+1].ups = append(chain[i+1].ups, chain[i])
		}

		u.chains[e.ID] = chain
	}

	for _, row := range u.rows {
		u.sortRow(row, func(it *item) float64 { return it.key })
	}
}

// sortRow orders a row by key, breaking ties by name so the result never depends on
// the input order.
func (u *unit) sortRow(row []*item, key func(*item) float64) {
	keys := make(map[*item]float64, len(row))
	for _, it := range row {
		keys[it] = key(it)
	}

	sort.SliceStable(row, func(i, j int) bool {
		a, b := row[i], row[j]

		switch {
		case keys[a] != keys[b]:
			return keys[a] < keys[b]
		case a.lane() != b.lane():
			return b.lane()
		case a.lane():
			return a.edge < b.edge
		default:
			return u.l.rank[a.node] < u.l.rank[b.node]
		}
	})

	for i, it := range row {
		it.pos = i
	}
}

// order reduces crossings with alternating barycenter sweeps and keeps the best
// arrangement seen.
func (u *unit) order() {
	best, bestCount := u.snapshot(), u.crossings()

	for i := 0; i < sweeps && bestCount > 0; i++ {
		if i%2 == 0 {
			for r := 1; r < len(u.rows); r++ {
				u.sweep(u.rows[r], func(it *item) []*item { return it.ups })
			}
		} else {
			for r := len(u.rows) - 2; r >= 0; r-- { //nolint:mnd // from the row above the last
				u.sweep(u.rows[r], func(it *item) []*item { return it.downs })
			}
		}

		if c := u.crossings(); c < bestCount {
			best, bestCount = u.snapshot(), c
		}
	}

	for r, row := range best {
		copy(u.rows[r], row)

		for i, it := range row {
			it.pos = i
		}
	}
}

func (u *unit) snapshot() [][]*item {
	out := make([][]*item, len(u.rows))
	for r, row := range u.rows {
		out[r] = append([]*item(nil), row...)
	}

	return out
}

// sweep moves each item of a row to the mean position of its neighbors in the
// adjacent row. Positions are fractions of the row length, so rows of different
// sizes compare.
func (u *unit) sweep(row []*item, towards func(*item) []*item) {
	frac := func(it *item) float64 {
		return (float64(it.pos) + 0.5) / float64(len(u.rows[it.row])) //nolint:mnd
	}

	u.sortRow(row, func(it *item) float64 {
		nbs := towards(it)
		if len(nbs) == 0 {
			return frac(it)
		}

		sum := 0.0
		for _, nb := range nbs {
			sum += frac(nb)
		}

		return sum / float64(len(nbs))
	})
}

// crossings counts the segment pairs that cross between adjacent rows.
func (u *unit) crossings() int {
	total := 0

	for _, row := range u.rows {
		var lower []int

		for _, it := range row {
			first := len(lower)

			for _, d := range it.downs {
				lower = append(lower, d.pos)
			}

			sort.Ints(lower[first:])
		}

		total += inversions(lower)
	}

	return total
}

// inversions counts the pairs that are out of order.
func inversions(v []int) int {
	if len(v) < 2 { //nolint:mnd // nothing to compare
		return 0
	}

	mid := len(v) / 2 //nolint:mnd // merge sort halves
	left := append([]int(nil), v[:mid]...)
	right := append([]int(nil), v[mid:]...)
	count := inversions(left) + inversions(right)
	i, j := 0, 0

	for k := range v {
		if j >= len(right) || (i < len(left) && left[i] <= right[j]) {
			v[k] = left[i]
			i++
		} else {
			v[k] = right[j]
			count += len(left) - i
			j++
		}
	}

	return count
}
