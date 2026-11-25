// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package topo

import (
	"math"
	"sort"
	"strings"
)

// Tiers of a data-center, campus or service-provider lab, top to bottom.
const (
	tierInternet = iota
	tierFirewall
	tierReflector
	tierCore
	tierSuperSpine
	tierSpine
	tierLeaf
	tierHost
)

// roleTiers maps a name token to its tier.
var roleTiers = map[string]int{
	"cloud": tierInternet, "internet": tierInternet, "isp": tierInternet, "wan": tierInternet,
	"fw": tierFirewall, "firewall": tierFirewall,
	"rr": tierReflector, "reflector": tierReflector,
	"dcgw": tierCore, "border": tierCore, "gw": tierCore, "core": tierCore, "p": tierCore,
	"superspine": tierSuperSpine,
	"spine":      tierSpine, "agg": tierSpine, "dist": tierSpine, "pe": tierSpine,
	"leaf": tierLeaf, "tor": tierLeaf, "access": tierLeaf, "acc": tierLeaf,
	"ce": tierLeaf, "cpe": tierLeaf, "pon": tierLeaf,
	"server": tierHost, "client": tierHost, "host": tierHost, "ue": tierHost, "rgw": tierHost,
}

const (
	// hostDegree is the most links a linux container may have and still be taken
	// for an end host rather than a router built from one.
	hostDegree = 2
	// pluralLen is the shortest token whose trailing s is read as a plural, so
	// that "leafs" is a leaf but "ps" is not a P router.
	pluralLen = 4
)

// tiers returns a tier per node when numeric levels or role names describe enough
// of the lab, and nil otherwise. A node stays at -1 only when nothing in its
// component is known.
func (l *Layout) tiers() []int {
	t := make([]int, len(l.nodes))

	for i, b := range l.nodes {
		t[i] = -1
		if b.Level > 0 {
			t[i] = b.Level
		}
	}

	if !describes(t) {
		for i, b := range l.nodes {
			t[i] = l.roleTier(b)
		}

		if !describes(t) {
			return nil
		}
	}

	l.inferTiers(t)

	return t
}

// describes reports whether tiers cover at least 60 % of the nodes and separate them
// into at least two rows. Below that the names are more likely coincidence.
func describes(t []int) bool {
	known := 0
	distinct := map[int]bool{}

	for _, v := range t {
		if v >= 0 {
			known++
			distinct[v] = true
		}
	}

	return known*5 >= len(t)*3 && len(distinct) >= 2
}

func (l *Layout) roleTier(b *box) int {
	for _, s := range []string{b.Role, b.ID, b.Label, b.Group} {
		if t, ok := tokenTier(s); ok {
			return t
		}
	}

	if b.Kind == "linux" && len(b.adj) <= hostDegree {
		return tierHost
	}

	return -1
}

// tokenTier looks for a role word in a name such as "dc1-super-spine2". A role only
// counts as a whole word: letters between digits and separators.
func tokenTier(s string) (int, bool) {
	tokens := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r < 'a' || r > 'z'
	})

	for i, tok := range tokens {
		if tok == "super" && i+1 < len(tokens) && strings.HasPrefix(tokens[i+1], "spine") {
			return roleTiers["superspine"], true
		}

		if t, ok := roleTiers[tok]; ok {
			return t, true
		}

		if len(tok) >= pluralLen {
			if t, ok := roleTiers[strings.TrimSuffix(tok, "s")]; ok {
				return t, true
			}
		}
	}

	return 0, false
}

// stem is a node name without its numbers: p1 and p12 share the stem "p#".
func stem(id string) string {
	var b strings.Builder

	digits := false

	for _, r := range strings.ToLower(id) {
		if r >= '0' && r <= '9' {
			if !digits {
				b.WriteByte('#')
			}

			digits = true

			continue
		}

		digits = false

		b.WriteRune(r)
	}

	return b.String()
}

// inferTiers places the nodes that have no tier of their own.
//
// Nodes named alike belong together, whatever the name means: p1 to p4 share a row
// even if only one of them could be placed. So a stem with known members lends
// their tier to the rest, and the remaining stems are placed as a whole by what
// they connect to.
func (l *Layout) inferTiers(t []int) {
	byStem := map[string][]int{}

	var stems []string

	for i, b := range l.nodes {
		s := stem(b.ID)
		if _, ok := byStem[s]; !ok {
			stems = append(stems, s)
		}

		byStem[s] = append(byStem[s], i)
	}

	var open [][]int

	for _, s := range stems {
		group := byStem[s]

		if tier, ok := commonTier(t, group); ok {
			for _, n := range group {
				if t[n] < 0 {
					t[n] = tier
				}
			}

			continue
		}

		open = append(open, group)
	}

	for len(open) > 0 {
		settled := l.settle(t, open)
		if len(settled) == 0 {
			return
		}

		rest := open[:0]

		for i, group := range open {
			if !settled[i] {
				rest = append(rest, group)
			}
		}

		open = rest
	}
}

// commonTier returns the tier most members of a group already have, the upper one
// on a draw.
func commonTier(t, group []int) (int, bool) {
	count := map[int]int{}
	best := -1

	for _, n := range group {
		if t[n] < 0 {
			continue
		}

		count[t[n]]++

		if best < 0 || count[t[n]] > count[best] ||
			(count[t[n]] == count[best] && t[n] < best) {
			best = t[n]
		}
	}

	return best, best >= 0
}

// evidence is what the placed neighbors of a group say about its tier.
type evidence struct {
	lo, hi  int // lowest and highest tier among the placed neighbors
	sum, n  int
	pending bool // some neighbors are not placed yet
}

func (l *Layout) evidence(t, group []int) evidence {
	member := map[int]bool{}
	for _, n := range group {
		member[n] = true
	}

	var ev evidence

	for _, n := range group {
		for _, nb := range l.nodes[n].adj {
			switch {
			case member[nb]:
			case t[nb] < 0:
				ev.pending = true
			default:
				if ev.n == 0 || t[nb] < ev.lo {
					ev.lo = t[nb]
				}

				ev.hi = max(ev.hi, t[nb])
				ev.sum += t[nb]
				ev.n++
			}
		}
	}

	return ev
}

// settle places the groups the evidence is clearest for and returns which ones.
//
// A group with neighbors above and below goes between them, on a tier of its own
// when one is free: that never leaves a node with all its links on one side while
// there was room in the middle. Only when no group is in that position are groups
// with neighbors in a single tier placed, one tier further down, which is where
// hosts hang off a switch. A group that still has unplaced neighbors waits for
// them as long as anything else can be placed first.
func (l *Layout) settle(t []int, open [][]int) map[int]bool {
	var between, below, waiting []int

	evs := make([]evidence, len(open))

	for i, group := range open {
		evs[i] = l.evidence(t, group)

		switch ev := evs[i]; {
		case ev.n == 0:
		case ev.lo < ev.hi:
			between = append(between, i)
		case !ev.pending:
			below = append(below, i)
		default:
			waiting = append(waiting, i)
		}
	}

	settled := map[int]bool{}

	place := func(i, tier int) {
		for _, n := range open[i] {
			t[n] = tier
		}

		settled[i] = true
	}

	switch {
	case len(between) > 0:
		for _, i := range between {
			ev := evs[i]
			tier := int(math.Round(float64(ev.sum) / float64(ev.n)))

			if ev.hi-ev.lo > 1 {
				tier = min(max(tier, ev.lo+1), ev.hi-1)
			}

			place(i, tier)
		}
	case len(below) > 0:
		for _, i := range below {
			place(i, evs[i].lo+1)
		}
	case len(waiting) > 0:
		place(waiting[0], evs[waiting[0]].lo+1)
	}

	return settled
}

// tierRows turns the tiers of one component into consecutive rows.
func (l *Layout) tierRows(comp []int) [][]int {
	byTier := map[int][]int{}

	for _, n := range comp {
		byTier[l.tier[n]] = append(byTier[l.tier[n]], n)
	}

	keys := make([]int, 0, len(byTier))
	for k := range byTier {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	rows := make([][]int, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, l.byGroup(byTier[k]))
	}

	return rows
}

// componentUnits builds one drawing per connected component and returns the nodes
// without any link separately.
func (l *Layout) componentUnits() (units []*unit, loose []int) {
	for _, comp := range l.components() {
		if len(comp) == 1 {
			loose = append(loose, comp[0])
			continue
		}

		var (
			rows  [][]int
			fixed bool
		)

		if l.mode == ModeTiers && l.tier[comp[0]] >= 0 {
			rows = l.tierRows(comp)
		} else {
			rows, fixed = l.graphRows(comp)
		}

		units = append(units, l.newUnit(rows, nil, fixed))
	}

	return units, loose
}
