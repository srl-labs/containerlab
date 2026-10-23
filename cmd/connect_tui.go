// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/sahilm/fuzzy"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------

var (
	connectAccent = lipgloss.Color("212") // hot pink

	// State dot colors: green = running, purple = selected line.
	connectRunningDotColor  = lipgloss.Color("48")
	connectSelectedDotColor = lipgloss.Color("141")

	connectTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(connectAccent).
				Padding(0, 2)

	connectPromptStyle = lipgloss.NewStyle().
				Foreground(connectAccent).
				Bold(true)

	connectQueryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	connectHelpStyle = lipgloss.NewStyle().
				Faint(true).
				Foreground(lipgloss.Color("241"))

	connectDimStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("243"))

	connectSelectedNameStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("255"))

	connectSelectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("57"))

	connectDetailBorderStyle = lipgloss.NewStyle().
					BorderLeft(true).
					BorderStyle(lipgloss.ThickBorder()).
					BorderForeground(connectAccent).
					PaddingLeft(2)

	connectKindColors = map[string]lipgloss.Color{
		"nokia_srlinux":  lipgloss.Color("48"),  // green
		"arista_ceos":    lipgloss.Color("214"), // orange
		"fdio_vpp":       lipgloss.Color("141"), // purple
		"linux":          lipgloss.Color("81"),  // cyan
		"vrnetlab":       lipgloss.Color("203"), // salmon
		"nokia_srsim":    lipgloss.Color("81"),
		"sonic_vm":       lipgloss.Color("81"),
		"cisco_xrd":      lipgloss.Color("81"),
		"juniper_vjunos": lipgloss.Color("81"),
		"nokia_sros":     lipgloss.Color("81"),
	}

	kindBadgeBaseStyle = lipgloss.NewStyle().Bold(true)
)

// ---------------------------------------------------------------------------
// Shared fuzzy helpers
// ---------------------------------------------------------------------------

// fuzzyMatch returns the indexes of entries in data that fuzzy-match query,
// sorted by match score (best first).
func fuzzyMatch(query string, data []string) []int {
	if strings.TrimSpace(query) == "" {
		idx := make([]int, len(data))
		for i := range data {
			idx[i] = i
		}
		return idx
	}

	matches := fuzzy.Find(query, data)
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Index)
	}
	return out
}

// highlightMatched renders s with the characters at idx highlighted in the
// accent color and bold, leaving the rest in baseStyle.
func highlightMatched(s string, idx []int, matchStyle lipgloss.Style) string {
	if len(idx) == 0 {
		return s
	}

	marked := make(map[int]struct{}, len(idx))
	for _, i := range idx {
		if i < len(s) {
			marked[i] = struct{}{}
		}
	}

	var b strings.Builder
	var run []rune
	flush := func(style lipgloss.Style) {
		if len(run) == 0 {
			return
		}
		b.WriteString(style.Render(string(run)))
		run = run[:0]
	}

	matchSet := false
	for i, r := range []rune(s) {
		_, isMatch := marked[i]
		if isMatch && !matchSet {
			flush(lipgloss.NewStyle()) // base is plain
			matchSet = true
		} else if !isMatch && matchSet {
			flush(matchStyle)
			matchSet = false
		}
		run = append(run, r)
	}
	if matchSet {
		flush(matchStyle)
	} else {
		flush(lipgloss.NewStyle())
	}

	return b.String()
}

// matchIndicesFor returns the fuzzy match indices of query inside s, or nil.
func matchIndicesFor(query, s string) []int {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	matches := fuzzy.Find(query, []string{s})
	if len(matches) == 0 {
		return nil
	}
	return matches[0].MatchedIndexes
}

// ---------------------------------------------------------------------------
// Node picker
// ---------------------------------------------------------------------------

type connectNodeEntry struct {
	container clabruntime.GenericContainer
	name      string // long container name (clab-<lab>-<node>)
	shortName string
	kind      string
	lab       string
	image     string
	state     string
	ipv4      string
	ipv6      string
	ports     string
	shell     string // default shell preview
}

// shortImage returns a human-friendly shortened image name (strip digest,
// keep the last two path components for readability).
func shortImage(img string) string {
	if idx := strings.Index(img, "@"); idx != -1 {
		img = img[:idx]
	}
	parts := strings.Split(img, "/")
	if len(parts) <= 2 {
		return img
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func buildConnectNodeEntries(containers []clabruntime.GenericContainer) []connectNodeEntry {
	entries := make([]connectNodeEntry, 0, len(containers))
	for _, ct := range containers {
		name := strings.TrimPrefix(ct.Names[0], "/")
		shortName := ct.Labels[clabconstants.NodeName]
		kind := ct.Labels[clabconstants.NodeKind]
		if kind == "" {
			kind = "linux"
		}
		lab := ct.Labels[clabconstants.Containerlab]
		if lab == "" {
			lab = "-"
		}
		ipv4 := ct.NetworkSettings.IPv4addr
		if ipv4 != "" {
			ipv4 = fmt.Sprintf("%s/%d", ipv4, ct.NetworkSettings.IPv4pLen)
		}
		ipv6 := ct.NetworkSettings.IPv6addr
		if ipv6 != "" {
			ipv6 = fmt.Sprintf("%s/%d", ipv6, ct.NetworkSettings.IPv6pLen)
		}
		ports := make([]string, 0, len(ct.Ports))
		for _, p := range ct.Ports {
			ports = append(ports, fmt.Sprintf("%d", p.HostPort))
		}

		entries = append(entries, connectNodeEntry{
			container: ct,
			name:      name,
			shortName: shortName,
			kind:      kind,
			lab:       lab,
			image:     shortImage(ct.Image),
			state:     ct.State,
			ipv4:      ipv4,
			ipv6:      ipv6,
			ports:     strings.Join(ports, ","),
			shell:     "",
		})
	}
	return entries
}

type nodePickerModel struct {
	input    textinput.Model
	entries  []connectNodeEntry
	names    []string
	filtered []int
	cursor   int
	offset   int
	chosen   int
	cancel   bool
	width    int
	height   int
	visible  int
}

func newNodePicker(entries []connectNodeEntry) nodePickerModel {
	input := textinput.New()
	input.Placeholder = "type to fuzzy search nodes…"
	input.Prompt = "❯ "
	input.PromptStyle = connectPromptStyle
	input.TextStyle = connectQueryStyle
	input.CharLimit = 64

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}

	m := nodePickerModel{
		input:    input,
		entries:  entries,
		names:    names,
		filtered: fuzzyMatch("", names),
		chosen:   -1,
		visible:  10,
	}
	m.input.Focus()
	return m
}

func (m nodePickerModel) Init() tea.Cmd { return textinput.Blink }

func (m nodePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.width > 0 {
			m.input.Width = m.width - 8
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.chosen = m.filtered[m.cursor]
				return m, tea.Quit
			}
		case "up", "ctrl+p", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			m.clampOffset()
			return m, nil
		case "down", "ctrl+n", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			m.clampOffset()
			return m, nil
		case "home", "ctrl+a":
			m.cursor = 0
			m.clampOffset()
			return m, nil
		case "end", "ctrl+e":
			m.cursor = max(0, len(m.filtered)-1)
			m.clampOffset()
			return m, nil
		case "pgup", "ctrl+b":
			m.cursor = max(0, m.cursor-m.visible)
			m.clampOffset()
			return m, nil
		case "pgdown", "ctrl+f":
			m.cursor = min(len(m.filtered)-1, m.cursor+m.visible)
			m.clampOffset()
			return m, nil
		}

		// default: feed to the input and refilter
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refilter()
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()
	return m, cmd
}

func (m *nodePickerModel) refilter() {
	m.filtered = fuzzyMatch(m.input.Value(), m.names)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.clampOffset()
}

func (m *nodePickerModel) clampOffset() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.visible {
		m.offset = m.cursor - m.visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m nodePickerModel) currentEntry() *connectNodeEntry {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.entries[m.filtered[m.cursor]]
}

func (m nodePickerModel) View() string {
	var b strings.Builder

	b.WriteString(connectTitleStyle.Render(" containerlab connect "))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	listWidth := 46
	if m.width > 120 {
		listWidth = 60
	}

	// Render the visible slice of the node list.
	rows := make([]string, 0, m.visible)
	for i := m.offset; i < min(len(m.filtered), m.offset+m.visible); i++ {
		idx := m.filtered[i]
		entry := m.entries[idx]
		selected := i == m.cursor

		matched := matchIndicesFor(m.input.Value(), entry.name)

		name := highlightMatched(entry.name, matched, lipgloss.NewStyle().Bold(true).Foreground(connectAccent))
		if selected {
			name = connectSelectedNameStyle.Render(name)
		}

		kindBadge := kindBadgeBaseStyle.
			Foreground(connectKindColor(entry.kind)).
			Render(fmt.Sprintf("%s", entry.kind))

		// Smaller state dot (• instead of ●) so lines stay compact.
		dot := "•"
		dotStyle := lipgloss.NewStyle().Foreground(connectRunningDotColor)
		if entry.state != "running" {
			dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			if entry.state == "exited" || entry.state == "created" {
				dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			}
		}
		if selected {
			dotStyle = lipgloss.NewStyle().Foreground(connectSelectedDotColor)
		}

		img := entry.image
		if len(img) > 30 {
			img = img[:27] + "…"
		}

		// Fixed two-space indent keeps lines perfectly aligned when the
		// cursor moves — selection is conveyed by the dot color and row
		// background instead of a chevron.
		line := fmt.Sprintf("  %s %s  %-18s %s",
			dotStyle.Render(dot),
			name,
			kindBadge,
			connectDimStyle.Render(img),
		)

		if selected {
			line = connectSelectedRowStyle.Render(line)
		}
		rows = append(rows, line)
	}

	if len(rows) == 0 {
		rows = append(rows, connectDimStyle.Render("  no nodes match — keep typing…"))
	}

	// Details panel for the highlighted node.
	detail := ""
	if cur := m.currentEntry(); cur != nil {
		detail = renderNodeDetail(cur)
	}

	left := strings.Join(rows, "\n")
	if len(m.filtered) > m.visible {
		left += "\n" + connectDimStyle.Render(fmt.Sprintf("  … %d/%d shown", m.visible, len(m.filtered)))
	}

	// Layout: list on the left, details on the right.
	listBlock := lipgloss.NewStyle().Width(listWidth).Render(left)
	detailBlock := lipgloss.NewStyle().Width(0).Render(detail)

	body := lipgloss.JoinHorizontal(lipgloss.Top, listBlock, "  ", detailBlock)
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(connectHelpStyle.Render("type to fuzzy search • ↑/↓ move • enter select • esc quit"))

	return lipgloss.NewStyle().PaddingLeft(1).Render(b.String())
}

func renderNodeDetail(e *connectNodeEntry) string {
	var b strings.Builder

	label := connectDimStyle.Render
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(connectAccent).Render("node details"))
	b.WriteString("\n\n")

	writeDetail := func(k, v string) {
		if v == "" {
			v = "—"
		}
		fmt.Fprintf(&b, "%s %s\n", label(fmt.Sprintf("%-10s", k)), value.Render(v))
	}

	writeDetail("name", e.name)
	writeDetail("kind", e.kind)
	writeDetail("lab", e.lab)
	writeDetail("image", e.image)
	writeDetail("state", e.state)
	writeDetail("ipv4", e.ipv4)
	writeDetail("ipv6", e.ipv6)
	if e.ports != "" {
		writeDetail("ports", e.ports)
	}

	return connectDetailBorderStyle.Render(b.String())
}

func connectKindColor(kind string) lipgloss.Color {
	if c, ok := connectKindColors[kind]; ok {
		return c
	}
	return lipgloss.Color("81")
}

// ---------------------------------------------------------------------------
// Method picker
// ---------------------------------------------------------------------------

type connectMethod struct {
	Label       string
	Description string
	Shell       []string
	SSH         bool
}

type methodPickerModel struct {
	input    textinput.Model
	node     *connectNodeEntry
	methods  []connectMethod
	labels   []string
	filtered []int
	cursor   int
	chosen   int
	cancel   bool
	width    int
}

func newMethodPicker(node *connectNodeEntry, methods []connectMethod) methodPickerModel {
	input := textinput.New()
	input.Placeholder = "filter methods…"
	input.Prompt = "❯ "
	input.PromptStyle = connectPromptStyle
	input.TextStyle = connectQueryStyle
	input.CharLimit = 64

	labels := make([]string, len(methods))
	for i, m := range methods {
		labels[i] = m.Label
	}

	m := methodPickerModel{
		input:    input,
		node:     node,
		methods:  methods,
		labels:   labels,
		filtered: fuzzyMatch("", labels),
		chosen:   -1,
	}
	m.input.Focus()
	return m
}

func (m methodPickerModel) Init() tea.Cmd { return textinput.Blink }

func (m methodPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.width > 0 {
			m.input.Width = m.width - 8
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.chosen = m.filtered[m.cursor]
				return m, tea.Quit
			}
		case "up", "ctrl+p", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "home":
			m.cursor = 0
			return m, nil
		case "end":
			m.cursor = max(0, len(m.filtered)-1)
			return m, nil
		}

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refilter()
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()
	return m, cmd
}

func (m *methodPickerModel) refilter() {
	m.filtered = fuzzyMatch(m.input.Value(), m.labels)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m methodPickerModel) View() string {
	var b strings.Builder

	b.WriteString(connectTitleStyle.Render(" how to connect? "))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	for i, idx := range m.filtered {
		method := m.methods[idx]
		selected := i == m.cursor

		label := method.Label
		if selected {
			label = connectSelectedNameStyle.Render(label)
		}
		desc := connectDimStyle.Render(method.Description)

		// Fixed two-space indent keeps method lines aligned while navigating —
		// selection is conveyed by the row background only.
		line := fmt.Sprintf("  %-16s %s", label, desc)
		if selected {
			line = connectSelectedRowStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(connectDimStyle.Render("  no methods match\n"))
	}

	b.WriteString("\n")
	if m.node != nil {
		b.WriteString(connectDimStyle.Render(fmt.Sprintf("target: %s (%s)", m.node.name, m.node.kind)))
		b.WriteString("\n")
	}
	b.WriteString(connectHelpStyle.Render("type to filter • ↑/↓ move • enter select • esc quit"))

	return lipgloss.NewStyle().PaddingLeft(1).Render(b.String())
}

// ---------------------------------------------------------------------------
// Public entrypoints used by connect.go
// ---------------------------------------------------------------------------

// runNodePicker shows the node picker TUI and returns the index of the chosen
// container in the original slice.
func runNodePicker(containers []clabruntime.GenericContainer, _ *clabcore.CLab) (int, error) {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stderr.Fd()) {
		// Fall back to a plain, readable listing when there's no TTY.
		var b strings.Builder
		b.WriteString("multiple nodes matched, please narrow the selection:\n")
		for _, ct := range containers {
			name := strings.TrimPrefix(ct.Names[0], "/")
			fmt.Fprintf(&b, "  %s (%s) [%s]\n", name, shortImage(ct.Image), ct.State)
		}
		b.WriteString("\nhint: run with a TTY to get an interactive fuzzy-search picker\n")
		fmt.Fprint(os.Stderr, b.String())

		return -1, fmt.Errorf("interactive connect requires a terminal")
	}

	entries := buildConnectNodeEntries(containers)
	p := tea.NewProgram(
		newNodePicker(entries),
		tea.WithOutput(os.Stderr),
	)
	res, err := p.Run()
	if err != nil {
		return -1, err
	}

	m, ok := res.(nodePickerModel)
	if !ok {
		return -1, fmt.Errorf("unexpected picker result")
	}
	if m.cancel || m.chosen < 0 {
		return -1, fmt.Errorf("selection cancelled")
	}
	return m.chosen, nil
}

// runMethodPicker shows the method picker TUI for a single node and returns
// the selected connection method.
func runMethodPicker(ct clabruntime.GenericContainer, c *clabcore.CLab) (connectMethod, error) {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stderr.Fd()) {
		return connectMethod{}, fmt.Errorf("interactive connect requires a terminal")
	}

	// Build the node entry once so the method picker can show target details.
	containers := []clabruntime.GenericContainer{ct}
	entries := buildConnectNodeEntries(containers)
	if len(entries) == 0 {
		return connectMethod{}, fmt.Errorf("failed to build node entry")
	}
	node := &entries[0]
	node.shell = strings.Join(resolveConnectShell(c, ct), " ")

	defaultShell := resolveConnectShell(c, ct)

	methods := buildConnectMethods(defaultShell, node.kind)

	p := tea.NewProgram(
		newMethodPicker(node, methods),
		tea.WithOutput(os.Stderr),
	)
	res, err := p.Run()
	if err != nil {
		return connectMethod{}, err
	}

	m, ok := res.(methodPickerModel)
	if !ok {
		return connectMethod{}, fmt.Errorf("unexpected picker result")
	}
	if m.cancel || m.chosen < 0 {
		return connectMethod{}, fmt.Errorf("selection cancelled")
	}
	return m.methods[m.chosen], nil
}

// buildConnectMethods assembles the connection options offered for a node:
// its default shell, common fallback shells, and SSH via the lab's ssh config.
func buildConnectMethods(defaultShell []string, kind string) []connectMethod {
	defaultStr := strings.Join(defaultShell, " ")

	seen := map[string]bool{}
	var out []connectMethod

	add := func(label, desc string, shell []string, ssh bool) {
		key := label
		if ssh {
			key = "ssh"
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, connectMethod{
			Label:       label,
			Description: desc,
			Shell:       shell,
			SSH:         ssh,
		})
	}

	// 1) The kind's default shell.
	add(defaultStr,
		fmt.Sprintf("default shell for kind %q", kind),
		defaultShell, false)

	// 2) SSH via the per-lab ssh config (written by clab deploy).
	add("ssh",
		"ssh via per-lab config (/etc/ssh/ssh_config.d/clab-*.conf)",
		nil, true)

	// 3) Common shells as fallbacks.
	add("/bin/bash", "bash", []string{"/bin/bash"}, false)
	add("/bin/bash -l", "bash login shell", []string{"/bin/bash", "-l"}, false)
	add("/bin/sh", "POSIX sh", []string{"/bin/sh"}, false)
	add("/bin/ash", "alpine ash", []string{"/bin/ash"}, false)

	// Sort methods: default first, ssh second, then the rest alphabetically
	// for a stable, predictable list.
	sort.SliceStable(out, func(i, j int) bool {
		ri := methodRank(out[i])
		rj := methodRank(out[j])
		if ri != rj {
			return ri < rj
		}
		return out[i].Label < out[j].Label
	})

	return out
}

func methodRank(m connectMethod) int {
	switch {
	case m.SSH:
		return 1
	case m.Label == "/bin/bash":
		return 2
	case m.Label == "/bin/bash -l":
		return 3
	case m.Label == "/bin/sh":
		return 4
	case m.Label == "/bin/ash":
		return 5
	default:
		return 0 // the node default comes first
	}
}
