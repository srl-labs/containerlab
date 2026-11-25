// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

// fieldGap separates the columns of a block of fields.
const fieldGap = 3

// Field is a labeled value of a details pane.
type Field struct {
	Label string
	Value string
	// Style overrides the default text style of the value when Styled is set.
	Style  lipgloss.Style
	Styled bool
}

// Fields lays labeled values out in columns of the given number of rows and returns a
// block of exactly that many lines, each exactly width cells wide. The fields fill the
// columns top to bottom, values that do not fit their column are cut.
func Fields(th *clabinternaltuitheme.Theme, fields []Field, width, rows int) string {
	if width <= 0 || rows <= 0 {
		return ""
	}

	cols := (len(fields) + rows - 1) / rows
	if cols == 0 {
		return FitLines("", width, rows)
	}

	// Leave a one cell margin on the left, like the tables have their gutter.
	inner := width - 1
	colWidth := max(1, (inner-fieldGap*(cols-1))/cols)

	lines := make([]strings.Builder, rows)

	for c := range cols {
		column := fields[c*rows : min(len(fields), (c+1)*rows)]

		labelWidth := 0
		for i := range column {
			labelWidth = max(labelWidth, Width(column[i].Label))
		}

		labelWidth = min(labelWidth, colWidth/2) //nolint:mnd // leave room for the value

		for r := range rows {
			if c > 0 {
				lines[r].WriteString(strings.Repeat(" ", fieldGap))
			} else {
				lines[r].WriteString(" ")
			}

			if r >= len(column) {
				lines[r].WriteString(strings.Repeat(" ", colWidth))

				continue
			}

			lines[r].WriteString(renderField(th, &column[r], labelWidth, colWidth))
		}
	}

	out := make([]string, rows)
	for r := range rows {
		out[r] = lines[r].String()
	}

	return FitLines(strings.Join(out, "\n"), width, rows)
}

func renderField(th *clabinternaltuitheme.Theme, f *Field, labelWidth, width int) string {
	valueWidth := max(0, width-labelWidth-1)

	value := f.Value
	if value == "" {
		value = Dash
	}

	value = Truncate(value, valueWidth)

	style := th.Text
	if f.Styled {
		style = f.Style
	}

	return th.Muted.Render(PadRight(f.Label, labelWidth)) + " " +
		style.Render(value) + strings.Repeat(" ", max(0, valueWidth-Width(value)))
}
