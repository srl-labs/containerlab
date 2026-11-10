// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"slices"
	"sort"
)

// trackGap is the least distance between two nets on one track. One blank cell
// between their corners keeps them from reading as a single line.
const trackGap = 2

// net is everything in a channel that shares one horizontal track: the hop of a
// single edge, or all hops that formNets put on one trunk.
type net struct {
	hops   []*hop
	ups    []int // columns with a vertical from the track up to the row above
	downs  []int // columns with a vertical from the track down to the row below
	lo, hi int
	track  int // -1 for a straight vertical, which needs none

	// Scheduling state: nets that must (hard) or should (soft) be on a track
	// below this one, and how many such predecessors are still unplaced.
	after      []*net
	afterHard  []bool
	hard, soft int
	placed     bool
}

// measure records where the verticals of a net are, once its ports have a column.
func (n *net) measure() {
	n.lo, n.hi = n.hops[0].slots[0].col, n.hops[0].slots[0].col
	n.ups, n.downs = nil, nil

	for _, h := range n.hops {
		for i, sl := range h.slots {
			n.lo, n.hi = min(n.lo, sl.col), max(n.hi, sl.col)

			if h.at[i] == sideBottom {
				n.ups = addUnique(n.ups, sl.col)
			} else {
				n.downs = addUnique(n.downs, sl.col)
			}
		}
	}

	n.track = -1
}

func addUnique(v []int, x int) []int {
	if slices.Contains(v, x) {
		return v
	}

	return append(v, x)
}

// assignTracks stacks the nets of a channel and returns the number of tracks used.
//
// It is the left-edge algorithm with an ordering between nets. Two nets that share
// a column, one reaching it from above and one from below, must be stacked in that
// order or their verticals would overlap. Beyond that, a pair is stacked in the
// order that makes their lines cross less: for edges fanning out of one box this
// puts the outermost on top and nests the rest below without a single crossing.
func assignTracks(nets []*net) int {
	var todo []*net

	for _, n := range nets {
		if n.lo < n.hi {
			todo = append(todo, n)
		}
	}

	sort.SliceStable(todo, func(i, j int) bool {
		a, b := todo[i], todo[j]

		switch {
		case a.lo != b.lo:
			return a.lo < b.lo
		case a.hi != b.hi:
			return a.hi < b.hi
		default:
			return a.hops[0].edge < b.hops[0].edge
		}
	})

	for i, a := range todo {
		for _, b := range todo[i+1:] {
			constrain(a, b)
		}
	}

	track := 0

	for ; len(todo) > 0; track++ {
		row := fillTrack(todo)

		for _, n := range row {
			n.track, n.placed = track, true
		}

		// Successors are released only now, so that none lands on the same
		// track as the net it has to stay below.
		for _, n := range row {
			for i, next := range n.after {
				if n.afterHard[i] {
					next.hard--
				} else {
					next.soft--
				}
			}
		}

		rest := todo[:0]

		for _, n := range todo {
			if !n.placed {
				rest = append(rest, n)
			}
		}

		todo = rest
	}

	return track
}

// constrain records which of two nets belongs on the upper track.
func constrain(a, b *net) {
	aFirst, bFirst := shares(a.ups, b.downs), shares(b.ups, a.downs)

	switch {
	case aFirst && bFirst:
		// No order avoids the overlap; leave the pair to the left-edge order.
		return
	case aFirst:
		a.before(b, true)
		return
	case bFirst:
		b.before(a, true)
		return
	}

	ab, ba := crossings(a, b), crossings(b, a)

	switch {
	case ab < ba:
		a.before(b, false)
	case ba < ab:
		b.before(a, false)
	}
}

func (n *net) before(next *net, hard bool) {
	n.after = append(n.after, next)
	n.afterHard = append(n.afterHard, hard)

	if hard {
		next.hard++
	} else {
		next.soft++
	}
}

func shares(a, b []int) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}

	return false
}

// crossings counts the line crossings between two nets when upper is on the higher
// track: verticals of the lower net rising through the upper track, and verticals
// of the upper net dropping through the lower one.
func crossings(upper, lower *net) int {
	count := 0

	for _, c := range lower.ups {
		if c >= upper.lo && c <= upper.hi {
			count++
		}
	}

	for _, c := range upper.downs {
		if c >= lower.lo && c <= lower.hi {
			count++
		}
	}

	return count
}

// fillTrack picks the nets for the next track, left to right. When the preferred
// orders contradict each other it gives them up, and as a last resort the required
// ones as well, so that every pass places at least one net.
func fillTrack(todo []*net) []*net {
	for _, ready := range []func(*net) bool{
		func(n *net) bool { return n.hard == 0 && n.soft == 0 },
		func(n *net) bool { return n.hard == 0 },
	} {
		var (
			row  []*net
			last *net
		)

		for _, n := range todo {
			if ready(n) && (last == nil || n.lo-last.hi >= trackGap) {
				row = append(row, n)
				last = n
			}
		}

		if len(row) > 0 {
			return row
		}
	}

	return todo[:1]
}
