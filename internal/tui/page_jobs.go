// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// jobStateWidth bounds the state of a job in the job list, error included.
const jobStateWidth = 40

// jobLook returns the glyph and the severity that stand for the state of a job.
func jobLook(s clabinternaltuibackend.JobState) (string, clabinternaltuitheme.Level) {
	switch s {
	case clabinternaltuibackend.JobRunning, clabinternaltuibackend.JobCancelling:
		return glyphPartial, clabinternaltuitheme.LevelInfo
	case clabinternaltuibackend.JobSucceeded:
		return glyphReady, clabinternaltuitheme.LevelSuccess
	case clabinternaltuibackend.JobFailed:
		return glyphDown, clabinternaltuitheme.LevelError
	default:
		return glyphNone, clabinternaltuitheme.LevelMuted
	}
}

func jobLabel(id clabinternaltuibackend.JobID) string {
	return "#" + strconv.Itoa(int(id))
}

// jobsPage lists the operations started from the UI, the most recent first.
type jobsPage struct {
	basePage

	tbl *clabinternaltuiui.Table
}

//nolint:mnd // the widths of the columns are what they are
func jobColumns() []clabinternaltuiui.Column {
	return []clabinternaltuiui.Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "id", Title: "ID", Numeric: true},
		{ID: "kind", Title: "KIND"},
		{ID: "lab", Title: "LAB", Max: 24, Priority: 2},
		{ID: "state", Title: "STATE"},
		{
			ID:       "time",
			Title:    "TIME",
			Priority: 1,
			Numeric:  true,
			Align:    clabinternaltuiui.AlignRight,
		},
		{ID: "started", Title: "STARTED", Priority: 3},
		{ID: "what", Title: "WHAT", Min: 12, Flex: true, Priority: 1},
	}
}

func newJobsPage(m *Model) *jobsPage {
	p := &jobsPage{tbl: clabinternaltuiui.NewTable(m.th, jobColumns())}
	p.tbl.Empty = "no jobs yet — deploy, destroy and the other operations show up here"
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*jobsPage) tab() tabID                        { return tabJobs }
func (p *jobsPage) table() *clabinternaltuiui.Table { return p.tbl }
func (p *jobsPage) crumbs(*Model) []string {
	return []string{"jobs " + tableCount(p.tbl)}
}
func (p *jobsPage) info(*Model) []string { return tableInfo(p.tbl) }

func (p *jobsPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.tbl.SetSize(width, height)
}

func (p *jobsPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicJobs | clabinternaltuibackend.TopicJobOutput) {
		return
	}

	p.rebuild(m)
}

func (p *jobsPage) rebuild(m *Model) {
	rows := make([]clabinternaltuiui.Row, 0, len(m.jobs))

	for i := range m.jobs {
		job := &m.jobs[i]
		glyph, level := jobLook(job.State)

		state := job.State.String()
		if job.State == clabinternaltuibackend.JobFailed && job.Err != "" {
			state += ": " + job.Err
		}

		state = clabinternaltuiui.Truncate(clabinternaltuiui.Sanitize(state), jobStateWidth)
		took := job.Duration(m.now())
		what := job.Title

		if job.Live {
			what += " · " + packetCount(job.Packets)
		}

		if job.Kind == clabinternaltuibackend.JobCapture && job.Bytes > 0 {
			what += " · " + clabinternaltuiui.Bytes(job.Bytes)
		}

		rows = append(rows, clabinternaltuiui.Row{
			Key: strconv.Itoa(int(job.ID)),
			Cells: []clabinternaltuiui.Cell{
				m.levelCell(glyph, level),
				{Text: jobLabel(job.ID), Style: m.th.Muted, Num: float64(job.ID)},
				m.cell(string(job.Kind)),
				m.mutedCell(job.Lab),
				m.levelCell(state, level),
				{Text: clabinternaltuiui.Duration(took), Style: m.th.Text, Num: took.Seconds()},
				m.mutedCell(job.Started.Format("15:04:05")),
				m.cell(what),
			},
		})
	}

	p.tbl.SetRows(rows)
}

func (p *jobsPage) current() (clabinternaltuibackend.JobID, bool) {
	id, err := strconv.Atoi(p.tbl.CursorKey())
	if err != nil {
		return 0, false
	}

	return clabinternaltuibackend.JobID(id), true
}

func (p *jobsPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.tbl.HandleKey(msg) {
		return true, nil
	}

	id, ok := p.current()
	if !ok {
		return false, nil
	}

	switch msg.String() {
	case keyEnter:
		m.push(m.jobView(id))

		return true, nil
	case "C":
		return true, m.cancelJob(id)
	}

	return false, nil
}

// jobView returns the page that shows a job: its packets for a live capture, its output
// for every other job.
func (m *Model) jobView(id clabinternaltuibackend.JobID) page {
	if job, ok := m.be.Job(id); ok && job.Live {
		return newCapturePage(m, id)
	}

	return newJobPage(m, id)
}

func (p *jobsPage) view(m *Model) string {
	// The durations of the running jobs move on with the clock, not with the backend.
	p.rebuild(m)

	return p.tbl.View()
}

func (*jobsPage) hints(*Model) []hint {
	return []hint{{"⏎", "output"}, {"C", "cancel"}, {"/", "filter"}, {"?", "help"}}
}

// cancelJob stops a running job after asking, since a cancelled operation can leave a lab
// half way.
func (m *Model) cancelJob(id clabinternaltuibackend.JobID) tea.Cmd {
	job, ok := m.be.Job(id)
	if !ok || job.State.Done() {
		return nil
	}

	if job.Kind == clabinternaltuibackend.JobCapture {
		m.be.CancelJob(id)

		return nil
	}

	note := "The operation is interrupted, which can leave the lab partially changed."
	if job.Kind == clabinternaltuibackend.JobDeploy {
		note = "A fresh deployment is rolled back: what was created is removed again."
	}

	return m.ask(newForm("Cancel "+job.Title, "Cancel job", func(formValues) tea.Cmd {
		m.be.CancelJob(id)

		return nil
	}).dangerous().text(note))
}

// jobPage shows the output of a job as it runs.
type jobPage struct {
	basePage

	id   clabinternaltuibackend.JobID
	log  *clabinternaltuiui.LogView
	read int
}

func newJobPage(m *Model, id clabinternaltuibackend.JobID) *jobPage {
	p := &jobPage{id: id, log: clabinternaltuiui.NewLogView(m.th)}
	p.log.Timestamps = true
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*jobPage) tab() tabID { return tabJobs }

func (p *jobPage) job(m *Model) (clabinternaltuibackend.JobInfo, bool) {
	return m.be.Job(p.id)
}

func (p *jobPage) crumbs(m *Model) []string {
	title := jobLabel(p.id)
	if job, ok := p.job(m); ok {
		title += " " + job.Title
	}

	return []string{"jobs", title}
}

func (p *jobPage) info(m *Model) []string {
	job, ok := p.job(m)
	if !ok {
		return nil
	}

	state := job.State.String() + " " + clabinternaltuiui.Duration(job.Duration(m.now()))
	if job.Kind == clabinternaltuibackend.JobCapture && job.Bytes > 0 {
		state += " · " + clabinternaltuiui.Bytes(job.Bytes)
	}

	return append([]string{state}, p.log.Status()...)
}

func (p *jobPage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicJobOutput | clabinternaltuibackend.TopicJobs) {
		return
	}

	lines := m.be.JobOutput(p.id, p.read)
	p.read += len(lines)
	p.log.Append(toLogLines(lines)...)
}

func toLogLines(lines []clabinternaltuibackend.LogLine) []clabinternaltuiui.LogLine {
	out := make([]clabinternaltuiui.LogLine, len(lines))
	for i := range lines {
		out[i] = clabinternaltuiui.LogLine{
			Time:   lines[i].Time,
			Level:  lines[i].Level,
			Stderr: lines[i].Stderr,
			Text:   lines[i].Text,
		}
	}

	return out
}

func (p *jobPage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.log.SetSize(width, height-2) //nolint:mnd // the command and the result line
}

func (p *jobPage) scroll(notches int) { p.log.Scroll(notches * 3) } //nolint:mnd // lines per notch

func (p *jobPage) find(query string) { p.log.SetQuery(query) }

func (p *jobPage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.log.HandleKey(msg) {
		return true, nil
	}

	switch msg.String() {
	case "C":
		return true, m.cancelJob(p.id)
	case "y":
		var text strings.Builder

		_, _ = p.log.WriteTo(&text)

		return true, m.copy("job output", text.String())
	case keyEnter:
		// Once a lab operation is through, enter leads on to the nodes of the lab.
		if job, ok := p.job(
			m,
		); ok && job.State == clabinternaltuibackend.JobSucceeded &&
			job.Lab != "" {
			if lab := m.lab(job.Lab); lab != nil && lab.Deployed {
				m.scope = job.Lab
				m.goTo(tabNodes)

				return true, nil
			}
		}
	}

	return false, nil
}

func (p *jobPage) view(m *Model) string {
	job, ok := p.job(m)
	if !ok {
		return " " + m.th.Muted.Render("this job is gone")
	}

	command := " " + m.th.Muted.Render("$ "+clabinternaltuiui.Sanitize(job.Command))

	return clabinternaltuiui.Truncate(
		command,
		p.width,
	) + "\n" + p.log.View() + "\n" + p.result(
		m,
		&job,
	)
}

// result sums the job up once it is through.
func (p *jobPage) result(m *Model, job *clabinternaltuibackend.JobInfo) string {
	_, level := jobLook(job.State)
	style := m.th.LevelStyle(level)

	var text string

	switch job.State {
	case clabinternaltuibackend.JobRunning:
		text = "running…"
	case clabinternaltuibackend.JobCancelling:
		text = "cancelling…"
	case clabinternaltuibackend.JobSucceeded:
		text = "✓ done in " + clabinternaltuiui.Duration(job.Duration(m.now()))
		if job.Warnings > 0 {
			text += fmt.Sprintf(" with %d warning(s)", job.Warnings)
		}
	case clabinternaltuibackend.JobFailed:
		text = "✗ failed"
		if job.Err != "" {
			text += ": " + clabinternaltuiui.Sanitize(job.Err)
		}
	case clabinternaltuibackend.JobCancelled:
		text = "cancelled — the lab may be left partially changed"
	}

	return clabinternaltuiui.Truncate(" "+style.Render(text), p.width)
}

func (p *jobPage) hints(m *Model) []hint {
	hints := []hint{{"f", "follow"}, {"w", "wrap"}, {"/", "find"}, {"y", "copy"}}

	if job, ok := p.job(m); ok {
		if !job.State.Done() {
			hints = append([]hint{{"C", "cancel"}}, hints...)
		} else if job.State == clabinternaltuibackend.JobSucceeded && job.Lab != "" {
			hints = append([]hint{{"⏎", "nodes"}}, hints...)
		}
	}

	return append(hints, hint{keyEsc, "back"}, hint{"?", "help"})
}
