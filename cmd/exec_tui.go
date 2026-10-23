// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// ErrSelectionCancelled is returned when the user aborts the picker.
var ErrSelectionCancelled = fmt.Errorf("selection cancelled")

// --- Lipgloss theme ---

var (
	tuiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6AC1")).
			Padding(0, 1)

	tuiSubtitleStyle = lipgloss.NewStyle().
				Faint(true).
				Foreground(lipgloss.Color("243"))

	tuiDocStyle = lipgloss.NewStyle().Margin(1, 1, 1, 2)

	tuiCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Faint(true).
			Italic(true)

	// container row badge colors per-kind: subtle tint
	kindColors = map[string]lipgloss.Color{
		"srl":      lipgloss.Color("#00D492"),
		"ceos":     lipgloss.Color("#FF6B35"),
		"fdio_vpp": lipgloss.Color("#7C5CFF"),
		"linux":    lipgloss.Color("#00BFFF"),
	}
)

// containerItem implements list.Item for the container picker.
type containerItem struct {
	ctr          clabruntime.GenericContainer
	displayName  string
	labName      string
	kind         string
	imageShort   string
	shellPreview string
	state        string
	status       string
}

func (c containerItem) Title() string { return c.displayName }
func (c containerItem) Description() string {
	kindBadge := c.kind
	if col, ok := kindColors[c.kind]; ok {
		kindBadge = lipgloss.NewStyle().Foreground(col).Bold(true).Render(c.kind)
	}
	// Truncate image if too long
	img := c.imageShort
	if len(img) > 40 {
		img = img[:37] + "..."
	}
	stateDot := "●"
	stateColor := lipgloss.Color("82") // green
	if c.state != "running" {
		stateColor = lipgloss.Color("214")
		if c.state == "exited" || c.state == "created" {
			stateColor = lipgloss.Color("196")
		}
	}
	dot := lipgloss.NewStyle().Foreground(stateColor).Render(stateDot)
	return fmt.Sprintf("%s %s  %s  %s  %s",
		dot,
		kindBadge,
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render(c.labName),
		lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(img),
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Render("▸ "+c.shellPreview),
	)
}
func (c containerItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s %s", c.displayName, c.labName, c.kind, c.imageShort, c.shellPreview)
}

// shellItem implements list.Item for the shell picker.
type shellItem struct {
	cmd         string
	description string
	isDefault   bool
}

func (s shellItem) Title() string {
	if s.isDefault {
		return s.cmd + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Render("★ default")
	}
	return s.cmd
}
func (s shellItem) Description() string { return s.description }
func (s shellItem) FilterValue() string { return s.cmd + " " + s.description }

// shortImage returns a human-friendly shortened image name (strip registry prefix digest etc.)
func shortImage(img string) string {
	// strip @sha256:...
	if idx := strings.Index(img, "@"); idx != -1 {
		img = img[:idx]
	}
	// keep last two path components if possible for readability
	parts := strings.Split(img, "/")
	if len(parts) <= 2 {
		return img
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

// styledDelegate returns a DefaultDelegate with containerlab-branded lipgloss styles.
func styledDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#5B2EFF")).
		Padding(0, 1)

	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8E0FF")).
		Background(lipgloss.Color("#5B2EFF")).
		Padding(0, 1)

	d.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	d.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Padding(0, 1)

	d.Styles.DimmedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Padding(0, 1)

	d.Styles.DimmedDesc = d.Styles.DimmedTitle.Foreground(lipgloss.Color("238"))

	// Make the cursor less intrusive – we use background highlight instead.
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

	return d
}

// buildListStyles creates list.Styles matching the TUI chrome.
func buildListStyles() list.Styles {
	styles := list.DefaultStyles()
	styles.TitleBar = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6AC1")).
		Background(lipgloss.Color("#1A1A2E")).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	styles.Title = tuiTitleStyle
	styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Faint(true).
		Padding(0, 1)

	styles.HelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Faint(true)

	styles.FilterPrompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	styles.FilterCursor = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212"))

	return styles
}

// selectWithList is a generic list picker helper. It returns the chosen index or ErrSelectionCancelled.
func selectWithList[T list.Item](listTitle string, items []T, placeholder string, subtitle string) (int, error) {
	// Non-interactive guard is expected to be done by caller; still check.
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stderr.Fd()) {
		return -1, fmt.Errorf("not a terminal")
	}

	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	delegate := styledDelegate()
	w, h := 100, 22
	if tw, th, err := term.GetSize(os.Stderr.Fd()); err == nil {
		w = tw - 4
		if w < 80 {
			w = 80
		}
		if w > 120 {
			w = 120
		}
		h = th - 6
		if h < 12 {
			h = 12
		}
		if h > 28 {
			h = 28
		}
	}

	m := list.New(listItems, delegate, w, h)
	m.Title = listTitle
	m.SetShowTitle(true)
	m.SetShowStatusBar(true)
	m.SetFilteringEnabled(true)
	m.SetShowPagination(true)
	m.SetShowHelp(false) // we render custom help in View wrapper
	m.Styles = buildListStyles()
	m.FilterInput.Placeholder = placeholder
	m.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	m.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	m.FilterInput.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	m.StatusMessageLifetime = 0

	// Wrap the list in a small model that adds header/footer chrome and handles quit keys uniformly.
	wrapper := tuiWrapper{
		list:     m,
		subtitle: subtitle,
		width:    w,
		height:   h,
	}

	p := tea.NewProgram(wrapper, tea.WithOutput(os.Stderr), tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return -1, err
	}

	wm, ok := res.(tuiWrapper)
	if !ok {
		return -1, fmt.Errorf("unexpected model type")
	}
	if wm.cancelled {
		return -1, ErrSelectionCancelled
	}
	if wm.chosen < 0 {
		return -1, ErrSelectionCancelled
	}
	return wm.chosen, nil
}

// tuiWrapper wraps a list.Model with containerlab chrome: rounded border, subtitle, footer help.
type tuiWrapper struct {
	list      list.Model
	subtitle  string
	width     int
	height    int
	chosen    int
	cancelled bool
}

func (w tuiWrapper) Init() tea.Cmd { return nil }

func (w tuiWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width - 4
		w.height = msg.Height - 6
		if w.width < 60 {
			w.width = 60
		}
		if w.height < 10 {
			w.height = 10
		}
		w.list.SetSize(w.width, w.height)
		return w, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			w.cancelled = true
			return w, tea.Quit
		case "enter":
			idx := w.list.Index()
			if idx >= 0 && idx < len(w.list.Items()) {
				w.chosen = idx
				return w, tea.Quit
			}
		case "q":
			// Only quit if not filtering – when filtering, 'q' is typed text.
			if w.list.FilterState() != list.Filtering {
				w.cancelled = true
				return w, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	w.list, cmd = w.list.Update(msg)
	return w, cmd
}

func (w tuiWrapper) View() string {
	header := ""
	if w.subtitle != "" {
		header = tuiSubtitleStyle.Render(w.subtitle)
	}

	// Footer help: context-sensitive
	help := ""
	if w.list.FilterState() == list.Filtering {
		help = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("212")).Render("▸ filtering  •  enter confirm  •  esc clear")
	} else if w.list.IsFiltered() {
		help = lipgloss.NewStyle().Faint(true).Render("↑↓ move  •  enter select  •  / filter  •  esc cancel  •  filtered: ") +
			tuiCountStyle.Render(fmt.Sprintf("%d/%d", len(w.list.VisibleItems()), len(w.list.Items())))
	} else {
		help = lipgloss.NewStyle().Faint(true).Render("↑↓/j k move  •  enter select  •  type to filter  •  esc quit") +
			"  " + tuiCountStyle.Render(fmt.Sprintf("%d containers", len(w.list.Items())))
	}

	// Compose with subtle border around list area
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(w.width + 2).
		Render(w.list.View())

	doc := ""
	if header != "" {
		doc = header + "\n"
	}
	doc += border + "\n" + help

	return tuiDocStyle.Render(doc)
}

// runContainerSelector presents the TUI container picker and returns the selected index.
func runContainerSelector(containers []clabruntime.GenericContainer, shells map[string][]string, labName string) (int, error) {
	items := make([]containerItem, len(containers))
	for i, ct := range containers {
		name := strings.TrimPrefix(ct.Names[0], "/")
		lab := ct.Labels["containerlab"]
		if lab == "" {
			lab = ct.Labels["clab-lab"]
		}
		if lab == "" {
			lab = "-"
		}
		kind := ct.Labels["clab-node-kind"]
		if kind == "" {
			kind = ct.Labels["kind"]
		}
		if kind == "" {
			kind = "unknown"
		}
		sh := ""
		if s, ok := shells[name]; ok {
			sh = strings.Join(s, " ")
		} else {
			sh = "/bin/sh"
		}
		items[i] = containerItem{
			ctr:          ct,
			displayName:  name,
			labName:      lab,
			kind:         kind,
			imageShort:   shortImage(ct.Image),
			shellPreview: sh,
			state:        ct.State,
			status:       ct.Status,
		}
	}

	title := "◉  Containerlab — attach shell"
	subtitle := ""
	if labName != "" {
		subtitle = fmt.Sprintf("Lab: %s  •  %d containers match  •  choose one to attach",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(labName),
			len(containers))
	} else {
		subtitle = fmt.Sprintf("%d containers available  •  type to filter, arrows to navigate",
			len(containers))
	}

	idx, err := selectWithList(title, items, "Filter by name, lab, kind, image…", subtitle)
	if err != nil {
		return -1, err
	}
	return idx, nil
}

// knownShellOptions provides curated shell candidates per-kind to surface in the shell picker.
func knownShellOptions(defaultShell []string, kind string) []shellItem {
	defaultStr := strings.Join(defaultShell, " ")
	seen := map[string]bool{}
	var out []shellItem

	add := func(cmd, desc string, isDefault bool) {
		if seen[cmd] {
			return
		}
		seen[cmd] = true
		out = append(out, shellItem{cmd: cmd, description: desc, isDefault: isDefault})
	}

	// Always first entry: resolved default
	add(defaultStr, fmt.Sprintf("Auto-detected for kind %q (env CLAB_EXEC_INTERACTIVE_SHELL > kind default)", kind), true)

	// Kind-specific curated extras
	switch kind {
	case "srl":
		add("/bin/bash", "Bash inside SRL container (host netns)", false)
		add("/bin/sr_cli -e -c 'conf; show system'", "sr_cli quick command example (customize after)", false)
	case "ceos":
		add("/usr/bin/Cli -p 15", "Arista EOS CLI (privileged)", false)
		add("/bin/bash", "Bash inside cEOS", false)
	case "fdio_vpp":
		add("/usr/bin/nsenter --net=/run/netns/dataplane /bin/bash", "Dataplane netns shell", false)
		add("/bin/bash", "Host netns bash", false)
	default:
		add("/bin/bash", "Bash (if available in image)", false)
		add("/bin/bash -l", "Bash login shell", false)
		add("/bin/sh", "POSIX sh (most compatible fallback)", false)
		add("/bin/ash", "Alpine ash", false)
	}

	// Generic fallbacks always worth surfacing
	add("/bin/sh", "POSIX sh fallback", false)
	add("/bin/bash", "Bash", false)
	add("/bin/bash -l", "Bash login", false)

	return out
}

// runShellSelector presents the shell picker for a given container.
func runShellSelector(containerName, kind string, defaultShell []string) ([]string, error) {
	options := knownShellOptions(defaultShell, kind)

	title := fmt.Sprintf("◉  Shell for %s", containerName)
	subtitle := fmt.Sprintf("Kind: %s  •  default: %s  •  pick an alternative or press enter to keep default",
		lipgloss.NewStyle().Bold(true).Foreground(kindColor(kind)).Render(kind),
		lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(strings.Join(defaultShell, " ")),
	)

	idx, err := selectWithList(title, options, "Filter shells…", subtitle)
	if err != nil {
		return nil, err
	}
	chosen := options[idx].cmd
	// Respect empty unlikely but handle.
	if chosen == "" {
		return defaultShell, nil
	}
	return strings.Fields(chosen), nil
}

func kindColor(kind string) lipgloss.Color {
	if c, ok := kindColors[kind]; ok {
		return c
	}
	return lipgloss.Color("212")
}
