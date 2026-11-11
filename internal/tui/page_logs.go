// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// logTail is the number of lines a log view starts with.
	logTail = 500
	// logFileMode makes a saved log readable by its owner only.
	logFileMode = 0o600
)

// logBatchMsg carries a batch of lines of a log stream to the page that reads it.
type logBatchMsg struct {
	page *logsPage
	// gen is the generation of the stream the batch was read from.
	gen   int
	lines []clabinternaltuibackend.LogLine
	// done is set when the stream has ended.
	done bool
}

// logsPage shows the log of a node and follows it.
type logsPage struct {
	basePage

	ref    clabinternaltuibackend.NodeRef
	log    *clabinternaltuiui.LogView
	stream clabinternaltuibackend.LogStream
	cancel context.CancelFunc
	// gen counts the streams the page opened and closed. A stream that was replaced or
	// closed may still deliver a batch or its end, which is told apart by it.
	gen int
	// ended tells why the stream stopped, empty while it runs.
	ended string
}

// openLogs opens the log of a node.
func (m *Model) openLogs(ref clabinternaltuibackend.NodeRef) tea.Cmd {
	p := &logsPage{ref: ref, log: clabinternaltuiui.NewLogView(m.th)}
	m.push(p)

	return p.start(m)
}

// start opens the stream and returns the command that reads from it.
func (p *logsPage) start(m *Model) tea.Cmd {
	p.gen++

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	stream, err := m.be.Logs(ctx, p.ref, clabinternaltuibackend.LogOptions{
		Tail: logTail, Follow: true, Timestamps: true,
	})
	if err != nil {
		cancel()

		p.ended = err.Error()

		return nil
	}

	p.stream = stream
	p.ended = ""

	return p.next()
}

// next waits for the next batch of lines.
func (p *logsPage) next() tea.Cmd {
	stream, gen := p.stream, p.gen

	return func() tea.Msg {
		lines, ok := <-stream.Lines()

		return logBatchMsg{page: p, gen: gen, lines: lines, done: !ok}
	}
}

func (*logsPage) tab() tabID { return tabNodes }

func (p *logsPage) crumbs(*Model) []string {
	return []string{"labs", p.ref.Lab, p.ref.Name, "logs"}
}

func (p *logsPage) info(*Model) []string {
	status := p.log.Status()
	if p.ended != "" {
		status = append([]string{p.ended}, status...)
	}

	return status
}

func (*logsPage) refresh(*Model, clabinternaltuibackend.Topic) {}

func (p *logsPage) focus(*Model) clabinternaltuibackend.Focus {
	return clabinternaltuibackend.Focus{Lab: p.ref.Lab, Node: p.ref.Name}
}

func (p *logsPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.log.SetSize(width, height)
}

func (p *logsPage) scroll(notches int) { p.log.Scroll(notches * 3) } //nolint:mnd // lines per notch

func (p *logsPage) find(query string) { p.log.SetQuery(query) }

func (p *logsPage) update(_ *Model, msg tea.Msg) tea.Cmd {
	batch, ok := msg.(logBatchMsg)
	if !ok || batch.page != p || batch.gen != p.gen {
		return nil
	}

	p.log.Append(toLogLines(batch.lines)...)

	if !batch.done {
		return p.next()
	}

	p.ended = "stream ended"
	if err := p.stream.Err(); err != nil {
		p.ended = clabinternaltuiui.Sanitize(err.Error())
	}

	return nil
}

func (p *logsPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.log.HandleKey(msg) {
		return true, nil
	}

	switch msg.String() {
	case "s":
		p.save(m)

		return true, nil
	case "y":
		var text strings.Builder

		_, _ = p.log.WriteTo(&text)

		return true, m.copy("log", text.String())
	case "r":
		p.close()
		p.log.Reset()

		return true, p.start(m)
	}

	return false, nil
}

// save writes the lines held to a file in the current directory.
func (p *logsPage) save(m *Model) {
	name := fmt.Sprintf("%s-%s.log", p.ref.Container, m.now().Format("20060102-150405"))

	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, logFileMode)
	if err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "cannot save the log: %v", err)

		return
	}
	defer file.Close()

	if _, err := p.log.WriteTo(file); err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "cannot save the log: %v", err)

		return
	}

	m.notifyf(clabinternaltuitheme.LevelSuccess, "saved %d lines to %s", p.log.Len(), name)
}

// sibling shows the log of the node next to this one in the lab.
func (p *logsPage) sibling(m *Model, delta int) tea.Cmd {
	next, ok := m.neighborNode(p.ref, delta)
	if !ok {
		return nil
	}

	p.close()
	p.log.Reset()
	p.log.Follow = true
	p.ref = next
	m.be.SetFocus(p.focus(m))

	return p.start(m)
}

func (p *logsPage) view(*Model) string { return p.log.View() }

func (*logsPage) hints(*Model) []hint {
	return []hint{
		{"f", "follow"},
		{"w", "wrap"},
		{"T", "time"},
		{"/", "find"},
		{"n/N", "next/prev"},
		{"[ ]", "other node"},
		{"s", "save"},
		{"y", "copy"},
		{"r", "reload"},
		{keyEsc, "back"},
		{"?", "help"},
	}
}

func (p *logsPage) close() {
	p.gen++

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	if p.stream != nil {
		p.stream.Close()
	}
}
