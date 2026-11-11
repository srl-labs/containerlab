// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package theme defines the color themes and the styles of the containerlab TUI.
package theme

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Auto is the name of the theme that follows the terminal: it keeps the terminal's
// background and picks light or dark colors to match it.
const Auto = "auto"

// Theme holds the palette of a color theme together with the styles derived from it.
type Theme struct {
	ID   string
	Name string
	Dark bool
	Palette

	// Text styles.
	Text   lipgloss.Style
	Muted  lipgloss.Style
	Faint  lipgloss.Style
	Bold   lipgloss.Style
	Accent lipgloss.Style
	Title  lipgloss.Style

	// Status styles.
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	// Table styles.
	TableHeader lipgloss.Style
	Selected    lipgloss.Style
	Match       lipgloss.Style
	Mark        lipgloss.Style

	// Chrome.
	Border      lipgloss.Style
	FocusBorder lipgloss.Style
	Chip        lipgloss.Style
	ChipActive  lipgloss.Style
	Key         lipgloss.Style
	KeyDesc     lipgloss.Style
}

// Names returns the names accepted by New, the automatic theme first.
func Names() []string {
	return []string{Auto, defaultDark.id, defaultLight.id}
}

// New builds the theme with the given name. The automatic theme, also selected by an
// empty name, resolves to the dark or light theme depending on darkBackground. The other
// two names force one of them, for a terminal that does not tell its background.
func New(name string, darkBackground bool) (*Theme, error) {
	name = strings.ToLower(strings.TrimSpace(name))

	switch name {
	case "", Auto:
		if darkBackground {
			return build(&defaultDark), nil
		}

		return build(&defaultLight), nil
	case defaultDark.id:
		return build(&defaultDark), nil
	case defaultLight.id:
		return build(&defaultLight), nil
	}

	return nil, fmt.Errorf("unknown theme %q, available themes: %s",
		name, strings.Join(Names(), ", "))
}

// Default returns the default theme for a dark or light terminal.
func Default(darkBackground bool) *Theme {
	t, _ := New(Auto, darkBackground)

	return t
}

// IsAuto reports whether the theme name follows the terminal's color scheme.
func IsAuto(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))

	return name == "" || name == Auto
}

func build(d *definition) *Theme {
	p := d.palette

	// Plain text keeps the terminal's own color.
	base := lipgloss.NewStyle()

	t := &Theme{
		ID:      d.id,
		Name:    d.name,
		Dark:    d.dark,
		Palette: p,

		Text:   base,
		Muted:  lipgloss.NewStyle().Foreground(p.Muted),
		Faint:  lipgloss.NewStyle().Foreground(p.Border),
		Bold:   base.Bold(true),
		Accent: lipgloss.NewStyle().Foreground(p.Accent),
		Title:  lipgloss.NewStyle().Foreground(p.Accent).Bold(true),

		Success: lipgloss.NewStyle().Foreground(p.Success),
		Warning: lipgloss.NewStyle().Foreground(p.Warning),
		Error:   lipgloss.NewStyle().Foreground(p.Error),
		Info:    lipgloss.NewStyle().Foreground(p.Info),

		TableHeader: lipgloss.NewStyle().Foreground(p.Muted).Bold(true),
		Selected:    base.Background(p.Selection).Bold(true),
		Match:       lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Underline(true),
		Mark:        lipgloss.NewStyle().Foreground(p.Warning).Bold(true),

		Border:      lipgloss.NewStyle().Foreground(p.Border),
		FocusBorder: lipgloss.NewStyle().Foreground(p.Accent),
		Chip:        base.Background(p.Raised).Padding(0, 1),
		ChipActive: lipgloss.NewStyle().
			Foreground(p.OnAccent).Background(p.Accent).Bold(true).Padding(0, 1),
		Key:     lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		KeyDesc: lipgloss.NewStyle().Foreground(p.Muted),
	}

	return t
}

// familyHues pins the families of node kinds seen most often to a hue each, so that they
// are told apart at a glance and look the same in every lab. The values index Hues.
//
//nolint:mnd // indexes into Hues
var familyHues = map[string]int{
	"linux":   0,
	"cisco":   1,
	"juniper": 3,
	"arista":  4,
	"nokia":   5,
	"bridge":  7,
}

// Hue returns a stable color for a category name, ie. a node kind. Names sharing the
// part before the first underscore, such as the kinds of one vendor, get the same color.
func (t *Theme) Hue(name string) color.Color {
	if len(t.Hues) == 0 {
		return t.Palette.Accent
	}

	family, _, _ := strings.Cut(strings.ToLower(name), "_")

	if i, ok := familyHues[family]; ok {
		return t.Hues[i%len(t.Hues)]
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(family))

	return t.Hues[int(h.Sum32()%uint32(len(t.Hues)))] //nolint:gosec // bounded by the modulo
}

// Level identifies a severity, used to color states and messages.
type Level int

// Severity levels.
const (
	LevelNone Level = iota
	LevelSuccess
	LevelWarning
	LevelError
	LevelInfo
	LevelMuted
)

// LevelStyle returns the text style of a severity level.
func (t *Theme) LevelStyle(l Level) lipgloss.Style {
	switch l {
	case LevelSuccess:
		return t.Success
	case LevelWarning:
		return t.Warning
	case LevelError:
		return t.Error
	case LevelInfo:
		return t.Info
	case LevelMuted:
		return t.Muted
	default:
		return t.Text
	}
}
