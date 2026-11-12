// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is the handful of colors a theme is designed around. Every style of the UI is
// derived from it. It has no background and no text color: those stay the terminal's own.
type Palette struct {
	// Raised is the background of chips and inputs.
	Raised color.Color
	Border color.Color
	Muted  color.Color
	// Accent is the brand color, used for focus, titles and the active elements, and
	// OnAccent the text color that reads well on top of it.
	Accent   color.Color
	OnAccent color.Color
	// Selection is the background of the row under the cursor.
	Selection color.Color

	Error   color.Color
	Warning color.Color
	Info    color.Color
	Success color.Color

	// Hues are used to tell categories apart, ie. node kinds.
	Hues []color.Color
}

// definition describes a built-in theme.
type definition struct {
	id      string
	name    string
	dark    bool
	palette Palette
}

func hex(s string) color.Color { return lipgloss.Color(s) }

func hues(values ...string) []color.Color {
	out := make([]color.Color, 0, len(values))
	for _, v := range values {
		out = append(out, hex(v))
	}

	return out
}

// The themes leave the terminal's background and foreground alone, so that the UI blends
// in with whatever color scheme the terminal uses.
var (
	defaultDark = definition{
		id:   "dark",
		name: "Containerlab Dark",
		dark: true,
		palette: Palette{
			Raised:    hex("#2b323b"),
			Border:    hex("#4a535e"),
			Muted:     hex("#8b949e"),
			Accent:    hex("#00c9ff"),
			OnAccent:  hex("#00202a"),
			Selection: hex("#123a4b"),
			Error:     hex("#f85149"),
			Warning:   hex("#d29922"),
			Info:      hex("#58a6ff"),
			Success:   hex("#3fb950"),
			Hues: hues(
				"#39c5cf", "#bc8cff", "#e3b341", "#56d364",
				"#ff9bce", "#79c0ff", "#ffa657", "#a5d6ff",
			),
		},
	}

	defaultLight = definition{
		id:   "light",
		name: "Containerlab Light",
		palette: Palette{
			Raised:    hex("#e6ebf0"),
			Border:    hex("#b6bfc9"),
			Muted:     hex("#59636e"),
			Accent:    hex("#007a9e"),
			OnAccent:  hex("#ffffff"),
			Selection: hex("#cfe8f3"),
			Error:     hex("#cf222e"),
			Warning:   hex("#9a6700"),
			Info:      hex("#0969da"),
			Success:   hex("#1a7f37"),
			Hues: hues(
				"#1b7c83", "#8250df", "#9a6700", "#1a7f37",
				"#bf3989", "#0969da", "#bc4c00", "#0550ae",
			),
		},
	}
)
