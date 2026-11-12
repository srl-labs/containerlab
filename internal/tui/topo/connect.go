// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"sort"
	"strconv"
	"strings"
)

// side is the top or bottom border of an item with the edges that attach to it.
type side struct {
	it     *item
	top    bool
	hops   []*hop
	slots  []*slot
	lo, hi int  // first and last column of the border open to ports
	shared bool // more ports than columns: some ports sit on the same one
}

// slot is one port: a column on a side where one edge attaches, or all the edges
// of the side that belong to one net.
type slot struct {
	side *side
	net  *net
	hops []*hop
	far  float64 // center of what the port leads to, the port order on the side
	col  int
}

// hop is the part of an edge inside one channel. A hop between rows has its upper
// end first; a hop between two boxes of the same row hangs from both bottoms and has
// the left one first.
type hop struct {
	edge  int
	ch    int
	ends  [2]*item
	at    [2]int // side of each end
	slots [2]*slot
	net   *net
}

// connect turns edges into hops, groups the hops that have to share a line and
// sizes the boxes for their ports.
func (u *unit) connect() {
	u.hops = make([][]*hop, len(u.rows))

	for _, row := range u.rows {
		for _, it := range row {
			it.w = 1
			if !it.lane() {
				it.w = u.l.labelWidth(u.l.nodes[it.node])
			}
		}
	}

	for _, e := range u.l.edges {
		chain := u.chains[e.ID]

		for i := 0; i+1 < len(chain); i++ {
			u.addHop(&hop{
				edge: e.ID, ch: chain[i].row,
				ends: [2]*item{chain[i], chain[i+1]}, at: [2]int{sideBottom, sideTop},
			})
		}
	}

	for _, id := range u.flat {
		a, b := u.boxes[u.l.edges[id].a], u.boxes[u.l.edges[id].b]
		if a.pos > b.pos {
			a, b = b, a
		}

		// Neighbors get a side link; anything further apart would cut through
		// the boxes between, so it dips into the channel below instead.
		if b.pos-a.pos == 1 {
			u.direct[id] = true
			continue
		}

		u.addHop(&hop{
			edge: id, ch: a.row,
			ends: [2]*item{a, b}, at: [2]int{sideBottom, sideBottom},
		})
	}

	u.nets = make([][]*net, len(u.rows))
	for ch := range u.rows {
		u.formNets(ch)
	}

	u.makeSlots()
	u.sizeBoxes()
}

func (u *unit) addHop(h *hop) {
	u.hops[h.ch] = append(u.hops[h.ch], h)
	u.edgeHops[h.edge] = append(u.edgeHops[h.edge], h)

	for i, it := range h.ends {
		s := &it.sides[h.at[i]]
		s.it, s.top = it, h.at[i] == sideTop
		s.hops = append(s.hops, h)
	}
}

// formNets decides which hops of a channel are drawn as one line.
//
// A box has room for a limited number of ports. When more edges arrive from above
// than that, they all enter through one port, and everything that port connects to
// joins a common trunk. Such a trunk says that every box above it is linked to
// every box below it, so boxes may only share one when that is true: when they have
// the same set of neighbors above. A fabric where every leaf connects to every
// spine becomes a single trunk this way.
//
// A box with too many edges going down bundles those that are still on their own
// into one trunk of its own, with a drop to each neighbor.
func (u *unit) formNets(ch int) {
	groups := map[string]*net{}

	add := func(n *net, hops []*hop) {
		for _, h := range hops {
			h.net = n
			n.hops = append(n.hops, h)
		}
	}

	if ch+1 < len(u.rows) {
		for _, it := range u.rows[ch+1] {
			s := &it.sides[sideTop]
			if it.lane() || len(s.hops) <= u.l.spec.capacity(it.w) {
				continue
			}

			key := above(s.hops)
			if groups[key] == nil {
				groups[key] = &net{}
				u.nets[ch] = append(u.nets[ch], groups[key])
			}

			add(groups[key], s.hops)
		}
	}

	for _, it := range u.rows[ch] {
		s := &it.sides[sideBottom]

		var free []*hop

		ports := map[*net]bool{}

		for _, h := range s.hops {
			if h.net == nil {
				free = append(free, h)
			} else {
				ports[h.net] = true
			}
		}

		if !it.lane() && len(free) > 1 && len(ports)+len(free) > u.l.spec.capacity(it.w) {
			n := &net{}
			u.nets[ch] = append(u.nets[ch], n)
			add(n, free)
		}
	}

	for _, h := range u.hops[ch] {
		if h.net == nil {
			n := &net{}
			u.nets[ch] = append(u.nets[ch], n)
			add(n, []*hop{h})
		}
	}
}

// above names the set of items a group of hops comes down from.
func above(hops []*hop) string {
	ids := make([]int, len(hops))
	for i, h := range hops {
		ids[i] = h.ends[0].id
	}

	sort.Ints(ids)

	var b strings.Builder
	for _, id := range ids {
		b.WriteString(strconv.Itoa(id))
		b.WriteByte(' ')
	}

	return b.String()
}

// makeSlots gives every side one port per net it takes part in.
func (u *unit) makeSlots() {
	for _, row := range u.rows {
		for _, it := range row {
			for i := range it.sides {
				s := &it.sides[i]
				byNet := map[*net]*slot{}

				for _, h := range s.hops {
					sl := byNet[h.net]
					if sl == nil {
						sl = &slot{side: s, net: h.net}
						byNet[h.net] = sl
						s.slots = append(s.slots, sl)
					}

					sl.hops = append(sl.hops, h)

					for e := range h.ends {
						if h.ends[e] == it && h.at[e] == i {
							h.slots[e] = sl
						}
					}
				}
			}
		}
	}
}

// sizeBoxes widens boxes for the ports of their busier side where the zoom level
// allows it, and settles the label text, which may use the room gained.
func (u *unit) sizeBoxes() {
	spec := &u.l.spec

	for _, row := range u.rows {
		for _, it := range row {
			if it.lane() {
				continue
			}

			ports := max(len(it.sides[sideTop].slots), len(it.sides[sideBottom].slots))
			if spec.widen && ports > 0 {
				it.w = max(it.w, spec.portWidth(min(ports, maxPorts)))
			}

			u.l.nodes[it.node].setText(it.w, spec)
		}
	}
}

// room returns the columns asked for on the link between two nodes, zero for none.
func (l *Layout) room(a, b int) int {
	if len(l.opts.Room) == 0 {
		return 0
	}

	x, y := l.nodes[a].ID, l.nodes[b].ID

	return max(l.opts.Room[[2]string{x, y}], l.opts.Room[[2]string{y, x}])
}

// gap is the least number of blank columns between two neighbors in a row.
func (u *unit) gap(a, b *item) int {
	switch {
	case !a.lane() && !b.lane():
		return max(u.l.spec.gap, u.l.room(a.node, b.node))
	case a.lane() && b.lane():
		return 1
	default:
		return u.l.spec.laneGap
	}
}
