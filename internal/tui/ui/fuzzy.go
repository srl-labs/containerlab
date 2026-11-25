// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
)

// FuzzyMatch is an item that matched a fuzzy query.
type FuzzyMatch struct {
	// Index is the position of the item in the list that was searched.
	Index int
	// Runes are the positions of the matched characters, counted in runes.
	Runes []int
}

// FuzzyFind returns the items that match the query, the best match first. Items that
// contain the query as it was typed come before the ones that only match it loosely, with
// characters in between: what the user spelled out is what they most likely mean. An
// empty query matches every item, in their original order.
func FuzzyFind(query string, items []string) []FuzzyMatch {
	query = strings.TrimSpace(query)

	if query == "" {
		all := make([]FuzzyMatch, len(items))
		for i := range items {
			all[i] = FuzzyMatch{Index: i}
		}

		return all
	}

	needle := []rune(strings.ToLower(query))
	exact := map[int]int{}

	var matches []FuzzyMatch

	for i, item := range items {
		at := indexRunes([]rune(strings.ToLower(item)), needle, 0)
		if at < 0 {
			continue
		}

		positions := make([]int, len(needle))
		for k := range needle {
			positions[k] = at + k
		}

		exact[i] = at
		matches = append(matches, FuzzyMatch{Index: i, Runes: positions})
	}

	// Among the items containing the query, an earlier occurrence and a shorter item
	// make the better match.
	sort.SliceStable(matches, func(a, b int) bool {
		ia, ib := matches[a].Index, matches[b].Index
		if exact[ia] != exact[ib] {
			return exact[ia] < exact[ib]
		}

		return len(items[ia]) < len(items[ib])
	})

	for _, f := range fuzzy.Find(query, items) {
		if _, found := exact[f.Index]; found {
			continue
		}

		matches = append(matches, FuzzyMatch{
			Index: f.Index,
			Runes: runePositions(items[f.Index], f.MatchedIndexes),
		})
	}

	return matches
}

// runePositions converts byte offsets into a string to rune positions. The matcher
// reports the former, while text is cut and styled by the latter.
func runePositions(s string, offsets []int) []int {
	positions := make([]int, 0, len(offsets))

	for _, off := range offsets {
		if off < 0 || off > len(s) {
			continue
		}

		positions = append(positions, utf8.RuneCountInString(s[:off]))
	}

	return positions
}

// HighlightRunes renders s in the base style, with the runes at the given positions in
// the match style. Positions beyond the end of s, as left by a truncation, are ignored.
func HighlightRunes(s string, positions []int, base, match lipgloss.Style) string {
	if len(positions) == 0 {
		return base.Render(s)
	}

	runes := []rune(s)
	marked := make([]bool, len(runes))

	for _, p := range positions {
		if p >= 0 && p < len(runes) {
			marked[p] = true
		}
	}

	var b strings.Builder

	for start := 0; start < len(runes); {
		end := start
		for end < len(runes) && marked[end] == marked[start] {
			end++
		}

		if marked[start] {
			b.WriteString(match.Render(string(runes[start:end])))
		} else {
			b.WriteString(base.Render(string(runes[start:end])))
		}

		start = end
	}

	return b.String()
}
