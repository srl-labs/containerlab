// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// sparkWidth is the width of the sparklines shown in table columns.
	sparkWidth = 8

	// CPU load is colored by these thresholds, in percent of one core.
	cpuWarn = 50
	cpuHigh = 85
)

func (m *Model) cell(text string) clabinternaltuiui.Cell {
	return clabinternaltuiui.Cell{Text: clabinternaltuiui.Sanitize(text), Style: m.th.Text}
}

func (m *Model) mutedCell(text string) clabinternaltuiui.Cell {
	return clabinternaltuiui.Cell{Text: clabinternaltuiui.Sanitize(text), Style: m.th.Muted}
}

func (m *Model) levelCell(text string, level clabinternaltuitheme.Level) clabinternaltuiui.Cell {
	return clabinternaltuiui.Cell{Text: text, Style: m.th.LevelStyle(level)}
}

func (m *Model) styledCell(text string, style lipgloss.Style) clabinternaltuiui.Cell {
	return clabinternaltuiui.Cell{Text: clabinternaltuiui.Sanitize(text), Style: style}
}

// dash is the cell of a value that does not apply.
func (m *Model) dash() clabinternaltuiui.Cell {
	return clabinternaltuiui.Cell{Text: clabinternaltuiui.Dash, Style: m.th.Faint, Num: -1}
}

// rateCell shows a traffic rate. Idle interfaces are dimmed, so busy ones stand out.
func (m *Model) rateCell(bps float64, has bool) clabinternaltuiui.Cell {
	if !has {
		return m.dash()
	}

	style := m.th.Text
	if bps < 1 {
		style = m.th.Muted
	}

	return clabinternaltuiui.Cell{Text: clabinternaltuiui.Rate(bps), Style: style, Num: bps}
}

func cpuLevel(percent float64) clabinternaltuitheme.Level {
	switch {
	case percent >= cpuHigh:
		return clabinternaltuitheme.LevelError
	case percent >= cpuWarn:
		return clabinternaltuitheme.LevelWarning
	default:
		return clabinternaltuitheme.LevelSuccess
	}
}

func (m *Model) cpuCell(percent float64, has bool) clabinternaltuiui.Cell {
	if !has {
		return m.dash()
	}

	return clabinternaltuiui.Cell{
		Text:  clabinternaltuiui.Percent(percent),
		Style: m.th.LevelStyle(cpuLevel(percent)),
		Num:   percent,
	}
}

func (m *Model) memCell(bytes uint64, has bool) clabinternaltuiui.Cell {
	if !has {
		return m.dash()
	}

	return clabinternaltuiui.Cell{
		Text:  clabinternaltuiui.Bytes(bytes),
		Style: m.th.Text,
		Num:   float64(bytes),
	}
}

// cpuScale is the load a full bar of a CPU history stands for, in percent of one core.
// Histories that go beyond it scale to their peak.
const cpuScale = 100

// cpuLimit returns the scale of a CPU history for a number of nodes.
func cpuLimit(series []float64, nodes int) float64 {
	limit := float64(cpuScale * max(1, nodes))
	for _, v := range series {
		limit = max(limit, v)
	}

	return limit
}

// sparkCell draws the recent history of a value. A limit that is not positive scales the
// bars to the highest value shown.
func (m *Model) sparkCell(
	series []float64,
	limit float64,
	style lipgloss.Style,
) clabinternaltuiui.Cell {
	if len(series) == 0 {
		return clabinternaltuiui.Cell{}
	}

	return clabinternaltuiui.Cell{
		Text:  clabinternaltuiui.Sparkline(series, sparkWidth, limit),
		Style: style,
	}
}

// sumSeries adds up histories sample by sample, aligned at their most recent value.
func sumSeries(series ...[]float64) []float64 {
	longest := 0
	for _, s := range series {
		longest = max(longest, len(s))
	}

	if longest == 0 {
		return nil
	}

	sum := make([]float64, longest)

	for _, s := range series {
		offset := longest - len(s)
		for i, v := range s {
			sum[offset+i] += v
		}
	}

	return sum
}

// kindSummary counts the nodes of a lab by kind, ie. "6 nokia_srlinux · 4 linux".
func kindSummary(nodes []clabinternaltuibackend.Node) string {
	counts := map[string]int{}

	var order []string

	for i := range nodes {
		kind := nodes[i].Kind
		if counts[kind] == 0 {
			order = append(order, kind)
		}

		counts[kind]++
	}

	parts := make([]string, 0, len(order))
	for _, kind := range order {
		parts = append(parts, itoa(counts[kind])+" "+kind)
	}

	return strings.Join(parts, " · ")
}

// uptimeRank orders the units of a compact uptime, so that the longest can be found.
//
//nolint:mnd // ranks
var uptimeRank = map[string]int{"s": 1, "m": 2, "h": 3, "d": 4, "w": 5, "mo": 6, "y": 7}

// labAge returns the uptime of the longest running node of a lab.
func labAge(lab *clabinternaltuibackend.Lab) string {
	best, bestRank, bestAmount := "", 0, 0

	for i := range lab.Nodes {
		up := lab.Nodes[i].Uptime()
		if up == "" {
			continue
		}

		digits := strings.TrimRight(up, "smhdwoy")
		rank, amount := uptimeRank[up[len(digits):]], atoi(digits)

		if rank > bestRank || (rank == bestRank && amount > bestAmount) {
			best, bestRank, bestAmount = up, rank, amount
		}
	}

	return best
}

func itoa(n int) string { return strconv.Itoa(n) }

// atoi parses a number, returning zero for anything else.
func atoi(s string) int {
	n, _ := strconv.Atoi(s)

	return n
}
