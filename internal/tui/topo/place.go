// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"sort"
)

// Saved positions are scaled by a factor within these bounds, in cells per pixel.
// The app snaps to 20 px, so the lower bound still keeps columns apart and the
// upper bound stops a single tight pair from stretching the whole drawing.
const (
	minScale = 1.0 / 40
	maxScale = 1.0 / 6
)

// place assigns columns. Rows start spread over the width of the widest one, which
// puts a short row of spines evenly above a long row of leaves. The widest row then
// stays as it is and the others are pulled towards it one after the other, so that
// single links run straight. Pulling in both directions would let a narrow row drag
// a wide one together.
func (u *unit) place() {
	widths := make([]int, len(u.rows))
	anchor := 0

	for r, row := range u.rows {
		for i, it := range row {
			widths[r] += it.w
			if i > 0 {
				widths[r] += u.gap(row[i-1], it)
			}

			// A lane may have to move up to a step to find its column.
			if it.lane() {
				widths[r] += u.l.spec.step - 1
			}
		}

		if widths[r] > widths[anchor] {
			anchor = r
		}
	}

	u.span = widths[anchor]

	for r, row := range u.rows {
		x := 0.0
		slack := float64(widths[anchor] - widths[r])

		for i, it := range row {
			if i > 0 {
				x += float64(u.gap(row[i-1], it))
			}

			shift := slack * float64(2*i+1) / float64(2*len(row)) //nolint:mnd
			it.cx = x + shift + float64(it.w)/2                   //nolint:mnd
			x += float64(it.w)
		}
	}

	for r := anchor - 1; r >= 0; r-- {
		u.pull(u.rows[r], func(it *item) []*item { return it.downs })
	}

	for r := anchor + 1; r < len(u.rows); r++ {
		u.pull(u.rows[r], func(it *item) []*item { return it.ups })
	}

	u.snap()
}

// pull moves the items of a row towards the neighbors selected by towards: onto a
// single neighbor, and to the nearest end of the stretch several neighbors cover.
// An item already above or below that stretch stays, which keeps a row evenly
// spread when its items share their neighbors.
func (u *unit) pull(row []*item, towards func(*item) []*item) {
	want := make([]float64, len(row))
	offset := make([]float64, len(row))

	for i, it := range row {
		if i > 0 {
			prev := row[i-1]
			offset[i] = offset[i-1] + float64(prev.w+it.w)/2 + float64(u.gap(prev, it))
		}

		target := it.cx

		if nbs := towards(it); len(nbs) > 0 {
			xs := make([]float64, len(nbs))
			for k, nb := range nbs {
				xs[k] = nb.cx
			}

			sort.Float64s(xs)
			target = math.Min(math.Max(it.cx, xs[0]), xs[len(xs)-1])
		}

		want[i] = target - offset[i]
	}

	// No row may stick out beyond the widest one: the drawing is as wide as its
	// widest row needs and no wider.
	last := len(row) - 1
	lo := float64(row[0].w) / 2 //nolint:mnd
	hi := float64(u.span) - float64(row[last].w)/2 - offset[last]

	for i, v := range monotone(want) {
		row[i].cx = math.Min(math.Max(v, lo), math.Max(lo, hi)) + offset[i]
	}
}

// monotone returns the non-decreasing sequence closest to v in the least-squares
// sense, by pooling adjacent violators. With the minimum distances already taken
// out of v, this is the placement nearest to the wishes that keeps items apart.
func monotone(v []float64) []float64 {
	type block struct {
		sum float64
		n   int
	}

	var blocks []block

	for _, x := range v {
		blocks = append(blocks, block{x, 1})

		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/float64(a.n) <= b.sum/float64(b.n) {
				break
			}

			blocks = append(blocks[:len(blocks)-2], block{a.sum + b.sum, a.n + b.n})
		}
	}

	out := make([]float64, 0, len(v))

	for _, b := range blocks {
		for range b.n {
			out = append(out, b.sum/float64(b.n))
		}
	}

	return out
}

// snap rounds centers to columns and restores the minimum distances lost to
// rounding. Lanes go on the columns no port uses, and never right under the lane of
// another edge: both would need the same column in the channel between.
func (u *unit) snap() {
	spec := &u.l.spec
	leftmost := math.MaxInt

	for r, row := range u.rows {
		for i, it := range row {
			it.x = int(math.Round(it.cx - float64(it.w)/2))

			if i > 0 {
				prev := row[i-1]
				it.x = max(it.x, prev.x+prev.w+u.gap(prev, it))
			}

			for it.lane() && !u.laneFree(it, r, it.x) {
				it.x++
			}
		}

		u.fit(r)

		leftmost = min(leftmost, row[0].x)
	}

	// Shifting by whole steps keeps every item on its kind of column. What is
	// left of the first column stays blank and is cut off when units are packed.
	shift := leftmost - floorMod(leftmost, spec.step)

	for _, row := range u.rows {
		for _, it := range row {
			it.x -= shift
		}
	}

	u.straighten()
}

// fit pushes a row back inside the width of the widest one where rounding, or a
// lane looking for its column, made it stick out to the right.
func (u *unit) fit(r int) {
	row := u.rows[r]
	limit := u.span

	for i := len(row) - 1; i >= 0 && u.span > 0; i-- {
		it := row[i]
		if it.x+it.w <= limit {
			return
		}

		it.x = limit - it.w

		for it.lane() && !u.laneFree(it, r, it.x) {
			it.x--
		}

		if i > 0 {
			limit = it.x - u.gap(row[i-1], it)
		}
	}
}

// laneFree reports whether a lane may use a column of a row.
func (u *unit) laneFree(ln *item, row, col int) bool {
	return u.l.spec.laneColumn(col) && !u.laneClash(ln, row, col)
}

// laneClash reports whether a lane on a column of a row would sit right under or
// above the lane of another edge.
func (u *unit) laneClash(ln *item, row, col int) bool {
	for _, r := range []int{row - 1, row + 1} {
		if r < 0 || r >= len(u.rows) {
			continue
		}

		for _, other := range u.rows[r] {
			if other.lane() && other.edge != ln.edge && other.x == col {
				return true
			}
		}
	}

	return false
}

// straighten gives all lanes of an edge the same column where the gaps allow it, so
// a long edge crosses the rows between its ends in one straight line.
func (u *unit) straighten() {
	for _, e := range u.l.edges {
		chain := u.chains[e.ID]
		if len(chain) < 4 { //nolint:mnd // two boxes and at least two lanes
			continue
		}

		lanes := chain[1 : len(chain)-1]

		for _, candidate := range lanes {
			if u.lanesFit(lanes, candidate.x) {
				for _, ln := range lanes {
					ln.x = candidate.x
				}

				break
			}
		}
	}
}

func (u *unit) lanesFit(lanes []*item, col int) bool {
	for _, ln := range lanes {
		row := u.rows[ln.row]

		if ln.pos > 0 {
			if prev := row[ln.pos-1]; col < prev.x+prev.w+u.gap(prev, ln) {
				return false
			}
		}

		if ln.pos+1 < len(row) {
			if next := row[ln.pos+1]; col+1+u.gap(ln, next) > next.x {
				return false
			}
		}

		if u.laneClash(ln, ln.row, col) {
			return false
		}
	}

	return true
}

// floorMod is the remainder of a division that rounds towards negative infinity.
func floorMod(a, b int) int { return ((a % b) + b) % b }

// placeSaved keeps the proportions of saved positions. The scale is the smallest
// one at which the tightest pair of neighbors no longer overlaps; pairs that are
// still too close at the upper bound are pushed apart afterwards.
func (u *unit) placeSaved() {
	scale := minScale
	origin := math.Inf(1)

	for _, row := range u.rows {
		var prev *item

		for _, it := range row {
			origin = math.Min(origin, it.key)

			if it.lane() {
				continue
			}

			if prev != nil && it.key > prev.key {
				need := float64(prev.w+it.w)/2 + float64(u.gap(prev, it)) //nolint:mnd
				scale = math.Max(scale, need/(it.key-prev.key))
			}

			prev = it
		}
	}

	scale = math.Min(scale, maxScale)

	for _, row := range u.rows {
		for _, it := range row {
			it.cx = (it.key - origin) * scale
		}
	}

	u.snap()
}
