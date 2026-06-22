package selector

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")) // blue/aqua
	noteStyle   = lipgloss.NewStyle().Faint(true)
	helpStyle   = lipgloss.NewStyle().Faint(true)
)

// ErrCancelled is returned by FromList when the user aborts the picker.
var ErrCancelled = errors.New("selection cancelled")

// Item is a single row in the picker: a primary label plus an optional grey
// subtext (e.g. "(running)" or an image name).
type Item struct {
	Label string
	Note  string
}

// model is a minimal Bubble Tea single-choice list. chosen is -1 until an item
// is picked.
type model struct {
	title     string
	items     []Item
	cursor    int
	chosen    int
	cancelled bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m model) View() string {
	// Leave no residue once a choice has been made or the picker was cancelled.
	if m.chosen >= 0 || m.cancelled {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, it := range m.items {
		note := ""
		if it.Note != "" {
			note = " " + noteStyle.Render(it.Note)
		}

		if i == m.cursor {
			fmt.Fprintf(&b, "%s %s%s\n", cursorStyle.Render("▸"), cursorStyle.Render(it.Label), note)

			continue
		}

		fmt.Fprintf(&b, "  %s%s\n", it.Label, note)
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move • enter select • esc cancel"))

	return b.String()
}

// FromList renders an interactive single-choice list and returns the index of the
// chosen item, or ErrCancelled if the user aborts. It renders to stderr so it does
// not pollute stdout. Callers must ensure they are attached to an interactive
// terminal (e.g. term.IsTerminal(os.Stdin.Fd())) before calling.
func FromList(title string, items []Item) (int, error) {
	res, err := tea.NewProgram(
		model{title: title, items: items, chosen: -1},
		tea.WithOutput(os.Stderr),
	).Run()
	if err != nil {
		return -1, err
	}

	m, ok := res.(model)
	if !ok || m.cancelled || m.chosen < 0 {
		return -1, ErrCancelled
	}

	return m.chosen, nil
}
