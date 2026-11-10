// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"strings"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	brand = "clab"

	crumbSeparator = " ▸ "
	ruleChar       = "─"

	glyphReady   = "●"
	glyphPartial = "◐"
	glyphDown    = "○"
	glyphNone    = "·"

	arrowRx = "↓"
	arrowTx = "↑"
)

// zone is a clickable stretch of the header.
type zone struct {
	from, to int
	tab      tabID
}

// header renders the first line: the tabs on the left and the context on the right.
func (m *Model) header() string {
	left, zones := m.headerTabs(false)
	right := m.headerContext(contextFull)

	fits := func() bool {
		return clabinternaltuiui.Width(left)+clabinternaltuiui.Width(right) < m.width
	}

	// On narrow terminals the header degrades step by step: shorter tab titles first,
	// then less and less context.
	if !fits() {
		left, zones = m.headerTabs(true)
	}

	for detail := contextShort; !fits() && detail >= contextMinimal; detail-- {
		right = m.headerContext(detail)
	}

	m.tabZones = zones

	gap := m.width - clabinternaltuiui.Width(left) - clabinternaltuiui.Width(right)
	if gap < 1 {
		return clabinternaltuiui.PadRight(left, m.width)
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) headerTabs(short bool) (string, []zone) {
	var b strings.Builder

	b.WriteString(" " + m.th.Title.Render(brand) + " ")

	pos := clabinternaltuiui.Width(brand) + 2 //nolint:mnd // the spaces around the brand
	active := m.top().tab()
	zones := make([]zone, 0, len(tabs))

	for _, t := range tabs {
		title := t.title
		if short {
			title = t.short
		}

		if t.id == tabJobs {
			if n := m.runningJobs(); n > 0 {
				title += fmt.Sprintf("·%d", n)
			}
		}

		label := " " + t.key + " " + title + " "
		width := clabinternaltuiui.Width(label)

		if t.id == active {
			b.WriteString(m.th.ChipActive.UnsetPadding().Render(label))
		} else {
			b.WriteString(" " + m.th.Key.Render(t.key) + " " + m.th.Muted.Render(title) + " ")
		}

		zones = append(zones, zone{from: pos, to: pos + width, tab: t.id})
		pos += width
	}

	return b.String(), zones
}

func (m *Model) runningJobs() int {
	n := 0

	for i := range m.jobs {
		if !m.jobs[i].State.Done() {
			n++
		}
	}

	return n
}

// Levels of detail of the context shown in the header.
const (
	contextMinimal = iota
	contextShort
	contextFull
)

// headerContext summarizes what the views cover: the lab scope, the runtime, how many
// nodes are ready and how much traffic flows.
func (m *Model) headerContext(detail int) string {
	labs := m.scopedLabs()

	var ready, total int

	var rx, tx float64

	for i := range labs {
		if !labs[i].Deployed {
			continue
		}

		total += len(labs[i].Nodes)
		ready += labs[i].ReadyNodes()

		for n := range labs[i].Nodes {
			rx += labs[i].Nodes[n].RxBps
			tx += labs[i].Nodes[n].TxBps
		}
	}

	scope := "all labs"

	switch {
	case m.scope != "" && detail == contextMinimal:
		scope = m.scope
	case m.scope != "":
		scope = "lab: " + m.scope
	}

	level := clabinternaltuitheme.LevelSuccess

	switch {
	case total == 0:
		level = clabinternaltuitheme.LevelMuted
	case ready < total:
		level = clabinternaltuitheme.LevelWarning
	}

	count := fmt.Sprintf(" %d/%d nodes", ready, total)
	if detail == contextMinimal {
		count = fmt.Sprintf(" %d/%d", ready, total)
	}

	nodes := m.th.LevelStyle(level).Render(glyphReady) + m.th.Text.Render(count)
	sep := m.th.Faint.Render(" · ")

	parts := []string{m.th.Text.Render(scope)}

	if detail == contextFull {
		parts = append(parts, m.th.Muted.Render(m.host.Runtime))
	}

	parts = append(parts, nodes)

	if detail == contextFull && (rx > 0 || tx > 0) {
		parts = append(parts, m.th.Muted.Render(
			arrowRx+clabinternaltuiui.Rate(rx)+" "+arrowTx+clabinternaltuiui.Rate(tx)))
	}

	return strings.Join(parts, sep) + " "
}

// titleRule renders the line under the header: where the user is, and the state of the
// view, set in a horizontal rule.
func (m *Model) titleRule() string {
	p := m.top()
	rule := m.th.Border.Render

	crumbs := p.crumbs(m)
	for i, c := range crumbs {
		if i == len(crumbs)-1 {
			crumbs[i] = m.th.Title.Render(c)
		} else {
			crumbs[i] = m.th.Muted.Render(c)
		}
	}

	left := rule(
		ruleChar+ruleChar,
	) + " " + strings.Join(
		crumbs,
		m.th.Faint.Render(crumbSeparator),
	) + " "

	var right strings.Builder

	for _, seg := range p.info(m) {
		right.WriteString(" " + m.th.Text.Render(seg) + " " + rule(ruleChar+ruleChar))
	}

	fill := m.width - clabinternaltuiui.Width(left) - clabinternaltuiui.Width(right.String())
	if fill < 1 {
		return clabinternaltuiui.PadRight(clabinternaltuiui.Truncate(left, m.width), m.width)
	}

	return left + rule(strings.Repeat(ruleChar, fill)) + right.String()
}

// hintLine renders the keys of the current context. Hints that do not fit are left out
// from the end, the last one (help) always stays.
func (m *Model) hintLine() string {
	hints := m.top().hints(m)
	if n := len(m.overlays); n > 0 {
		hints = m.overlays[n-1].hints()
	}

	if m.prompt != nil {
		hints = m.prompt.hints()
	}

	render := func(h hint) string {
		return m.th.Key.Render(h.key) + " " + m.th.KeyDesc.Render(h.desc)
	}

	const gap = "  "

	var lastHint *hint

	last := ""

	if len(hints) > 0 {
		lastHint = &hints[len(hints)-1]
		last = render(*lastHint)
		hints = hints[:len(hints)-1]
	}

	line := " "
	budget := m.width - 1 - clabinternaltuiui.Width(last) - len(gap)
	m.hintZones = m.hintZones[:0]

	// Each hint is remembered with where it is drawn, so that it can be clicked.
	zone := func(h hint, part string) {
		if key, ok := hintKey(h); ok {
			from := clabinternaltuiui.Width(line)
			m.hintZones = append(m.hintZones, hintZone{
				from: from, to: from + clabinternaltuiui.Width(part), key: key,
			})
		}
	}

	for _, h := range hints {
		part := render(h)
		if clabinternaltuiui.Width(line)+clabinternaltuiui.Width(part)+len(gap) > budget {
			break
		}

		zone(h, part)

		line += part + gap
	}

	if lastHint != nil {
		zone(*lastHint, last)
	}

	return clabinternaltuiui.PadRight(line+last, m.width)
}

// statusLine renders the last line: the input when the user is typing a filter or a
// command, otherwise the latest notification.
func (m *Model) statusLine() string {
	right := m.statusBadges()
	room := m.width - clabinternaltuiui.Width(right)

	left := " " + m.toast.view(m.th)
	if m.prompt != nil {
		left = " " + m.prompt.view()
	}

	return clabinternaltuiui.PadRight(left, room) + right
}

// statusBadges flags the conditions the user should keep in mind.
func (m *Model) statusBadges() string {
	var badges []string

	if m.host.Demo {
		badges = append(badges, m.th.Warning.Render("demo"))
	}

	if !m.host.Privileged {
		badges = append(badges, m.th.Warning.Render("read-only (no root)"))
	}

	if len(badges) == 0 {
		return ""
	}

	return strings.Join(badges, m.th.Faint.Render(" · ")) + " "
}

// toast is a notification shown in the status line for a while.
type toast struct {
	level clabinternaltuitheme.Level
	text  string
	until time.Time
	// welcome marks the line of basics shown at the start, which the first key
	// press puts away.
	welcome bool
}

// Durations a notification stays visible, by severity.
const (
	toastInfo    = 4 * time.Second
	toastWarning = 8 * time.Second
	toastError   = 20 * time.Second
)

// notifyf shows a notification in the status line.
func (m *Model) notifyf(level clabinternaltuitheme.Level, format string, args ...any) {
	ttl := toastInfo

	switch level {
	case clabinternaltuitheme.LevelWarning:
		ttl = toastWarning
	case clabinternaltuitheme.LevelError:
		ttl = toastError
	case clabinternaltuitheme.LevelNone, clabinternaltuitheme.LevelSuccess,
		clabinternaltuitheme.LevelInfo, clabinternaltuitheme.LevelMuted:
	}

	m.toast = toast{
		level: level,
		text:  clabinternaltuiui.Sanitize(fmt.Sprintf(format, args...)),
		until: m.now().Add(ttl),
	}
}

func (t *toast) expire(now time.Time) {
	if t.text != "" && now.After(t.until) {
		*t = toast{}
	}
}

func (t *toast) view(th *clabinternaltuitheme.Theme) string {
	if t.text == "" {
		return ""
	}

	icon := "•"

	switch t.level {
	case clabinternaltuitheme.LevelSuccess:
		icon = "✓"
	case clabinternaltuitheme.LevelWarning:
		icon = "!"
	case clabinternaltuitheme.LevelError:
		icon = "✗"
	case clabinternaltuitheme.LevelNone, clabinternaltuitheme.LevelInfo,
		clabinternaltuitheme.LevelMuted:
	}

	return th.LevelStyle(t.level).Render(icon) + " " + th.Text.Render(t.text)
}

// readinessLook returns the glyph and the severity that stand for a node's readiness.
func readinessLook(r clabinternaltuibackend.Readiness) (string, clabinternaltuitheme.Level) {
	switch r {
	case clabinternaltuibackend.Ready:
		return glyphReady, clabinternaltuitheme.LevelSuccess
	case clabinternaltuibackend.Starting, clabinternaltuibackend.Unhealthy,
		clabinternaltuibackend.Paused, clabinternaltuibackend.Stopping:
		return glyphPartial, clabinternaltuitheme.LevelWarning
	case clabinternaltuibackend.Stopped, clabinternaltuibackend.Failed:
		return glyphDown, clabinternaltuitheme.LevelError
	default:
		return glyphNone, clabinternaltuitheme.LevelMuted
	}
}

// labLook returns the glyph and the severity that stand for the state of a lab.
func labLook(s clabinternaltuibackend.LabState) (string, clabinternaltuitheme.Level) {
	switch s {
	case clabinternaltuibackend.LabRunning:
		return glyphReady, clabinternaltuitheme.LevelSuccess
	case clabinternaltuibackend.LabPartial:
		return glyphPartial, clabinternaltuitheme.LevelWarning
	case clabinternaltuibackend.LabStopped, clabinternaltuibackend.LabFailed:
		return glyphDown, clabinternaltuitheme.LevelError
	default:
		return glyphNone, clabinternaltuitheme.LevelMuted
	}
}
