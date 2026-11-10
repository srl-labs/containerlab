// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

const (
	// unitGap separates drawings placed side by side.
	unitGap = 6
	// looseWrap is the least width at which the row of unlinked nodes wraps.
	looseWrap = 72
)

// pack places the units left to right, largest first, and the nodes without links
// in a grid underneath.
func (l *Layout) pack(units []*unit, loose []int) {
	x := 0

	for i, u := range units {
		// The kinds of columns only matter inside a unit, so each one can
		// start right where the previous one ends.
		u.ox = x - u.left
		l.w, l.h = max(l.w, u.ox+u.w), max(l.h, u.h)
		x = u.ox + u.w + unitGap

		rows := make([][]int, len(u.rows))

		for r, row := range u.rows {
			for _, it := range row {
				if !it.lane() {
					rows[r] = append(rows[r], it.node)
					l.nodes[it.node].unit, l.nodes[it.node].row = i, r
				}
			}
		}

		l.nav = append(l.nav, rows)
	}

	if len(loose) == 0 {
		return
	}

	var rows [][]int

	wrap := max(l.w, looseWrap)
	x, y := 0, l.h

	if l.h > 0 {
		y++
	}

	for _, n := range loose {
		b := l.nodes[n]
		w := l.labelWidth(b)
		b.setText(w, &l.spec)

		if x > 0 && x+w > wrap {
			x, y = 0, y+l.spec.boxH+1
		}

		if x == 0 {
			rows = append(rows, nil)
		}

		b.rect = Rect{X: x, Y: y, W: w, H: l.spec.boxH}
		b.unit, b.row = len(l.nav), len(rows)-1
		rows[len(rows)-1] = append(rows[len(rows)-1], n)
		l.w = max(l.w, x+w)
		x += w + l.spec.gap
	}

	l.h = y + l.spec.boxH
	l.nav = append(l.nav, rows)
}
