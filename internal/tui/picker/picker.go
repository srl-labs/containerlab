// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package picker provides the small inline selection lists used by commands that need the
// user to choose something, such as the node and the method of `containerlab connect`.
// Unlike the full-screen TUI they take a few lines below the prompt and leave nothing
// behind once a choice is made.
package picker

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// ErrCancelled is returned when the user leaves a picker without choosing anything.
var ErrCancelled = errors.New("selection cancelled")

const (
	// maxRows is the number of entries a picker shows at most.
	maxRows = 10
	// chromeRows are the lines a picker needs besides its entries: the input, the help
	// and the blank lines between them.
	chromeRows = 6
	// detailOverlap is the number of those lines the details panel may use as well.
	detailOverlap = 2
	// defaultWidth is used until the terminal reports its size.
	defaultWidth = 120

	rowPrefix     = 4 // indent, state dot and a space
	columnGap     = 2
	minImageWidth = 8
	portsWidth    = 24
	labelWidth    = 10
	queryLimit    = 64

	stateRunning = "running"
	stateExited  = "exited"
	stateCreated = "created"
)

// Node is what a picker shows about a lab node.
type Node struct {
	// Name is the name of the node's container.
	Name  string
	Kind  string
	Lab   string
	Image string
	State string
	IPv4  string
	IPv6  string
	Ports string
}

// model is a list narrowed down by typing. It backs both pickers, which differ in how they
// draw an entry and in what they show next to and below the list.
type model struct {
	th    *clabinternaltuitheme.Theme
	title string
	input textinput.Model

	// labels are the texts the query is matched against, one per entry.
	labels  []string
	matches []clabinternaltuiui.FuzzyMatch
	cursor  int
	offset  int

	width  int
	height int

	chosen    int
	cancelled bool

	nodes   []Node
	methods []clabinternaltuibackend.ConnectMethod
	// target is the node a method is picked for.
	target *Node
	// detailWidth and detailHeight are the size of the largest details panel. Reserving
	// the width keeps the list from shifting while the cursor moves between nodes.
	detailWidth  int
	detailHeight int
	help         string
}

func newModel(title, placeholder, help string, labels []string) *model {
	th := clabinternaltuitheme.Default(true)

	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = "❯ "
	input.CharLimit = queryLimit

	m := &model{
		th:     th,
		title:  title,
		input:  input,
		labels: labels,
		chosen: -1,
		help:   help,
	}
	m.applyTheme(th)
	m.refilter()
	// The query takes the keys from the start, before the program asks for it.
	m.input.Focus()

	return m
}

func (m *model) applyTheme(th *clabinternaltuitheme.Theme) {
	m.th = th

	styles := textinput.DefaultStyles(th.Dark)
	styles.Focused.Prompt = th.Title
	styles.Focused.Text = th.Text
	styles.Focused.Placeholder = th.Muted
	m.input.SetStyles(styles)

	m.detailWidth, m.detailHeight = 0, 0

	for i := range m.nodes {
		w, h := lipgloss.Size(m.nodeDetail(&m.nodes[i]))
		m.detailWidth, m.detailHeight = max(m.detailWidth, w), max(m.detailHeight, h)
	}
}

// contentWidth is the room the picker has, less its left padding.
func (m *model) contentWidth() int {
	if m.width <= 0 {
		return defaultWidth - 1
	}

	return m.width - 1
}

func newNodeModel(nodes []Node) *model {
	labels := make([]string, len(nodes))
	for i := range nodes {
		labels[i] = nodes[i].Name
	}

	m := newModel("containerlab connect", "type to fuzzy search nodes…",
		"type to fuzzy search • ↑/↓ move • enter select • esc quit", labels)
	m.nodes = nodes
	m.applyTheme(m.th)

	return m
}

func newMethodModel(target *Node, methods []clabinternaltuibackend.ConnectMethod) *model {
	labels := make([]string, len(methods))
	for i := range methods {
		labels[i] = methods[i].Label
	}

	m := newModel("containerlab connect — pick a method", "filter methods…",
		"type to filter • ↑/↓ move • enter select • esc quit", labels)
	m.methods = methods
	m.target = target

	return m
}

// Init implements tea.Model.
func (m *model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.RequestBackgroundColor)
}

func (m *model) refilter() {
	m.matches = clabinternaltuiui.FuzzyFind(m.input.Value(), m.labels)
	m.cursor = min(m.cursor, max(0, len(m.matches)-1))
	m.clampOffset()
}

// visible returns the number of entries shown: as many as fit the terminal, up to a cap.
func (m *model) visible() int {
	rows := maxRows
	if m.height > 0 {
		rows = min(rows, m.height-chromeRows)
	}

	return max(1, rows)
}

func (m *model) clampOffset() {
	visible := m.visible()

	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}

	m.offset = max(0, m.offset)
}

func (m *model) move(delta int) {
	m.cursor = max(0, min(m.cursor+delta, len(m.matches)-1))
	m.clampOffset()
}

// Update implements tea.Model.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(1, m.width-8)) //nolint:mnd // the prompt and some margin
		m.clampOffset()

		return m, nil
	case tea.BackgroundColorMsg:
		m.applyTheme(clabinternaltuitheme.Default(msg.IsDark()))

		return m, nil
	case tea.KeyPressMsg:
		return m.key(msg)
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m *model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true

		return m, tea.Quit
	case "enter":
		if m.cursor < len(m.matches) {
			m.chosen = m.matches[m.cursor].Index

			return m, tea.Quit
		}

		return m, nil
	case "up", "ctrl+p":
		m.move(-1)

		return m, nil
	case "down", "ctrl+n":
		m.move(1)

		return m, nil
	case "pgup", "ctrl+b":
		m.move(-m.visible())

		return m, nil
	case "pgdown", "ctrl+f":
		m.move(m.visible())

		return m, nil
	case "home":
		m.move(-len(m.matches))

		return m, nil
	case "end":
		m.move(len(m.matches))

		return m, nil
	}

	// Everything else is typed into the query, letters that move the cursor in other
	// lists included. A changed query starts over from the best match.
	before := m.input.Value()

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	if m.input.Value() != before {
		m.cursor, m.offset = 0, 0
		m.refilter()
	}

	return m, cmd
}

// View implements tea.Model.
func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.WindowTitle = m.title

	return v
}

// render draws the picker. Once a choice was made it draws nothing, which removes the
// picker from the screen.
func (m *model) render() string {
	if m.cancelled || m.chosen >= 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("\n" + m.input.View() + "\n\n")

	if m.nodes != nil {
		b.WriteString(m.nodeList())
	} else {
		b.WriteString(m.methodList())
	}

	b.WriteString("\n\n")

	width := m.contentWidth()

	if m.target != nil {
		target := fmt.Sprintf("target: %s (%s)", m.target.Name, m.target.Kind)
		b.WriteString(m.th.Muted.Render(clabinternaltuiui.Truncate(target, width)) + "\n")
	}

	b.WriteString(m.th.Muted.Render(clabinternaltuiui.Truncate(m.help, width)))

	return lipgloss.NewStyle().PaddingLeft(1).Render(b.String())
}

// nodeLayout holds the column widths of the node list. An image width of zero hides the
// image column.
type nodeLayout struct {
	name, kind, image int
	detail            bool
}

func (l nodeLayout) width() int {
	w := rowPrefix + l.name + columnGap + l.kind
	if l.image > 0 {
		w += columnGap + l.image
	}

	return w
}

// layout fits the node list into the terminal width so that rows never wrap. The details
// panel is hidden when node names and kinds would not fit next to it, and the image
// column gets whatever space remains.
func (m *model) layout() nodeLayout {
	var l nodeLayout

	imageWidth := 0

	for i := range m.nodes {
		l.name = max(l.name, clabinternaltuiui.Width(m.nodes[i].Name))
		l.kind = max(l.kind, clabinternaltuiui.Width(m.nodes[i].Kind))
		imageWidth = max(imageWidth, clabinternaltuiui.Width(m.nodes[i].Image))
	}

	termWidth := m.contentWidth()

	required := rowPrefix + l.name + columnGap + l.kind
	available := termWidth - columnGap - m.detailWidth

	// The panel needs room next to the list, and a terminal high enough to hold it: it
	// spans the entries and the blank lines around them.
	l.detail = required <= available &&
		(m.height <= 0 || m.height >= m.detailHeight+chromeRows-detailOverlap)
	if !l.detail {
		available = termWidth
	}

	l.name = max(1, l.name-max(0, required-available))

	if room := available - required - columnGap; room >= minImageWidth {
		l.image = min(imageWidth, room)
	}

	return l
}

func (m *model) nodeList() string {
	layout := m.layout()
	visible := m.visible()
	rows := make([]string, 0, visible)

	for i := m.offset; i < min(len(m.matches), m.offset+visible); i++ {
		rows = append(rows, m.nodeRow(i, layout))
	}

	if len(rows) == 0 {
		rows = append(rows, m.th.Muted.Render("  no nodes match — keep typing…"))
	}

	if len(m.matches) > visible {
		rows = append(rows, m.th.Muted.Render(
			fmt.Sprintf("  … %d/%d shown", visible, len(m.matches))))
	}

	list := lipgloss.NewStyle().Width(layout.width()).Render(strings.Join(rows, "\n"))
	if !layout.detail || m.cursor >= len(m.matches) {
		return list
	}

	detail := m.nodeDetail(&m.nodes[m.matches[m.cursor].Index])

	return lipgloss.JoinHorizontal(lipgloss.Top, list, strings.Repeat(" ", columnGap), detail)
}

func (m *model) nodeRow(i int, layout nodeLayout) string {
	match := m.matches[i]
	node := &m.nodes[match.Index]
	selected := i == m.cursor

	base := m.th.Text
	if selected {
		base = m.th.Bold
	}

	// The whole name is matched, the cut one is shown: matches beyond the cut are
	// simply not highlighted.
	name := clabinternaltuiui.Truncate(node.Name, layout.name)
	line := "  " + m.dot(node.State, selected) + " " +
		clabinternaltuiui.HighlightRunes(name, match.Runes, base, m.th.Match) +
		strings.Repeat(" ", layout.name-clabinternaltuiui.Width(name)) +
		strings.Repeat(" ", columnGap) +
		lipgloss.NewStyle().Foreground(m.th.Hue(node.Kind)).Width(layout.kind).Render(node.Kind)

	if layout.image > 0 {
		line += strings.Repeat(" ", columnGap) +
			m.th.Muted.Render(clabinternaltuiui.Truncate(node.Image, layout.image))
	}

	return line
}

// dot draws the state of a node. The line under the cursor is told by the color of its
// dot and its bold name, so that rows never shift when the cursor moves.
func (m *model) dot(state string, selected bool) string {
	style := m.th.Success

	switch {
	case selected:
		style = m.th.Accent
	case state == stateExited || state == stateCreated:
		style = m.th.Error
	case state != stateRunning:
		style = m.th.Warning
	}

	return style.Render("•")
}

// nodeDetail renders the details panel of a node.
func (m *model) nodeDetail(n *Node) string {
	var b strings.Builder

	b.WriteString(m.th.Title.Render("node details") + "\n\n")

	write := func(label, value string) {
		if value == "" {
			value = clabinternaltuiui.Dash
		}

		b.WriteString(m.th.Muted.Render(clabinternaltuiui.PadRight(label, labelWidth)) + " " +
			m.th.Text.Render(value) + "\n")
	}

	write("name", n.Name)
	write("kind", n.Kind)
	write("lab", n.Lab)
	write("image", n.Image)
	write("state", n.State)
	write("ipv4", n.IPv4)
	write("ipv6", n.IPv6)

	if n.Ports != "" {
		// Long port lists wrap, so that the panel does not take the space of the list.
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			m.th.Muted.Render(clabinternaltuiui.PadRight("ports", labelWidth))+" ",
			m.th.Text.Width(portsWidth).Render(n.Ports)) + "\n")
	}

	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(m.th.Palette.Accent).
		PaddingLeft(2). //nolint:mnd // room between the border and the text
		Render(b.String())
}

func (m *model) methodList() string {
	width := 0
	for i := range m.methods {
		width = max(width, clabinternaltuiui.Width(m.methods[i].Label))
	}

	var rows []string

	for i, match := range m.matches {
		method := &m.methods[match.Index]

		base := m.th.Text
		if i == m.cursor {
			base = m.th.Bold
		}

		rows = append(rows, clabinternaltuiui.Truncate("  "+
			clabinternaltuiui.HighlightRunes(method.Label, match.Runes, base, m.th.Match)+
			strings.Repeat(" ", width-clabinternaltuiui.Width(method.Label))+
			strings.Repeat(" ", columnGap)+m.th.Muted.Render(method.Description), m.contentWidth()))
	}

	if len(rows) == 0 {
		rows = append(rows, m.th.Muted.Render("  no methods match"))
	}

	return strings.Join(rows, "\n")
}

// run shows a picker and returns the index of the chosen entry.
func run(m *model, in io.Reader, out io.Writer) (int, error) {
	result, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return -1, err
	}

	final, ok := result.(*model)
	if !ok {
		return -1, errors.New("unexpected picker result")
	}

	if final.cancelled || final.chosen < 0 {
		return -1, ErrCancelled
	}

	return final.chosen, nil
}

// PickNode lets the user choose one of the nodes and returns its index.
func PickNode(nodes []Node, in io.Reader, out io.Writer) (int, error) {
	return run(newNodeModel(nodes), in, out)
}

// PickMethod lets the user choose how to connect to a node and returns the index of the
// chosen method.
func PickMethod(
	target *Node, methods []clabinternaltuibackend.ConnectMethod, in io.Reader, out io.Writer,
) (int, error) {
	return run(newMethodModel(target, methods), in, out)
}
