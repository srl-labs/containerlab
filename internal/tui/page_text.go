// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// messageLevelWidth is the width of the severity column of the messages page.
const messageLevelWidth = 5

// textPage shows scrollable text, such as the help or the details of a node.
type textPage struct {
	basePage

	owner tabID
	path  []string
	// lines are rendered lines, styles included.
	lines  []string
	offset int
	// reload rebuilds the lines when the backend reports changes, nil for static text.
	reload func(m *Model) []string
	// plain is the text without styles, for copying.
	plain string

	// query is the text being searched for, hits the lines that contain it and hit the
	// one the view was last moved to.
	query string
	hits  []int
	hit   int
}

func (p *textPage) tab() tabID             { return p.owner }
func (p *textPage) crumbs(*Model) []string { return append([]string(nil), p.path...) }

func (p *textPage) info(*Model) []string {
	var info []string

	if p.query != "" {
		if len(p.hits) == 0 {
			info = append(info, "find: "+p.query+" (none)")
		} else {
			info = append(info, fmt.Sprintf("find: %s %d/%d", p.query, p.hit+1, len(p.hits)))
		}
	}

	if len(p.lines) > p.height {
		last := min(len(p.lines), p.offset+p.height)
		info = append(info, fmt.Sprintf("%d-%d of %d", p.offset+1, last, len(p.lines)))
	}

	return info
}

// find searches the text and shows the first line at or below the top of the view that
// contains the query.
func (p *textPage) find(query string) {
	p.query = strings.ToLower(strings.TrimSpace(query))
	p.hits, p.hit = p.hits[:0], 0

	if p.query == "" {
		return
	}

	for i, line := range p.lines {
		if strings.Contains(strings.ToLower(ansi.Strip(line)), p.query) {
			p.hits = append(p.hits, i)
		}
	}

	for i, line := range p.hits {
		if line >= p.offset {
			p.hit = i

			break
		}
	}

	p.showHit()
}

func (p *textPage) nextHit(delta int) {
	if len(p.hits) == 0 {
		return
	}

	p.hit = (p.hit + delta + len(p.hits)) % len(p.hits)
	p.showHit()
}

func (p *textPage) showHit() {
	if len(p.hits) == 0 {
		return
	}

	const context = 3 // lines kept visible above the hit

	p.offset = p.hits[p.hit] - context
	p.clamp()
}

func (p *textPage) refresh(m *Model, _ clabinternaltuibackend.Topic) {
	if p.reload != nil {
		p.lines = p.reload(m)
		p.clamp()
	}
}

func (p *textPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.clamp()
}

func (p *textPage) clamp() {
	p.offset = max(0, min(p.offset, len(p.lines)-p.height))
}

func (p *textPage) scroll(notches int) {
	p.offset += notches * 3 //nolint:mnd // lines per wheel notch
	p.clamp()
}

func (p *textPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	page := max(1, p.height-1)

	switch msg.String() {
	case "up", "k":
		p.offset--
	case "down", "j":
		p.offset++
	case "pgup", "ctrl+b":
		p.offset -= page
	case "pgdown", "ctrl+f", "space":
		p.offset += page
	case "home", "g":
		p.offset = 0
	case "end", "G":
		p.offset = len(p.lines)
	case "n":
		p.nextHit(1)

		return true, nil
	case "N":
		p.nextHit(-1)

		return true, nil
	case "y":
		if p.plain == "" {
			return false, nil
		}

		return true, m.copy("text", p.plain)
	default:
		return false, nil
	}

	p.clamp()

	return true, nil
}

func (p *textPage) view(m *Model) string {
	end := min(len(p.lines), p.offset+p.height)
	lines := append([]string(nil), p.lines[p.offset:end]...)

	// The lines the search found are marked in the margin.
	for _, hit := range p.hits {
		if hit >= p.offset && hit < end {
			lines[hit-p.offset] = m.th.Accent.Render("▎") +
				strings.TrimPrefix(lines[hit-p.offset], " ")
		}
	}

	return strings.Join(lines, "\n")
}

func (p *textPage) hints(*Model) []hint {
	hints := []hint{{"↑↓", "scroll"}, {"g/G", "top/end"}, {"/", "find"}}
	if p.plain != "" {
		hints = append(hints, hint{"y", "copy"})
	}

	return append(hints, hint{keyEsc, "back"}, hint{"?", "help"})
}

// helpGroup is a titled list of keys on the help page.
type helpGroup struct {
	title string
	keys  []hint
}

func helpGroups() []helpGroup {
	return []helpGroup{
		{"The basics", []hint{
			{"↑ ↓", "move to a lab, a node, an interface"},
			{"⏎", "open it"},
			{"esc", "go back"},
			{"a", "show what can be done with it, and do it"},
			{"/", "find it by typing part of its name"},
			{"q", "quit"},
		}},
		{"Getting around", []hint{
			{"⏎ or →", "go into the selected item: a lab, a node, a job"},
			{"esc or ←", "go back out: clear the filter, the marks, then leave"},
			{"[ ]", "previous and next: lab, or node when one is open"},
			{"ctrl+k", "go to a lab, a node or a view by typing part of its name"},
			{"1 … 6", "views: labs, nodes, interfaces, links, map, jobs"},
			{"0", "show all labs in the current view"},
			{":", "command line: a view, the name of a lab or node, :theme, :quit"},
			{"q", "go back, quit from the list of labs"},
			{"ctrl+c", "quit"},
		}},
		{"Lists", []hint{
			{"↑ ↓ j k", "move"},
			{"pgup pgdn", "move a page, ctrl+u and ctrl+d half a page"},
			{"g G", "first and last row"},
			{"/", "filter; several words must all match, !word excludes"},
			{"< >", "sort by the previous or next column, also shift+← →"},
			{"I", "invert the sort order"},
			{"space", "mark the row, actions then apply to all marked rows"},
			{"ctrl+a", "mark or unmark all rows"},
			{"p", "show or hide the details pane"},
			{"a", "menu of everything that can be done with the selection"},
			{"ctrl+r", "reload"},
			{"ctrl+z", "suspend, return with fg"},
		}},
		{"Labs", []hint{
			{"D", "deploy"},
			{"X", "destroy"},
			{"R", "redeploy"},
			{"W", "save the configuration of all nodes"},
			{"t", "topology map"},
			{"x", "run a command in all nodes"},
			{"H", "share a terminal (sshx, gotty)"},
		}},
		{"Nodes", []hint{
			{"s", "shell"},
			{"S", "ssh"},
			{"c", "choose how to connect, then 1 … 9"},
			{"l", "logs"},
			{"d", "describe"},
			{"x", "run a command"},
			{"U O R", "start, stop, restart"},
			{"W", "save the configuration"},
			{"y", "copy a value: n name, 4 IPv4, 6 IPv6, i image, s ssh command"},
		}},
		{"Menus and dialogs", []hint{
			{"key", "the key in front of a menu entry chooses it"},
			{"/", "search a menu"},
			{"y n", "confirm or cancel a dialog"},
			{"tab", "next field, space toggles an option"},
		}},
		{"Interfaces and links", []hint{
			{"n", "impair a link: delay, jitter, loss, rate limit, corruption"},
			{"N", "remove the impairments"},
			{"C", "capture packets: watch them live here or in Wireshark, or keep a file"},
		}},
		{"Live capture", []hint{
			{"↑ ↓", "look at earlier packets, which pauses the view"},
			{"f", "pause, or follow the newest packet again; also space and G"},
			{"⏎", "the headers and the bytes of the packet"},
			{"/", "filter, ie. bgp, an address or !lldp"},
			{"C", "stop the capture, the packets stay"},
			{"l", "what the capture command logged"},
		}},
		{"Map", []hint{
			{"← ↑ ↓ →", "move the focus from node to node"},
			{"tab", "step through the links of the node, o jumps to the far end"},
			{"/", "go to a node by name"},
			{"e", "how much is drawn: all, without hosts, or around the focus"},
			{"+ - f", "zoom in, zoom out, fit"},
			{"v", "layout: saved positions, tiers or by connectivity"},
			{"i", "interface names at the ends of links: of the node in focus, all, off"},
			{"r", "show or hide the traffic rates on the links"},
			{"H J K L", "pan"},
		}},
		{"Mouse", []hint{
			{"click", "select a row, a view, a column title to sort by, or a key hint"},
			{"double click", "open the row or the node"},
			{"right click", "menu of actions for what was clicked"},
			{"wheel", "scroll"},
			{"drag", "move the map"},
			{"shift+drag", "select text with the terminal, bypassing the UI"},
		}},
	}
}

func newHelpPage(m *Model) *textPage {
	const keyWidth = 12

	var lines []string

	for i, group := range helpGroups() {
		if i > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, " "+m.th.Title.Render(group.title))

		for _, k := range group.keys {
			lines = append(
				lines,
				"   "+m.th.Key.Render(clabinternaltuiui.PadRight(k.key, keyWidth))+
					" "+m.th.Text.Render(k.desc),
			)
		}
	}

	return &textPage{owner: m.top().tab(), path: []string{"help"}, lines: lines}
}

func messageLines(m *Model) []string {
	messages := m.be.Messages()
	lines := make([]string, 0, len(messages))

	for i := range messages {
		msg := &messages[i]
		level := clabinternaltuitheme.LevelMuted

		switch msg.Level {
		case "WARN":
			level = clabinternaltuitheme.LevelWarning
		case "ERRO", "ERROR", "FATA":
			level = clabinternaltuitheme.LevelError
		}

		lines = append(lines, " "+m.th.Muted.Render(msg.Time.Format("15:04:05"))+" "+
			m.th.LevelStyle(level).Render(clabinternaltuiui.PadRight(msg.Level, messageLevelWidth))+
			" "+m.th.Text.Render(clabinternaltuiui.Sanitize(msg.Text)))
	}

	if len(lines) == 0 {
		lines = []string{" " + m.th.Muted.Render("no messages")}
	}

	return lines
}

func newMessagesPage(m *Model) *textPage {
	return &textPage{
		owner:  m.top().tab(),
		path:   []string{"messages"},
		lines:  messageLines(m),
		reload: messageLines,
	}
}
