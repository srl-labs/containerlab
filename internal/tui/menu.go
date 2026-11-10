// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// menuRows is the number of entries a menu shows at once.
	menuRows = 12
	// menuMinWidth keeps a menu with short entries from collapsing.
	menuMinWidth = 40
	// menuRowOverhead is the width of an entry besides its label: the marker and the gap.
	menuRowOverhead = 4
)

// menuItem is an entry of a menu.
type menuItem struct {
	// shortcut is the key that does the same without the menu, shown in front.
	shortcut string
	label    string
	desc     string
	// disabled explains why the entry cannot be chosen, empty when it can.
	disabled string
	run      func() tea.Cmd
}

// menuDialog is a list to choose from. Entries with a shortcut are chosen by pressing it,
// which makes a menu two key presses from the selection to the action: the key that opens
// it and the key of the entry. Long lists are narrowed down by typing instead; text is
// matched fuzzily and the best match comes first.
type menuDialog struct {
	title   string
	items   []menuItem
	input   textinput.Model
	matches []clabinternaltuiui.FuzzyMatch
	cursor  int
	offset  int

	// keyed is set for menus whose entries have shortcuts. Such a menu takes key
	// presses as shortcuts until the user starts a search.
	keyed bool
	// searching is set while typed text narrows the list down.
	searching bool
	// firstRow is the line of the dialog the first entry was last drawn on.
	firstRow int
}

// choose opens a menu.
func (m *Model) choose(title string, items []menuItem) tea.Cmd {
	in := textinput.New()
	in.Prompt = "❯ "
	in.Placeholder = "type to search"
	in.CharLimit = promptCharLimit
	in.SetWidth(menuMinWidth)

	styles := textinput.DefaultStyles(m.th.Dark)
	styles.Focused.Prompt = m.th.Key
	styles.Focused.Text = m.th.Text
	styles.Focused.Placeholder = m.th.Faint
	in.SetStyles(styles)

	d := &menuDialog{title: title, items: items, input: in}

	for i := range items {
		if items[i].shortcut != "" {
			d.keyed = true
		}
	}

	d.searching = !d.keyed
	d.refilter()
	m.open(d)

	if d.searching {
		return d.input.Focus()
	}

	return nil
}

// numbered gives the entries of a short menu the digits as shortcuts, in their order.
func numbered(items []menuItem) []menuItem {
	const digits = "123456789"

	for i := range items {
		if i < len(digits) && items[i].shortcut == "" {
			items[i].shortcut = digits[i : i+1]
		}
	}

	return items
}

func (d *menuDialog) labels() []string {
	labels := make([]string, len(d.items))
	for i := range d.items {
		labels[i] = d.items[i].label
	}

	return labels
}

func (d *menuDialog) refilter() {
	d.matches = clabinternaltuiui.FuzzyFind(d.input.Value(), d.labels())
	d.cursor, d.offset = 0, 0
}

func (d *menuDialog) move(delta int) {
	d.cursor = max(0, min(d.cursor+delta, len(d.matches)-1))

	if d.cursor < d.offset {
		d.offset = d.cursor
	}

	if d.cursor >= d.offset+menuRows {
		d.offset = d.cursor - menuRows + 1
	}
}

func (d *menuDialog) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	key := msg.String()

	switch key {
	case keyEsc:
		// Leaving a search of a menu with shortcuts goes back to the shortcuts.
		if d.keyed && d.searching {
			d.searching = false
			d.input.Blur()
			d.input.SetValue("")
			d.refilter()

			return false, nil
		}

		return true, nil
	case "up", "ctrl+p":
		d.move(-1)

		return false, nil
	case "down", "ctrl+n":
		d.move(1)

		return false, nil
	case "pgup":
		d.move(-menuRows)

		return false, nil
	case "pgdown":
		d.move(menuRows)

		return false, nil
	case "home":
		d.move(-len(d.matches))

		return false, nil
	case "end":
		d.move(len(d.matches))

		return false, nil
	case keyEnter:
		return d.run(m)
	}

	if !d.searching {
		return d.shortcut(m, key)
	}

	before := d.input.Value()

	var cmd tea.Cmd

	d.input, cmd = d.input.Update(msg)

	if d.input.Value() != before {
		d.refilter()
	}

	return false, cmd
}

// shortcut handles a key press in a menu with shortcuts: the key of an entry chooses it,
// a slash starts a search, and the keys that move the cursor in lists do so here.
func (d *menuDialog) shortcut(m *Model, key string) (bool, tea.Cmd) {
	for i, match := range d.matches {
		if d.items[match.Index].shortcut == key {
			d.cursor = i

			return d.run(m)
		}
	}

	switch key {
	case "/":
		d.searching = true

		return false, d.input.Focus()
	case "j":
		d.move(1)
	case "k":
		d.move(-1)
	}

	return false, nil
}

func (d *menuDialog) run(m *Model) (bool, tea.Cmd) {
	if d.cursor >= len(d.matches) {
		return false, nil
	}

	item := &d.items[d.matches[d.cursor].Index]
	if item.disabled != "" {
		m.notifyf(clabinternaltuitheme.LevelWarning, "%s: %s", item.label, item.disabled)

		return false, nil
	}

	if item.run == nil {
		return true, nil
	}

	return true, item.run()
}

func (d *menuDialog) update(_ *Model, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	d.input, cmd = d.input.Update(msg)

	return cmd
}

func (d *menuDialog) paste(text string) {
	if !d.searching {
		return
	}

	d.input, _ = d.input.Update(tea.PasteMsg{Content: firstLine(text)})
	d.refilter()
}

func (d *menuDialog) hints() []hint {
	if d.keyed && !d.searching {
		return []hint{{"key", "choose"}, {"↑↓", "move"}, {"⏎", "select"}, {"/", "search"},
			{keyEsc, "cancel"}}
	}

	return []hint{{"↑↓", "move"}, {"⏎", "select"}, {keyEsc, "cancel"}}
}

// click chooses the entry that was clicked.
func (d *menuDialog) click(m *Model, _, y int) (bool, tea.Cmd) {
	i := d.offset + y - d.firstRow
	if y < d.firstRow || i >= min(len(d.matches), d.offset+menuRows) {
		return false, nil
	}

	d.cursor = i

	return d.run(m)
}

// scroll moves the cursor with the mouse wheel.
func (d *menuDialog) scroll(notches int) { d.move(notches) }

// modal reports that a click next to the menu closes it: nothing typed is lost.
func (*menuDialog) modal() bool { return false }

func (d *menuDialog) view(m *Model, maxWidth, _ int) string {
	labelWidth, keyWidth := 0, 0
	for i := range d.items {
		labelWidth = max(labelWidth, clabinternaltuiui.Width(d.items[i].label))
		keyWidth = max(keyWidth, clabinternaltuiui.Width(d.items[i].shortcut))
	}

	width := min(maxWidth, dialogMaxWidth) - clabinternaltuiui.FrameOverhead

	var lines []string

	if d.searching {
		lines = append(lines, d.input.View(), "")
	}

	// The entries start below the top edge of the frame and what was put above them.
	d.firstRow = len(lines) + 1

	end := min(len(d.matches), d.offset+menuRows)
	for i := d.offset; i < end; i++ {
		lines = append(lines, d.row(m, i, keyWidth, labelWidth, width))
	}

	if len(d.matches) == 0 {
		lines = append(lines, m.th.Muted.Render("  nothing matches"))
	}

	if len(d.matches) > menuRows {
		lines = append(lines, m.th.Faint.Render(
			"  "+itoa(d.offset+1)+"–"+itoa(end)+" of "+itoa(len(d.matches))))
	}

	// Keep the box from changing its width while the list is narrowed down.
	keep := min(width, max(menuMinWidth, labelWidth+menuRowOverhead))
	lines = append(lines, strings.Repeat(" ", keep))

	return clabinternaltuiui.Frame(d.title, strings.Join(lines, "\n"), maxWidth,
		m.th.FocusBorder, m.th.Title)
}

func (d *menuDialog) row(m *Model, i, keyWidth, labelWidth, width int) string {
	match := d.matches[i]
	item := &d.items[match.Index]

	marker := "  "
	base, key := m.th.Text, m.th.Key

	if i == d.cursor {
		marker = m.th.Accent.Render("▸ ")
		base = m.th.Bold
	}

	desc := item.desc

	if item.disabled != "" {
		base, key = m.th.Faint, m.th.Faint
		desc = item.disabled
	}

	label := clabinternaltuiui.HighlightRunes(item.label, match.Runes, base, m.th.Match)
	pad := strings.Repeat(" ", max(0, labelWidth-clabinternaltuiui.Width(item.label)))
	line := marker

	if keyWidth > 0 {
		line += key.Render(clabinternaltuiui.PadRight(item.shortcut, keyWidth)) + "  "
	}

	line += label + pad

	if desc != "" {
		line += "  " + m.th.Muted.Render(desc)
	}

	return clabinternaltuiui.Truncate(line, width)
}
