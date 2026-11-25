// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"sort"
)

// assignPorts picks a column for every port facing a channel: the ports of a side
// are spread evenly over the columns open to it, in the order of where they lead.
func (u *unit) assignPorts(ch int) {
	for _, it := range u.rows[ch] {
		it.sides[sideBottom].prepare(&u.l.spec)
	}

	if ch+1 < len(u.rows) {
		for _, it := range u.rows[ch+1] {
			it.sides[sideTop].prepare(&u.l.spec)
		}
	}

	u.alignStraight(ch)
}

// prepare orders the ports of a side by where they lead and gives each its column.
func (s *side) prepare(spec *zoomSpec) {
	if len(s.slots) == 0 {
		return
	}

	if s.it.lane() {
		s.slots[0].col = s.it.x
		return
	}

	for _, sl := range s.slots {
		sum := 0.0

		for _, h := range sl.hops {
			sum += h.other(s.it).center()
		}

		sl.far = sum / float64(len(sl.hops))
	}

	sort.SliceStable(s.slots, func(i, j int) bool {
		a, b := s.slots[i], s.slots[j]
		if a.far != b.far {
			return a.far < b.far
		}

		return a.hops[0].edge < b.hops[0].edge
	})

	var free []int

	s.lo, s.hi = s.it.x+spec.inset, s.it.x+s.it.w-1-spec.inset

	for c := s.lo; c <= s.hi; c++ {
		if spec.portColumn(c, s.top) {
			free = append(free, c)
		}
	}

	// A box that may not grow can have more ports than columns. The ports then
	// double up, and their lines run down one stem to their different trunks.
	ports, cols := len(s.slots), len(free)
	s.shared = ports > cols

	for i, sl := range s.slots {
		if s.shared {
			sl.col = free[i*cols/ports]
		} else {
			sl.col = free[(2*i+1)*cols/(2*ports)] //nolint:mnd // center of a share
		}
	}

	// A port on its own goes to the middle, on the side of where it leads when
	// two columns are equally central.
	if ports == 1 {
		mid := s.it.center()
		aim := s.slots[0].far - mid

		for _, c := range free {
			was := math.Abs(float64(s.slots[0].col) - mid)
			now := math.Abs(float64(c) - mid)

			if now < was || (now == was && (float64(c)-mid)*aim > 0) {
				s.slots[0].col = c
			}
		}
	}
}

func (h *hop) other(it *item) *item {
	if h.ends[0] == it {
		return h.ends[1]
	}

	return h.ends[0]
}

// alignStraight moves ports so that edges run straight down instead of taking a
// step. Such an edge is the one exception to the rule of portColumn: it owns its
// column over the whole channel, and that column lies inside both boxes, or under a
// lane, where no other port of the channel can be.
//
// A port that is alone on its side goes wherever the other end is. Ports that share
// a side keep their even spacing, except to close a gap of half a step: that is how
// far the ports of two boxes drawn one above the other miss each other, and a row
// of small steps reads worse than two ports sitting a little closer.
func (u *unit) alignStraight(ch int) {
	for _, h := range u.hops[ch] {
		a, b := h.slots[0], h.slots[1]

		switch {
		case h.at[0] == h.at[1]:
		case a.side.lone() && b.side.lone():
			meet(a, b, &u.l.spec)
		case a.side.lone() && a.side.move(a, b.col):
		case b.side.lone() && b.side.move(b, a.col):
		case abs(a.col-b.col) > u.l.spec.step/2:
		case len(a.side.slots) <= len(b.side.slots) && a.side.move(a, b.col):
		case b.side.move(b, a.col):
		default:
			a.side.move(a, b.col)
		}
	}
}

// lone reports whether the side holds a single port for a single edge.
func (s *side) lone() bool {
	return !s.it.lane() && len(s.slots) == 1 && len(s.slots[0].hops) == 1
}

// move puts the port of one edge on a column if the border has room for it there,
// keeping a column free between ports.
func (s *side) move(sl *slot, col int) bool {
	if s.it.lane() || s.shared || len(sl.hops) > 1 || col < s.lo || col > s.hi {
		return false
	}

	for _, other := range s.slots {
		if other != sl && abs(other.col-col) < 2 {
			return false
		}
	}

	sl.col = col

	return true
}

// meet puts two lone ports on a shared column, as close as possible to the middle
// of both boxes.
func meet(a, b *slot, spec *zoomSpec) {
	want := (a.side.it.center() + b.side.it.center()) / 2 //nolint:mnd // mean
	best := -1

	for c := max(a.side.lo, b.side.lo); c <= min(a.side.hi, b.side.hi); c++ {
		// A column some port could use keeps its distance to the other
		// verticals of the channel.
		if spec.laneColumn(c) {
			continue
		}

		if best < 0 || math.Abs(float64(c)-want) < math.Abs(float64(best)-want) {
			best = c
		}
	}

	if best >= 0 {
		a.col, b.col = best, best
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
