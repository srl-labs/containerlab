// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	describeTimeout = 20 * time.Second
	execTimeout     = 2 * time.Minute

	percentMax = 100

	sinkWireshark = "wireshark"
	sinkFile      = "file"
)

// describeMsg carries the description of a node back from the backend.
type describeMsg struct {
	ref  clabinternaltuibackend.NodeRef
	text string
	err  error
}

// describe fetches the details of a node and shows them.
func (m *Model) describe(ref clabinternaltuibackend.NodeRef) tea.Cmd {
	be := m.be

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), describeTimeout)
		defer cancel()

		text, err := be.Inspect(ctx, ref)

		return describeMsg{ref: ref, text: text, err: err}
	}
}

func (m *Model) described(msg describeMsg) {
	if msg.err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "cannot describe %s: %v", msg.ref.Name, msg.err)

		return
	}

	raw := strings.Split(strings.TrimRight(msg.text, "\n"), "\n")
	lines := make([]string, len(raw))

	for i, line := range raw {
		line = clabinternaltuiui.Sanitize(line)
		raw[i] = line

		// Keys are dimmed, so that the values are what the eye lands on.
		key, value, found := strings.Cut(line, ":")
		if found && key != "" && !strings.Contains(strings.TrimLeft(key, " "), " ") {
			lines[i] = " " + m.th.Muted.Render(key+":") + m.th.Text.Render(value)
		} else {
			lines[i] = " " + m.th.Text.Render(line)
		}
	}

	plain := strings.Join(raw, "\n")

	m.push(&textPage{
		owner: tabNodes,
		path:  []string{"labs", msg.ref.Lab, msg.ref.Name, "describe"},
		lines: lines,
		plain: plain,
	})
}

// runCommandIn asks for a command and runs it in the given nodes.
func (m *Model) runCommandIn(lab string, nodes []*clabinternaltuibackend.Node) tea.Cmd {
	var refs []clabinternaltuibackend.NodeRef

	for _, n := range nodes {
		if n.Readiness().Running() {
			refs = append(refs, n.NodeRef)
		}
	}

	if len(refs) == 0 {
		m.notifyf(clabinternaltuitheme.LevelWarning, "no running node to run a command in")

		return nil
	}

	names := make([]string, len(refs))
	for i := range refs {
		names[i] = refs[i].Name
	}

	notEmpty := func(value string) error {
		if value == "" {
			return errors.New("enter a command")
		}

		return nil
	}

	submit := func(v formValues) tea.Cmd {
		p := newExecPage(m, lab, refs, v.Text("command"))
		m.push(p)

		return p.start(m)
	}

	return m.ask(newForm(fmt.Sprintf("Run a command in %d node(s)", len(refs)), "Run", submit).
		text(targetList(names)).
		input("command", "command", m.lastCommand, "", notEmpty))
}

// parseDuration reads a duration field. A bare number is taken as milliseconds.
func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	if ms, err := strconv.ParseFloat(value, 64); err == nil {
		if ms < 0 {
			return 0, errors.New("must not be negative")
		}

		return time.Duration(ms * float64(time.Millisecond)), nil
	}

	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return 0, errors.New("use a duration such as 50ms or 2s")
	}

	return d, nil
}

func parsePercent(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}

	p, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
	if err != nil || p < 0 || p > percentMax {
		return 0, errors.New("must be between 0 and 100")
	}

	return p, nil
}

func parseRate(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}

	rate, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("use a whole number of kbit/s")
	}

	return rate, nil
}

func durationField(d time.Duration) string {
	if d == 0 {
		return ""
	}

	return d.String()
}

func numberField(v float64) string {
	if v == 0 {
		return ""
	}

	return strconv.FormatFloat(v, 'f', -1, 64)
}

func checkWith[T any](parse func(string) (T, error)) func(string) error {
	return func(value string) error {
		_, err := parse(value)

		return err
	}
}

func interfaceNames(ifaces []*clabinternaltuibackend.Interface) []string {
	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Node.Name + ":" + iface.Name
	}

	return names
}

// netemForm edits the impairments of interfaces. The form starts from what is set on the
// first of them and writes all values at once, as the underlying command replaces them.
func (m *Model) netemForm(ifaces []*clabinternaltuibackend.Interface) tea.Cmd {
	if !m.allowed(m.host.Caps.Netem, "netem") {
		return nil
	}

	current := clabinternaltuibackend.Impairment{}
	if ifaces[0].Netem != nil {
		current = *ifaces[0].Netem
	}

	rate := ""
	if current.RateKbit > 0 {
		rate = strconv.FormatUint(current.RateKbit, 10)
	}

	read := func(v formValues) clabinternaltuibackend.Impairment {
		delay, _ := parseDuration(v.Text("delay"))
		jitter, _ := parseDuration(v.Text("jitter"))
		loss, _ := parsePercent(v.Text("loss"))
		rateKbit, _ := parseRate(v.Text("rate"))
		corruption, _ := parsePercent(v.Text("corruption"))

		return clabinternaltuibackend.Impairment{
			Delay:         delay,
			Jitter:        jitter,
			LossPct:       loss,
			RateKbit:      rateKbit,
			CorruptionPct: corruption,
		}
	}

	submit := func(v formValues) tea.Cmd {
		for _, iface := range ifaces {
			id, err := m.be.SetNetem(clabinternaltuibackend.NetemRequest{
				Node: iface.Node, Interface: iface.Name, Impairment: read(v),
			})
			m.started(id, err, false)
		}

		return nil
	}

	title := "Impairments · " + ifaces[0].Node.Name + ":" + ifaces[0].Name
	if len(ifaces) > 1 {
		title = fmt.Sprintf("Impairments · %d interfaces", len(ifaces))
	}

	return m.ask(newForm(title, "Apply", submit).
		text(
			"Shapes the traffic leaving "+targetList(interfaceNames(ifaces))+".",
			"All values are set at once: an empty field clears that impairment.",
		).
		input("delay", "delay", durationField(current.Delay), "e.g. 50ms", checkWith(parseDuration)).
		input("jitter", "jitter", durationField(current.Jitter), "needs a delay",
			checkWith(parseDuration)).
		input("loss", "loss", numberField(current.LossPct), "percent", checkWith(parsePercent)).
		input("rate", "rate", rate, "kbit/s", checkWith(parseRate)).
		input("corruption", "corruption", numberField(current.CorruptionPct), "percent",
			checkWith(parsePercent)).
		withCheck(func(v formValues) error {
			if netem := read(v); netem.Jitter > 0 && netem.Delay == 0 {
				return errors.New("jitter needs a delay to vary")
			}

			return nil
		}).
		withPreview(func(v formValues) string {
			return netemCommand(ifaces[0], read(v))
		}))
}

func netemCommand(
	iface *clabinternaltuibackend.Interface,
	n clabinternaltuibackend.Impairment,
) string {
	line := fmt.Sprintf(
		"containerlab tools netem set -n %s -i %s",
		iface.Node.Container,
		iface.Name,
	)

	if n.Delay > 0 {
		line += " --delay " + n.Delay.String()
	}

	if n.Jitter > 0 {
		line += " --jitter " + n.Jitter.String()
	}

	if n.LossPct > 0 {
		line += " --loss " + numberField(n.LossPct)
	}

	if n.RateKbit > 0 {
		line += " --rate " + strconv.FormatUint(n.RateKbit, 10)
	}

	if n.CorruptionPct > 0 {
		line += " --corruption " + numberField(n.CorruptionPct)
	}

	return line
}

// netemReset removes the impairments of interfaces.
func (m *Model) netemReset(ifaces []*clabinternaltuibackend.Interface) tea.Cmd {
	if !m.allowed(m.host.Caps.Netem, "netem") {
		return nil
	}

	submit := func(formValues) tea.Cmd {
		for _, iface := range ifaces {
			id, err := m.be.ResetNetem(iface.Node, iface.Name)
			m.started(id, err, false)
		}

		return nil
	}

	return m.ask(newForm("Clear impairments", "Clear", submit).
		text("Remove every impairment from " + targetList(interfaceNames(ifaces)) + "."))
}

// captureForm starts a packet capture on interfaces.
func (m *Model) captureForm(ifaces []*clabinternaltuibackend.Interface) tea.Cmd {
	if !m.allowed(m.host.Caps.Capture, "capture") {
		return nil
	}

	// Captures listen on the interfaces of links: a node's management interface is
	// not part of the lab topology.
	var targets []*clabinternaltuibackend.Interface

	for _, iface := range ifaces {
		if iface.Peer != nil {
			targets = append(targets, iface)
		}
	}

	if len(targets) == 0 {
		m.notifyf(clabinternaltuitheme.LevelWarning, "only the interfaces of links can be captured")

		return nil
	}

	lab := m.lab(targets[0].Node.Lab)
	if lab == nil {
		return nil
	}

	for _, iface := range targets {
		if iface.Node.Lab != lab.Name {
			m.notifyf(
				clabinternaltuitheme.LevelWarning,
				"capture the interfaces of one lab at a time",
			)

			return nil
		}
	}

	sinks := []string{sinkWireshark, sinkFile}
	note := "Wireshark opens on this computer and shows the packets live."

	if !m.host.Caps.Wireshark {
		sinks = []string{sinkFile}
		note = "Wireshark is not available here (" + m.host.Caps.Reason("wireshark") + ")."
	}

	file := fmt.Sprintf("./%s-%s.pcapng", lab.Name, m.now().Format("20060102-150405"))

	submit := func(v formValues) tea.Cmd {
		duration, _ := parseDuration(v.Text("duration"))

		id, err := m.be.Capture(clabinternaltuibackend.CaptureRequest{
			Lab:       lab.Name,
			TopoFile:  lab.TopoFile,
			Targets:   captureTargets(targets),
			Wireshark: v.Text("sink") == sinkWireshark,
			File:      v.Text("file"),
			Duration:  duration,
		})
		m.started(id, err, false)

		if err == nil {
			m.notifyf(clabinternaltuitheme.LevelInfo,
				"capture running — stop it with C in the jobs view (6)")
		}

		return nil
	}

	return m.ask(newForm(fmt.Sprintf("Capture %d interface(s)", len(targets)), "Start", submit).
		text(targetList(interfaceNames(targets)), note).
		choice("sink", "send to", sinks, 0).
		input("file", "file", file, "used when sending to a file", nil).
		input("duration", "stop after", "", "empty: until stopped", checkWith(parseDuration)))
}

// captureTargets groups interfaces by node, the way the capture command takes them.
func captureTargets(
	ifaces []*clabinternaltuibackend.Interface,
) []clabinternaltuibackend.CaptureTarget {
	var targets []clabinternaltuibackend.CaptureTarget

	index := map[string]int{}

	for _, iface := range ifaces {
		i, seen := index[iface.Node.Name]
		if !seen {
			i = len(targets)
			index[iface.Node.Name] = i
			targets = append(targets, clabinternaltuibackend.CaptureTarget{Node: iface.Node.Name})
		}

		targets[i].Interfaces = append(targets[i].Interfaces, iface.Name)
	}

	return targets
}
