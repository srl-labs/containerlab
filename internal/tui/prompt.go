// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

type promptMode int

const (
	// promptFilter filters the table of the current page as the user types.
	promptFilter promptMode = iota
	// promptCommand runs a command when the user presses enter.
	promptCommand
	// promptFind searches the text of the current page as the user types.
	promptFind
)

// finder is implemented by pages whose text can be searched.
type finder interface {
	find(query string)
}

const promptCharLimit = 256

// prompt is the input line shown in the status line.
type prompt struct {
	mode  promptMode
	input textinput.Model
}

func (m *Model) openPrompt(mode promptMode) tea.Cmd {
	input := textinput.New()
	input.CharLimit = promptCharLimit
	input.SetWidth(max(1, m.width/2)) //nolint:mnd // half the status line

	styles := textinput.DefaultStyles(m.th.Dark)
	styles.Focused.Prompt = m.th.Key
	styles.Focused.Text = m.th.Text
	styles.Focused.Placeholder = m.th.Muted
	input.SetStyles(styles)

	switch mode {
	case promptFilter:
		input.Prompt = "/"
		input.Placeholder = "filter"

		if t := m.top().table(); t != nil {
			input.SetValue(t.Filter())
		}
	case promptCommand:
		input.Prompt = ":"
		input.Placeholder = "command"
	case promptFind:
		input.Prompt = "/"
		input.Placeholder = "find"
	}

	m.prompt = &prompt{mode: mode, input: input}

	return m.prompt.input.Focus()
}

func (p *prompt) view() string { return p.input.View() }

func (p *prompt) hints() []hint {
	switch p.mode {
	case promptFilter:
		return []hint{
			{"⏎", "keep"}, {keyEsc, "clear"}, {"↑↓", "move"}, {"!term", "exclude"},
		}
	case promptFind:
		return []hint{{"⏎", "keep"}, {keyEsc, "clear"}}
	default:
		return []hint{{"⏎", "run"}, {keyTab, "complete"}, {keyEsc, "cancel"}}
	}
}

// update handles the messages that make the cursor of the input blink.
func (p *prompt) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	p.input, cmd = p.input.Update(msg)

	return cmd
}

func (p *prompt) key(m *Model, msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case keyEsc:
		if p.mode != promptCommand {
			p.apply(m, "")
		}

		m.prompt = nil

		return nil
	case keyEnter:
		m.prompt = nil

		if p.mode == promptCommand {
			return m.runCommand(p.input.Value())
		}

		return nil
	case keyTab:
		if p.mode == promptCommand {
			p.complete(m)
		}

		return nil
	case "up", "down", "pgup", "pgdown":
		// The list stays navigable while the filter is being typed.
		if t := m.top().table(); t != nil && p.mode == promptFilter {
			t.HandleKey(msg)

			return nil
		}
	}

	var cmd tea.Cmd

	p.input, cmd = p.input.Update(msg)

	if p.mode != promptCommand {
		p.apply(m, p.input.Value())
	}

	return cmd
}

func (p *prompt) paste(m *Model, text string) tea.Cmd {
	var cmd tea.Cmd

	p.input, cmd = p.input.Update(tea.PasteMsg{Content: firstLine(text)})

	if p.mode != promptCommand {
		p.apply(m, p.input.Value())
	}

	return cmd
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")

	return line
}

// apply narrows the table of the page down to the query, or searches its text for it.
func (p *prompt) apply(m *Model, query string) {
	if p.mode == promptFind {
		if f, ok := m.top().(finder); ok {
			f.find(query)
		}

		return
	}

	if t := m.top().table(); t != nil {
		t.SetFilter(query)
	}
}

// complete extends the typed command to the longest unambiguous match.
func (p *prompt) complete(m *Model) {
	value := p.input.Value()

	var candidates []string

	word, arg, hasArg := strings.Cut(value, " ")

	switch {
	case !hasArg:
		for _, c := range append(commandNames(), m.names()...) {
			if strings.HasPrefix(c, word) {
				candidates = append(candidates, c)
			}
		}
	case word == "lab":
		for i := range m.labs {
			if strings.HasPrefix(m.labs[i].Name, arg) {
				candidates = append(candidates, "lab "+m.labs[i].Name)
			}
		}
	case word == "theme":
		for _, name := range clabinternaltuitheme.Names() {
			if strings.HasPrefix(name, arg) {
				candidates = append(candidates, "theme "+name)
			}
		}
	}

	if len(candidates) == 0 {
		return
	}

	completed := commonPrefix(candidates)
	if len(candidates) == 1 && !hasArg && takesArgument(completed) {
		completed += " "
	}

	if len(completed) > len(value) {
		p.input.SetValue(completed)
		p.input.CursorEnd()
	}
}

// names returns the names of the labs and nodes, which the command line takes as
// commands to go there.
func (m *Model) names() []string {
	var names []string

	for i := range m.labs {
		names = append(names, m.labs[i].Name)

		for n := range m.labs[i].Nodes {
			names = append(names, m.labs[i].Nodes[n].Name)
		}
	}

	return names
}

func commonPrefix(values []string) string {
	prefix := values[0]

	for _, v := range values[1:] {
		for !strings.HasPrefix(v, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}

	return prefix
}

func takesArgument(command string) bool {
	return command == "lab" || command == "theme"
}

func commandNames() []string {
	names := make([]string, 0, len(tabs)+7) //nolint:mnd // the commands besides the views
	names = append(names, "all", "lab", "theme", "help", "quit", "refresh", "messages")

	for _, t := range tabs {
		names = append(names, t.command)
	}

	slices.Sort(names)

	return names
}

// runCommand executes a command typed at the command line.
func (m *Model) runCommand(line string) tea.Cmd {
	word, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
	arg = strings.TrimSpace(arg)

	if word == "" {
		return nil
	}

	for _, t := range tabs {
		if word == t.command || strings.EqualFold(word, t.short) {
			m.goTo(t.id)

			return nil
		}
	}

	switch word {
	case "q", "quit", "exit":
		return tea.Quit
	case "all":
		m.setScope("")
	case "lab":
		m.commandLab(arg)
	case "theme":
		m.commandTheme(arg)
	case "help", "h":
		m.push(newHelpPage(m))
	case "refresh":
		m.be.Refresh()
	case "messages":
		m.push(newMessagesPage(m))
	case "topo", "topology":
		m.goTo(tabMap)
	default:
		// Anything else is taken for the name of a lab or a node to go to.
		if !m.goToName(word) {
			m.unknownCommand(word)
		}
	}

	return nil
}

func (m *Model) commandLab(name string) {
	if name == "" {
		m.setScope("")
		m.goTo(tabLabs)

		return
	}

	lab := m.lab(name)
	if lab == nil {
		m.notifyf(clabinternaltuitheme.LevelWarning, "no lab named %q", name)

		return
	}

	m.scope = lab.Name
	m.goTo(tabNodes)
}

func (m *Model) commandTheme(name string) {
	if name == "" {
		m.notifyf(clabinternaltuitheme.LevelInfo, "theme: %s — available: %s",
			m.th.ID, strings.Join(clabinternaltuitheme.Names(), ", "))

		return
	}

	th, err := clabinternaltuitheme.New(name, m.dark)
	if err != nil {
		m.notifyf(clabinternaltuitheme.LevelWarning, "%v", err)

		return
	}

	m.opts.Theme = name
	m.setTheme(th)
}
