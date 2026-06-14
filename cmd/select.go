package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectTitleStyle  = lipgloss.NewStyle().Bold(true)
	selectCursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")) // blue/aqua
	selectNoteStyle   = lipgloss.NewStyle().Faint(true)
	selectHelpStyle   = lipgloss.NewStyle().Faint(true)
)

// errSelectionCancelled is returned when the user aborts an interactive picker.
var errSelectionCancelled = errors.New("selection cancelled")

// selectItem is a single row in an interactive picker: a primary label plus an
// optional grey subtext (e.g. "(running)" or an image name).
type selectItem struct {
	label string
	note  string
}

// selectModel is a minimal Bubble Tea single-choice list. It returns the index
// of the chosen item via chosen (-1 until something is picked).
type selectModel struct {
	title     string
	items     []selectItem
	cursor    int
	chosen    int
	cancelled bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		m.chosen = m.cursor
		return m, tea.Quit
	}

	return m, nil
}

func (m selectModel) View() string {
	// Leave no residue once a choice has been made or the picker was cancelled.
	if m.chosen >= 0 || m.cancelled {
		return ""
	}

	var b strings.Builder

	b.WriteString(selectTitleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, it := range m.items {
		note := ""
		if it.note != "" {
			note = " " + selectNoteStyle.Render(it.note)
		}

		if i == m.cursor {
			fmt.Fprintf(&b, "%s %s%s\n", selectCursorStyle.Render("▸"), selectCursorStyle.Render(it.label), note)

			continue
		}

		fmt.Fprintf(&b, "  %s%s\n", it.label, note)
	}

	b.WriteString("\n")
	b.WriteString(selectHelpStyle.Render("↑/↓ move • enter select • esc cancel"))

	return b.String()
}

// selectFromList renders an interactive single-choice list and returns the index
// of the chosen item, or errSelectionCancelled if the user aborts. It renders to
// stderr so it does not pollute stdout. Callers must ensure they are attached to
// an interactive terminal (e.g. term.IsTerminal(os.Stdin.Fd())) before calling.
func selectFromList(title string, items []selectItem) (int, error) {
	res, err := tea.NewProgram(
		selectModel{title: title, items: items, chosen: -1},
		tea.WithOutput(os.Stderr),
	).Run()
	if err != nil {
		return -1, err
	}

	m, ok := res.(selectModel)
	if !ok || m.cancelled || m.chosen < 0 {
		return -1, errSelectionCancelled
	}

	return m.chosen, nil
}
