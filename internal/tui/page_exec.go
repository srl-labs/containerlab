// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// execResultMsg carries the result of a command for one node to the page that runs it.
type execResultMsg struct {
	page   *execPage
	result clabinternaltuibackend.ExecResult
	// done is set when every node has answered.
	done bool
}

// execPage runs a command in a set of nodes and shows the output of each as it arrives.
type execPage struct {
	basePage

	lab     string
	command string
	nodes   []clabinternaltuibackend.NodeRef
	results map[string]*clabinternaltuibackend.ExecResult
	stream  <-chan clabinternaltuibackend.ExecResult
	cancel  context.CancelFunc
	running bool
	failure string

	lines  []string
	offset int
}

func newExecPage(
	m *Model, lab string, nodes []clabinternaltuibackend.NodeRef, command string,
) *execPage {
	m.lastCommand = command

	return &execPage{lab: lab, command: command, nodes: nodes}
}

// start runs the command and returns the command that waits for the first result.
func (p *execPage) start(m *Model) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	p.cancel = cancel
	p.results = map[string]*clabinternaltuibackend.ExecResult{}
	p.failure = ""

	stream, err := m.be.Exec(ctx, p.nodes, p.command)
	if err != nil {
		cancel()

		p.failure = err.Error()
		p.render(m)

		return nil
	}

	p.stream = stream
	p.running = true
	p.render(m)

	return p.next()
}

func (p *execPage) next() tea.Cmd {
	stream := p.stream

	return func() tea.Msg {
		result, ok := <-stream

		return execResultMsg{page: p, result: result, done: !ok}
	}
}

func (*execPage) tab() tabID { return tabNodes }

func (p *execPage) crumbs(*Model) []string {
	crumbs := []string{"run"}
	if p.lab != "" {
		crumbs = []string{"labs", p.lab, "run"}
	}

	return crumbs
}

func (p *execPage) info(*Model) []string {
	var ok, failed int

	for _, r := range p.results {
		if r.Err != nil || r.ExitCode != 0 {
			failed++
		} else {
			ok++
		}
	}

	info := []string{fmt.Sprintf("%d ok", ok)}
	if failed > 0 {
		info = append(info, fmt.Sprintf("%d failed", failed))
	}

	if pending := len(p.nodes) - len(p.results); pending > 0 && p.running {
		info = append(info, fmt.Sprintf("%d pending", pending))
	}

	return info
}

func (*execPage) refresh(*Model, clabinternaltuibackend.Topic) {}

func (p *execPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.clamp()
}

func (p *execPage) clamp() {
	p.offset = max(0, min(p.offset, len(p.lines)-p.height))
}

func (p *execPage) scroll(notches int) {
	p.offset += notches * 3 //nolint:mnd // lines per wheel notch
	p.clamp()
}

func (p *execPage) update(m *Model, msg tea.Msg) tea.Cmd {
	res, ok := msg.(execResultMsg)
	if !ok || res.page != p {
		return nil
	}

	if res.done {
		p.running = false
		p.render(m)

		return nil
	}

	result := res.result
	p.results[result.Node.Container] = &result
	p.render(m)

	return p.next()
}

// render rebuilds the lines: one section per node, in the order the nodes were given.
func (p *execPage) render(m *Model) {
	lines := []string{" " + m.th.Muted.Render("$ "+clabinternaltuiui.Sanitize(p.command)), ""}

	if p.failure != "" {
		lines = append(lines, " "+m.th.Error.Render(
			"✗ "+clabinternaltuiui.Sanitize(p.failure)))
	}

	for i := range p.nodes {
		lines = append(lines, p.section(m, &p.nodes[i])...)
	}

	p.lines = lines
	p.clamp()
}

func (p *execPage) section(m *Model, ref *clabinternaltuibackend.NodeRef) []string {
	r := p.results[ref.Container]
	name := m.th.Bold.Render(ref.Name)

	if r == nil {
		state := "waiting…"
		if !p.running {
			state = "no answer"
		}

		return []string{
			" " + m.th.Info.Render(glyphPartial) + " " + name + "  " + m.th.Muted.Render(state),
		}
	}

	glyph, level := glyphReady, clabinternaltuitheme.LevelSuccess
	status := fmt.Sprintf("exit %d", r.ExitCode)

	if r.Err != nil {
		status = clabinternaltuiui.Sanitize(r.Err.Error())
	}

	if r.Err != nil || r.ExitCode != 0 {
		glyph, level = glyphDown, clabinternaltuitheme.LevelError
	}

	lines := []string{
		" " + m.th.LevelStyle(level).Render(glyph) + " " + name + "  " +
			m.th.LevelStyle(level).Render(status) + m.th.Muted.Render(" · "+r.Duration.String()),
	}

	for _, line := range outputLines(r.Stdout) {
		lines = append(lines, "   "+m.th.Text.Render(line))
	}

	for _, line := range outputLines(r.Stderr) {
		lines = append(lines, "  "+m.th.Error.Render("▎")+m.th.Warning.Render(line))
	}

	return append(lines, "")
}

// outputLines splits command output into lines that are safe to show.
func outputLines(output string) []string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	for i := range lines {
		lines[i] = clabinternaltuiui.Sanitize(lines[i])
	}

	return lines
}

func (p *execPage) plain() string {
	var b strings.Builder

	for i := range p.nodes {
		r := p.results[p.nodes[i].Container]
		if r == nil {
			continue
		}

		fmt.Fprintf(&b, "# %s (exit %d)\n%s%s\n", p.nodes[i].Name, r.ExitCode, r.Stdout, r.Stderr)
	}

	return b.String()
}

func (p *execPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
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
	case "r":
		if p.running {
			return true, nil
		}

		p.close()

		return true, p.start(m)
	case "y":
		return true, m.copy("output", p.plain())
	default:
		return false, nil
	}

	p.clamp()

	return true, nil
}

func (p *execPage) view(*Model) string {
	end := min(len(p.lines), p.offset+p.height)

	return strings.Join(p.lines[p.offset:end], "\n")
}

func (*execPage) hints(*Model) []hint {
	return []hint{
		{"↑↓", "scroll"}, {"r", "run again"}, {"y", "copy"}, {keyEsc, "back"}, {"?", "help"},
	}
}

func (p *execPage) close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// monospaced tells that the body of the page is literal text.
func (*execPage) monospaced() bool { return true }
