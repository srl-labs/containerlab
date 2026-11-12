// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import "strings"

// sparkLevels are the bars of a sparkline, from the lowest to the highest.
var sparkLevels = []rune("▁▂▃▄▅▆▇█")

// Sparkline draws the most recent values as a row of bars exactly width cells wide. The
// bars are scaled to limit, or to the highest value shown when limit is not positive.
// Missing history is left blank on the left, so that the newest value stays on the right.
func Sparkline(values []float64, width int, limit float64) string {
	if width <= 0 {
		return ""
	}

	if len(values) > width {
		values = values[len(values)-width:]
	}

	if limit <= 0 {
		for _, v := range values {
			limit = max(limit, v)
		}
	}

	var b strings.Builder

	b.WriteString(strings.Repeat(" ", width-len(values)))

	top := len(sparkLevels) - 1

	for _, v := range values {
		level := 0
		if limit > 0 && v > 0 {
			level = min(top, int(v/limit*float64(top)+0.5)) //nolint:mnd // round to nearest
		}

		b.WriteRune(sparkLevels[level])
	}

	return b.String()
}

// barParts are the partial blocks of a horizontal bar, from one to eight eighths.
var barParts = []rune("▏▎▍▌▋▊▉█")

// Bar draws a horizontal bar exactly width cells wide filled to the given fraction, which
// is clamped to the range from zero to one. The empty part is drawn with the track rune.
func Bar(fraction float64, width int, track rune) string {
	if width <= 0 {
		return ""
	}

	fraction = min(1, max(0, fraction))

	const eighths = 8

	filled := int(fraction*float64(width*eighths) + 0.5) //nolint:mnd // round to nearest
	full, part := filled/eighths, filled%eighths

	var b strings.Builder

	b.WriteString(strings.Repeat(string(barParts[eighths-1]), full))

	used := full

	if part > 0 && used < width {
		b.WriteRune(barParts[part-1])

		used++
	}

	b.WriteString(strings.Repeat(string(track), width-used))

	return b.String()
}
