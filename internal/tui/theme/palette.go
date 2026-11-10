// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is the handful of colors a theme is designed around. Every style of the UI is
// derived from it.
type Palette struct {
	// Background is the screen background. It is nil for themes that keep the terminal's
	// own background.
	Background color.Color
	// Surface is the background of bars and panels, Raised the one of chips and inputs.
	Surface color.Color
	Raised  color.Color
	Border  color.Color
	// Foreground is the default text color. It is nil for themes that keep the terminal's
	// own foreground.
	Foreground color.Color
	Muted      color.Color
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

// The default themes leave the terminal's background and foreground alone, so that the
// UI blends in with whatever color scheme the terminal uses.
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

// definitions lists the named themes. They mirror the themes of the containerlab app and
// paint the whole screen with their own background.
var definitions = []definition{
	{
		id: "github-dark", name: "GitHub Dark", dark: true,
		palette: Palette{
			Background: hex("#0d1117"), Surface: hex("#010409"), Raised: hex("#21262d"),
			Border: hex("#30363d"), Foreground: hex("#e6edf3"), Muted: hex("#9198a1"),
			Accent: hex("#4493f8"), OnAccent: hex("#0d1117"), Selection: hex("#1f3a5f"),
			Error: hex("#f85149"), Warning: hex("#d29922"), Info: hex("#58a6ff"),
			Success: hex("#3fb950"),
			Hues: hues("#39c5cf", "#bc8cff", "#e3b341", "#56d364",
				"#ff7b72", "#79c0ff", "#ffa198", "#d2a8ff"),
		},
	},
	{
		id: "one-dark", name: "One Dark", dark: true,
		palette: Palette{
			Background: hex("#282c34"), Surface: hex("#21252b"), Raised: hex("#333842"),
			Border: hex("#3a3f4b"), Foreground: hex("#abb2bf"), Muted: hex("#8b919c"),
			Accent: hex("#61afef"), OnAccent: hex("#21252b"), Selection: hex("#3e4451"),
			Error: hex("#e06c75"), Warning: hex("#e5c07b"), Info: hex("#61afef"),
			Success: hex("#98c379"),
			Hues: hues("#56b6c2", "#c678dd", "#e5c07b", "#98c379",
				"#e06c75", "#61afef", "#d19a66", "#abb2bf"),
		},
	},
	{
		id: "dracula", name: "Dracula", dark: true,
		palette: Palette{
			Background: hex("#282a36"), Surface: hex("#21222c"), Raised: hex("#343746"),
			Border: hex("#3c3f52"), Foreground: hex("#f8f8f2"), Muted: hex("#a0a6c6"),
			Accent: hex("#bd93f9"), OnAccent: hex("#21222c"), Selection: hex("#44475a"),
			Error: hex("#ff5555"), Warning: hex("#f1fa8c"), Info: hex("#8be9fd"),
			Success: hex("#50fa7b"),
			Hues: hues("#8be9fd", "#ff79c6", "#f1fa8c", "#50fa7b",
				"#ffb86c", "#bd93f9", "#ff6e6e", "#a4ffff"),
		},
	},
	{
		id: "nord", name: "Nord", dark: true,
		palette: Palette{
			Background: hex("#2e3440"), Surface: hex("#272c36"), Raised: hex("#3b4252"),
			Border: hex("#434c5e"), Foreground: hex("#d8dee9"), Muted: hex("#9aa5b8"),
			Accent: hex("#88c0d0"), OnAccent: hex("#2e3440"), Selection: hex("#434c5e"),
			Error: hex("#bf616a"), Warning: hex("#ebcb8b"), Info: hex("#81a1c1"),
			Success: hex("#a3be8c"),
			Hues: hues("#8fbcbb", "#b48ead", "#ebcb8b", "#a3be8c",
				"#d08770", "#81a1c1", "#bf616a", "#88c0d0"),
		},
	},
	{
		id: "tokyo-night", name: "Tokyo Night", dark: true,
		palette: Palette{
			Background: hex("#1a1b26"), Surface: hex("#16161e"), Raised: hex("#292e42"),
			Border: hex("#343a55"), Foreground: hex("#a9b1d6"), Muted: hex("#787c99"),
			Accent: hex("#7aa2f7"), OnAccent: hex("#1a1b26"), Selection: hex("#283457"),
			Error: hex("#f7768e"), Warning: hex("#e0af68"), Info: hex("#7aa2f7"),
			Success: hex("#9ece6a"),
			Hues: hues("#7dcfff", "#bb9af7", "#e0af68", "#9ece6a",
				"#ff9e64", "#7aa2f7", "#f7768e", "#a4daff"),
		},
	},
	{
		id: "gruvbox-dark", name: "Gruvbox Dark", dark: true,
		palette: Palette{
			Background: hex("#282828"), Surface: hex("#1d2021"), Raised: hex("#3c3836"),
			Border: hex("#504945"), Foreground: hex("#ebdbb2"), Muted: hex("#a89984"),
			Accent: hex("#d79921"), OnAccent: hex("#282828"), Selection: hex("#504945"),
			Error: hex("#fb4934"), Warning: hex("#fabd2f"), Info: hex("#83a598"),
			Success: hex("#b8bb26"),
			Hues: hues("#8ec07c", "#d3869b", "#fabd2f", "#b8bb26",
				"#fe8019", "#83a598", "#fb4934", "#689d6a"),
		},
	},
	{
		id: "catppuccin-mocha", name: "Catppuccin Mocha", dark: true,
		palette: Palette{
			Background: hex("#1e1e2e"), Surface: hex("#181825"), Raised: hex("#313244"),
			Border: hex("#45475a"), Foreground: hex("#cdd6f4"), Muted: hex("#a6adc8"),
			Accent: hex("#cba6f7"), OnAccent: hex("#11111b"), Selection: hex("#45475a"),
			Error: hex("#f38ba8"), Warning: hex("#f9e2af"), Info: hex("#89b4fa"),
			Success: hex("#a6e3a1"),
			Hues: hues("#94e2d5", "#f5c2e7", "#f9e2af", "#a6e3a1",
				"#fab387", "#89b4fa", "#f38ba8", "#74c7ec"),
		},
	},
	{
		id: "monokai", name: "Monokai", dark: true,
		palette: Palette{
			Background: hex("#272822"), Surface: hex("#1e1f1c"), Raised: hex("#3e3d32"),
			Border: hex("#414339"), Foreground: hex("#f8f8f2"), Muted: hex("#a59f85"),
			Accent: hex("#a6e22e"), OnAccent: hex("#272822"), Selection: hex("#49483e"),
			Error: hex("#f92672"), Warning: hex("#e6db74"), Info: hex("#66d9ef"),
			Success: hex("#a6e22e"),
			Hues: hues("#a1efe4", "#ae81ff", "#e6db74", "#a6e22e",
				"#fd971f", "#66d9ef", "#f92672", "#f8f8f2"),
		},
	},
	{
		id: "solarized-dark", name: "Solarized Dark", dark: true,
		palette: Palette{
			Background: hex("#002b36"), Surface: hex("#00212b"), Raised: hex("#073642"),
			Border: hex("#0e4250"), Foreground: hex("#93a1a1"), Muted: hex("#839496"),
			Accent: hex("#268bd2"), OnAccent: hex("#fdf6e3"), Selection: hex("#274642"),
			Error: hex("#dc322f"), Warning: hex("#b58900"), Info: hex("#268bd2"),
			Success: hex("#859900"),
			Hues: hues("#2aa198", "#d33682", "#b58900", "#859900",
				"#cb4b16", "#268bd2", "#dc322f", "#6c71c4"),
		},
	},
	{
		id: "github-light", name: "GitHub Light",
		palette: Palette{
			Background: hex("#ffffff"), Surface: hex("#f6f8fa"), Raised: hex("#eaeef2"),
			Border: hex("#d0d7de"), Foreground: hex("#1f2328"), Muted: hex("#59636e"),
			Accent: hex("#0969da"), OnAccent: hex("#ffffff"), Selection: hex("#cfe3fb"),
			Error: hex("#cf222e"), Warning: hex("#9a6700"), Info: hex("#0969da"),
			Success: hex("#1a7f37"),
			Hues: hues("#1b7c83", "#8250df", "#9a6700", "#1a7f37",
				"#bc4c00", "#0550ae", "#a40e26", "#6639ba"),
		},
	},
	{
		id: "solarized-light", name: "Solarized Light",
		palette: Palette{
			Background: hex("#fdf6e3"), Surface: hex("#eee8d5"), Raised: hex("#e4ddc8"),
			Border: hex("#ddd6c1"), Foreground: hex("#586e75"), Muted: hex("#657b83"),
			Accent: hex("#268bd2"), OnAccent: hex("#fdf6e3"), Selection: hex("#e7dfc6"),
			Error: hex("#dc322f"), Warning: hex("#9a7500"), Info: hex("#268bd2"),
			Success: hex("#6c7c00"),
			Hues: hues("#2aa198", "#d33682", "#b58900", "#859900",
				"#cb4b16", "#268bd2", "#dc322f", "#6c71c4"),
		},
	},
	{
		id: "gruvbox-light", name: "Gruvbox Light",
		palette: Palette{
			Background: hex("#fbf1c7"), Surface: hex("#f2e5bc"), Raised: hex("#ebdbb2"),
			Border: hex("#d5c4a1"), Foreground: hex("#3c3836"), Muted: hex("#665c54"),
			Accent: hex("#076678"), OnAccent: hex("#fbf1c7"), Selection: hex("#e2d3aa"),
			Error: hex("#9d0006"), Warning: hex("#af3a03"), Info: hex("#076678"),
			Success: hex("#79740e"),
			Hues: hues("#427b58", "#8f3f71", "#b57614", "#79740e",
				"#af3a03", "#076678", "#9d0006", "#689d6a"),
		},
	},
	{
		id: "catppuccin-latte", name: "Catppuccin Latte",
		palette: Palette{
			Background: hex("#eff1f5"), Surface: hex("#e6e9ef"), Raised: hex("#dce0e8"),
			Border: hex("#ccd0da"), Foreground: hex("#4c4f69"), Muted: hex("#5c5f77"),
			Accent: hex("#8839ef"), OnAccent: hex("#eff1f5"), Selection: hex("#d0d4e2"),
			Error: hex("#d20f39"), Warning: hex("#b35a00"), Info: hex("#1e66f5"),
			Success: hex("#2f7d1f"),
			Hues: hues("#0f7f85", "#8839ef", "#b36b00", "#40a02b",
				"#fe640b", "#1e66f5", "#d20f39", "#7287fd"),
		},
	},
	{
		id: "paper", name: "Paper",
		palette: Palette{
			Background: hex("#f5f0e6"), Surface: hex("#ece5d6"), Raised: hex("#e4dccb"),
			Border: hex("#d8cfbd"), Foreground: hex("#403b33"), Muted: hex("#665f55"),
			Accent: hex("#386b8c"), OnAccent: hex("#fffaf0"), Selection: hex("#e3d5ba"),
			Error: hex("#a63d40"), Warning: hex("#866316"), Info: hex("#386b8c"),
			Success: hex("#52783d"),
			Hues: hues("#39766f", "#795080", "#866316", "#52783d",
				"#9d741b", "#386b8c", "#a63d40", "#4a8780"),
		},
	},
}
