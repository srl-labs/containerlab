// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// dialogMinWidth keeps short dialogs from looking cramped, dialogMaxWidth long ones
	// from spanning the whole window.
	dialogMinWidth = 48
	dialogMaxWidth = 84
	// inputWidth is the width of the text fields of a form.
	inputWidth = 26

	buttonCancel = "Cancel"
)

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldToggle
	fieldChoice
)

// field is an input of a form.
type field struct {
	key   string
	label string
	kind  fieldKind

	input   textinput.Model
	on      bool
	choices []string
	choice  int
	// choiceNotes explain the choices, one each, next to the one that is selected.
	choiceNotes []string

	// note is shown next to a text field: its current value or what it expects.
	note string
	// validate checks the value of a text field, nil accepts anything.
	validate func(value string) error
	problem  string
}

// formValues are the values a form was submitted with.
type formValues struct {
	text  map[string]string
	flags map[string]bool
}

func (v formValues) Text(key string) string { return strings.TrimSpace(v.text[key]) }
func (v formValues) Bool(key string) bool   { return v.flags[key] }

// formDialog is a dialog made of a text, optional inputs and two buttons. With no inputs
// it is a plain confirmation.
type formDialog struct {
	title string
	body  []string
	// action is the label of the button that submits the form.
	action string
	// danger marks an action that destroys something: Cancel is then the default.
	danger bool
	fields []*field

	// preview returns the command the form would run, shown to the user.
	preview func(v formValues) string
	// check validates the values as a whole, on top of the per field checks.
	check  func(v formValues) error
	submit func(v formValues) tea.Cmd
	// onClose runs when the dialog goes away, whichever way.
	onClose func()

	// focus is the index of the focused field, followed by the two buttons.
	focus   int
	problem string
	ready   bool

	// The layout of the dialog as last drawn, for the mouse: the line each field is on
	// and the columns of the buttons, all relative to the content of the frame.
	fieldLines []int
	buttonLine int
	cancelZone [2]int
	actionZone [2]int
}

func newForm(title, action string, submit func(v formValues) tea.Cmd) *formDialog {
	return &formDialog{title: title, action: action, submit: submit}
}

func (d *formDialog) text(lines ...string) *formDialog {
	d.body = append(d.body, lines...)

	return d
}

func (d *formDialog) dangerous() *formDialog {
	d.danger = true

	return d
}

func (d *formDialog) withPreview(preview func(v formValues) string) *formDialog {
	d.preview = preview

	return d
}

func (d *formDialog) withCheck(check func(v formValues) error) *formDialog {
	d.check = check

	return d
}

func (d *formDialog) toggle(key, label string) *formDialog {
	d.fields = append(d.fields, &field{key: key, label: label, kind: fieldToggle})

	return d
}

func (d *formDialog) choice(key, label string, choices []string, selected int) *formDialog {
	d.fields = append(d.fields, &field{
		key: key, label: label, kind: fieldChoice, choices: choices, choice: selected,
	})

	return d
}

// describe adds an explanation to each choice of the field that was added last.
func (d *formDialog) describe(notes ...string) *formDialog {
	if len(d.fields) > 0 {
		d.fields[len(d.fields)-1].choiceNotes = notes
	}

	return d
}

func (d *formDialog) input(
	key, label, value, note string, validate func(string) error,
) *formDialog {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = promptCharLimit
	in.SetWidth(inputWidth)
	in.SetValue(value)

	d.fields = append(d.fields, &field{
		key: key, label: label, kind: fieldText, input: in, note: note, validate: validate,
	})

	return d
}

// closed is called when the dialog was removed from the screen.
func (d *formDialog) closed() {
	if d.onClose != nil {
		d.onClose()
	}
}

func (d *formDialog) cancelIndex() int { return len(d.fields) }
func (d *formDialog) actionIndex() int { return len(d.fields) + 1 }

// prepare sets the initial focus: the first input, or the button that is safe to hit
// enter on.
func (d *formDialog) prepare(m *Model) tea.Cmd {
	if d.ready {
		return nil
	}

	d.ready = true

	for _, f := range d.fields {
		if f.kind != fieldText {
			continue
		}

		styles := textinput.DefaultStyles(m.th.Dark)
		styles.Focused.Text = m.th.Text
		styles.Blurred.Text = m.th.Muted
		f.input.SetStyles(styles)
	}

	d.focus = d.actionIndex()

	switch {
	case len(d.fields) > 0 && d.fields[0].kind == fieldText:
		d.focus = 0
	case d.danger:
		d.focus = d.cancelIndex()
	}

	return d.syncFocus()
}

// syncFocus gives the keyboard to the focused text field and takes it from the others.
func (d *formDialog) syncFocus() tea.Cmd {
	var cmd tea.Cmd

	for i, f := range d.fields {
		if f.kind != fieldText {
			continue
		}

		if i == d.focus {
			cmd = f.input.Focus()
		} else {
			f.input.Blur()
		}
	}

	return cmd
}

func (d *formDialog) move(delta int) tea.Cmd {
	count := len(d.fields) + 2 //nolint:mnd // the fields and the two buttons
	d.focus = (d.focus + delta + count) % count

	return d.syncFocus()
}

func (d *formDialog) values() formValues {
	v := formValues{text: map[string]string{}, flags: map[string]bool{}}

	for _, f := range d.fields {
		switch f.kind {
		case fieldText:
			v.text[f.key] = f.input.Value()
		case fieldToggle:
			v.flags[f.key] = f.on
		case fieldChoice:
			v.text[f.key] = f.choices[f.choice]
		}
	}

	return v
}

// validate checks every field and the form as a whole. It moves the focus to the first
// field that has a problem.
func (d *formDialog) validate() bool {
	valid := true
	d.problem = ""

	for i, f := range d.fields {
		f.problem = ""

		if f.kind != fieldText || f.validate == nil {
			continue
		}

		if err := f.validate(strings.TrimSpace(f.input.Value())); err != nil {
			f.problem = err.Error()

			if valid {
				d.focus = i
			}

			valid = false
		}
	}

	if valid && d.check != nil {
		if err := d.check(d.values()); err != nil {
			d.problem = err.Error()
			valid = false
		}
	}

	return valid
}

func (d *formDialog) trySubmit() (bool, tea.Cmd) {
	if !d.validate() {
		return false, d.syncFocus()
	}

	if d.submit == nil {
		return true, nil
	}

	return true, d.submit(d.values())
}

func (d *formDialog) key(_ *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	key := msg.String()

	switch key {
	case keyEsc:
		return true, nil
	case keyTab, "down":
		return false, d.move(1)
	case "shift+tab", "up":
		return false, d.move(-1)
	}

	// A dialog is answered with y or n from anywhere but a text field, where the
	// letters are text.
	if d.focus >= len(d.fields) || d.fields[d.focus].kind != fieldText {
		switch key {
		case "y":
			return d.trySubmit()
		case "n":
			return true, nil
		}
	}

	if d.focus < len(d.fields) {
		return d.fieldKey(d.fields[d.focus], msg)
	}

	switch key {
	case "left", "right", "h", "l":
		if d.focus == d.cancelIndex() {
			d.focus = d.actionIndex()
		} else {
			d.focus = d.cancelIndex()
		}
	case keyEnter, "space":
		if d.focus == d.cancelIndex() {
			return true, nil
		}

		return d.trySubmit()
	}

	return false, nil
}

func (d *formDialog) fieldKey(f *field, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	key := msg.String()

	if key == keyEnter {
		return d.trySubmit()
	}

	switch f.kind {
	case fieldToggle:
		if key == "space" || key == "x" {
			f.on = !f.on
		}
	case fieldChoice:
		switch key {
		case "space", "right", "l":
			f.choice = (f.choice + 1) % len(f.choices)
		case "left", "h":
			f.choice = (f.choice - 1 + len(f.choices)) % len(f.choices)
		}
	case fieldText:
		var cmd tea.Cmd

		f.input, cmd = f.input.Update(msg)
		f.problem = ""

		return false, cmd
	}

	return false, nil
}

func (d *formDialog) update(_ *Model, msg tea.Msg) tea.Cmd {
	if d.focus >= len(d.fields) || d.fields[d.focus].kind != fieldText {
		return nil
	}

	// What is left are the messages that make the cursor of the focused field blink.
	var cmd tea.Cmd

	f := d.fields[d.focus]
	f.input, cmd = f.input.Update(msg)

	return cmd
}

func (d *formDialog) paste(text string) {
	if d.focus < len(d.fields) && d.fields[d.focus].kind == fieldText {
		f := d.fields[d.focus]
		f.input, _ = f.input.Update(tea.PasteMsg{Content: firstLine(text)})
	}
}

// click handles a mouse click at a position relative to the top left of the dialog: it
// focuses the field that was clicked, flips a toggle, or presses a button.
func (d *formDialog) click(_ *Model, x, y int) (bool, tea.Cmd) {
	// The content starts after the border and the padding of the frame.
	cx, cy := x-clabinternaltuiui.FrameOverhead/2, y-1 //nolint:mnd // half on each side

	if cy == d.buttonLine {
		switch {
		case cx >= d.cancelZone[0] && cx < d.cancelZone[1]:
			return true, nil
		case cx >= d.actionZone[0] && cx < d.actionZone[1]:
			d.focus = d.actionIndex()

			return d.trySubmit()
		}

		return false, nil
	}

	for i, line := range d.fieldLines {
		if line != cy {
			continue
		}

		d.focus = i
		f := d.fields[i]

		switch f.kind {
		case fieldToggle:
			f.on = !f.on
		case fieldChoice:
			f.choice = (f.choice + 1) % len(f.choices)
		case fieldText:
		}

		return false, d.syncFocus()
	}

	return false, nil
}

// scroll does nothing: a form has nothing to scroll.
func (*formDialog) scroll(int) {}

// modal reports that a click next to the dialog does not close it: what was entered
// would be lost by accident.
func (*formDialog) modal() bool { return true }

func (d *formDialog) hints() []hint {
	hints := []hint{{keyTab, "next"}}

	for _, f := range d.fields {
		if f.kind != fieldText {
			hints = append(hints, hint{"␣", "toggle"})

			break
		}
	}

	return append(hints, hint{"⏎", strings.ToLower(d.action)}, hint{keyEsc, "cancel"})
}

func (d *formDialog) view(m *Model, maxWidth, _ int) string {
	width := min(maxWidth, dialogMaxWidth) - clabinternaltuiui.FrameOverhead

	var lines []string

	for _, line := range d.body {
		lines = append(lines, m.th.Text.Render(clabinternaltuiui.Truncate(line, width)))
	}

	d.fieldLines = d.fieldLines[:0]

	if len(d.fields) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}

		for i := range d.fields {
			d.fieldLines = append(d.fieldLines, len(lines)+i)
		}

		lines = append(lines, d.renderFields(m, width)...)
	}

	if d.preview != nil {
		lines = append(lines, "",
			m.th.Muted.Render(clabinternaltuiui.Truncate("$ "+d.preview(d.values()), width)))
	}

	if d.problem != "" {
		lines = append(lines, "", m.th.Error.Render(
			clabinternaltuiui.Truncate(d.problem, width)))
	}

	d.buttonLine = len(lines) + 1
	lines = append(lines, "", d.buttons(m, lines, width))

	border, title := m.th.FocusBorder, m.th.Title
	if d.danger {
		border, title = m.th.Error, m.th.Error.Bold(true)
	}

	return clabinternaltuiui.Frame(d.title, strings.Join(lines, "\n"), maxWidth, border, title)
}

func (d *formDialog) renderFields(m *Model, width int) []string {
	labelWidth := 0

	for _, f := range d.fields {
		if f.kind != fieldToggle {
			labelWidth = max(labelWidth, clabinternaltuiui.Width(f.label))
		}
	}

	var lines []string

	for i, f := range d.fields {
		marker := "  "
		if i == d.focus {
			marker = m.th.Accent.Render("▸ ")
		}

		label := m.th.Text.Render(clabinternaltuiui.PadRight(f.label, labelWidth))

		switch f.kind {
		case fieldToggle:
			box := "[ ]"
			if f.on {
				box = "[" + m.th.Success.Render("x") + "]"
			}

			lines = append(lines, marker+box+" "+m.th.Text.Render(f.label))
		case fieldChoice:
			line := marker + label + "  " + m.th.Muted.Render("‹ ") +
				m.th.Accent.Render(f.choices[f.choice]) + m.th.Muted.Render(" ›")

			if f.choice < len(f.choiceNotes) {
				line += "  " + m.th.Muted.Render(f.choiceNotes[f.choice])
			}

			lines = append(lines, clabinternaltuiui.Truncate(line, width))
		case fieldText:
			value := clabinternaltuiui.PadRight(f.input.View(), inputWidth+1)
			line := marker + label + "  " + m.th.Border.Render(
				"[",
			) + value + m.th.Border.Render(
				"]",
			)

			if f.problem != "" {
				line += " " + m.th.Error.Render("✗ "+f.problem)
			} else if f.note != "" {
				line += " " + m.th.Muted.Render(f.note)
			}

			lines = append(lines, clabinternaltuiui.Truncate(line, width))
		}
	}

	return lines
}

// buttons renders the two buttons, set to the right edge of the dialog.
func (d *formDialog) buttons(m *Model, lines []string, width int) string {
	inner := dialogMinWidth
	for _, line := range lines {
		inner = max(inner, clabinternaltuiui.Width(line))
	}

	inner = min(inner, width)

	render := func(label string, index int) string {
		text := "  " + label + "  "

		switch {
		case index != d.focus:
			return m.th.Chip.UnsetPadding().Render(text)
		case d.danger && index == d.actionIndex():
			return m.th.ChipActive.UnsetPadding().Background(m.th.Palette.Error).Render(text)
		default:
			return m.th.ChipActive.UnsetPadding().Render(text)
		}
	}

	cancel, act := render(buttonCancel, d.cancelIndex()), render(d.action, d.actionIndex())
	row := cancel + "  " + act

	// Remember where the buttons are for the mouse.
	start := max(0, inner-clabinternaltuiui.Width(row))
	d.cancelZone = [2]int{start, start + clabinternaltuiui.Width(cancel)}
	d.actionZone = [2]int{inner - clabinternaltuiui.Width(act), inner}

	return clabinternaltuiui.PadLeft(row, inner)
}

// ask opens a dialog.
func (m *Model) ask(d *formDialog) tea.Cmd {
	m.open(d)

	return d.prepare(m)
}
