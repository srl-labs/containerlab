// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// framePadding is the blank space between the border of a frame and its content.
const framePadding = 1

// FrameOverhead is the width a frame adds around its content: the two borders and the
// padding on each side.
const FrameOverhead = 2 + 2*framePadding

// Frame draws a rounded border around a block of text, with a title set into the top
// edge. The content is padded to a common width; lines wider than maxWidth are cut.
func Frame(title, content string, maxWidth int, border, titleStyle lipgloss.Style) string {
	lines := strings.Split(content, "\n")

	inner := Width(title) + 2 //nolint:mnd // a dash on each side of the title
	for _, line := range lines {
		inner = max(inner, Width(line))
	}

	inner = max(1, min(inner, maxWidth-FrameOverhead))
	title = Truncate(title, inner-2)     //nolint:mnd // see above
	width := inner + 2*framePadding      //nolint:mnd // between the borders
	rest := max(0, width-Width(title)-3) //nolint:mnd // "─ " and " " around the title
	pad := strings.Repeat(" ", framePadding)

	var b strings.Builder

	b.WriteString(border.Render("╭─ ") + titleStyle.Render(title) + " " +
		border.Render(strings.Repeat("─", rest)+"╮") + "\n")

	for _, line := range lines {
		b.WriteString(
			border.Render("│") + pad + PadRight(line, inner) + pad + border.Render("│") + "\n",
		)
	}

	b.WriteString(border.Render("╰" + strings.Repeat("─", width) + "╯"))

	return b.String()
}
