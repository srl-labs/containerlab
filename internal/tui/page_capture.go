// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// maxShownPackets is the number of packets the live view of a capture keeps.
	maxShownPackets = 10000
	// packetTimeLayout shows the time of a packet in the list, packetExactTimeLayout in
	// its details.
	packetTimeLayout      = "15:04:05.000"
	packetExactTimeLayout = "15:04:05.000000"
	// hexBytesPerLine is the number of bytes on a line of a hex dump.
	hexBytesPerLine = 16
	// textMargin is the space left and right of the text of a page.
	textMargin = 3
)

// packetCount formats a number of packets.
func packetCount(n int) string {
	if n == 1 {
		return "1 packet"
	}

	return strconv.Itoa(n) + " packets"
}

// capturePage shows the packets of a live capture as they arrive, the newest at the
// bottom.
type capturePage struct {
	basePage

	id  clabinternaltuibackend.JobID
	tbl *clabinternaltuiui.Table
	// packets are the packets shown, rows what the table shows of them.
	packets []clabinternaltuibackend.Packet
	rows    []clabinternaltuiui.Row
	// read is the number of packets that were read from the backend.
	read int
	// follow keeps the cursor on the newest packet.
	follow bool
}

//nolint:mnd // the widths of the columns are what they are
func packetColumns() []clabinternaltuiui.Column {
	return []clabinternaltuiui.Column{
		{ID: "no", Title: "NO", Numeric: true, Align: clabinternaltuiui.AlignRight, Min: 3},
		{ID: "time", Title: "TIME", Min: 12, Priority: 1},
		{ID: "iface", Title: "INTERFACE", Max: 24, Priority: 2},
		{ID: "src", Title: "SOURCE", Min: 15, Max: 30},
		{ID: "dst", Title: "DESTINATION", Min: 15, Max: 30},
		{ID: "proto", Title: "PROTOCOL", Max: 12},
		{
			ID: "len", Title: "LENGTH", Numeric: true, Priority: 3,
			Align: clabinternaltuiui.AlignRight,
		},
		{ID: "info", Title: "INFO", Min: 24, Flex: true},
	}
}

func newCapturePage(m *Model, id clabinternaltuibackend.JobID) *capturePage {
	p := &capturePage{
		id:     id,
		tbl:    clabinternaltuiui.NewTable(m.th, packetColumns()),
		follow: true,
	}
	p.refresh(m, clabinternaltuibackend.TopicAll)

	return p
}

func (*capturePage) tab() tabID                        { return tabJobs }
func (p *capturePage) table() *clabinternaltuiui.Table { return p.tbl }

func (p *capturePage) job(m *Model) (clabinternaltuibackend.JobInfo, bool) {
	return m.be.Job(p.id)
}

func (p *capturePage) crumbs(m *Model) []string {
	title := jobLabel(p.id)
	if job, ok := p.job(m); ok {
		title += " " + job.Title
	}

	return []string{"jobs", title, "packets " + tableCount(p.tbl)}
}

func (p *capturePage) info(m *Model) []string {
	job, ok := p.job(m)
	if !ok {
		return nil
	}

	info := []string{job.State.String() + " " +
		clabinternaltuiui.Duration(job.Duration(m.now()))}
	if job.Bytes > 0 {
		info = append(info, clabinternaltuiui.Bytes(job.Bytes))
	}

	if !job.State.Done() && !p.follow {
		info = append(info, "paused")
	}

	return append(info, tableInfo(p.tbl)...)
}

func (p *capturePage) resize(width, height int) {
	p.basePage.resize(width, height)
	p.tbl.SetSize(width, height-1) // the line that sums the capture up
}

func (p *capturePage) refresh(m *Model, topics clabinternaltuibackend.Topic) {
	if !topics.Has(clabinternaltuibackend.TopicJobOutput | clabinternaltuibackend.TopicJobs) {
		return
	}

	p.tbl.Empty = "waiting for packets — they show up here as they cross the link"
	if job, ok := p.job(m); !ok || job.State.Done() {
		p.tbl.Empty = "no packets were captured"
	}

	packets := m.be.Packets(p.id, p.read)
	if len(packets) == 0 {
		return
	}

	// The numbers count from one, and a packet that was dropped before it was read is
	// counted as read all the same.
	p.read = packets[len(packets)-1].Number

	p.packets = append(p.packets, packets...)
	for i := range packets {
		p.rows = append(p.rows, p.row(m, &packets[i]))
	}

	if extra := len(p.packets) - maxShownPackets; extra > 0 {
		p.packets = slices.Delete(p.packets, 0, extra)
		p.rows = slices.Delete(p.rows, 0, extra)
	}

	p.show()
}

// show hands the rows to the table and keeps following the newest packet.
func (p *capturePage) show() {
	p.tbl.SetRows(p.rows)

	if p.follow {
		p.tbl.MoveToEnd()
	}
}

func (*capturePage) row(m *Model, packet *clabinternaltuibackend.Packet) clabinternaltuiui.Row {
	return clabinternaltuiui.Row{
		Key: strconv.Itoa(packet.Number),
		Cells: []clabinternaltuiui.Cell{
			{Text: strconv.Itoa(packet.Number), Style: m.th.Muted, Num: float64(packet.Number)},
			m.mutedCell(packet.Time.Format(packetTimeLayout)),
			m.cell(clabinternaltuiui.Sanitize(packet.Interface)),
			m.cell(packet.Source),
			m.cell(packet.Destination),
			m.styledCell(packet.Protocol, m.th.Accent),
			{Text: strconv.Itoa(packet.Length), Style: m.th.Muted, Num: float64(packet.Length)},
			m.cell(clabinternaltuiui.Sanitize(packet.Info)),
		},
	}
}

// setTheme draws the rows again, which carry the colors of the theme.
func (p *capturePage) setTheme(*clabinternaltuitheme.Theme) {
	p.rows = p.rows[:0]
}

// restyle rebuilds the rows that a change of the theme dropped.
func (p *capturePage) restyle(m *Model) {
	if len(p.rows) == len(p.packets) {
		return
	}

	p.rows = p.rows[:0]
	for i := range p.packets {
		p.rows = append(p.rows, p.row(m, &p.packets[i]))
	}

	p.show()
}

// current returns the number of the packet under the cursor.
func (p *capturePage) current() (int, bool) {
	number, err := strconv.Atoi(p.tbl.CursorKey())

	return number, err == nil
}

func (p *capturePage) scroll(notches int) {
	p.tbl.Scroll(notches)
	p.follow = p.tbl.AtEnd()
}

func (p *capturePage) key(m *Model, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "f", "space":
		// Following again starts at the newest packet, pausing stays where the cursor is.
		p.follow = !p.follow
		if p.follow {
			p.tbl.MoveToEnd()
		}

		return true, nil
	case "ctrl+a":
		// There is nothing to do with marked packets.
		return true, nil
	case "C":
		return true, m.cancelJob(p.id)
	case "l":
		m.push(newJobPage(m, p.id))

		return true, nil
	}

	if p.tbl.HandleKey(msg) {
		// Moving off the newest packet pauses, moving back to it follows again.
		p.follow = p.tbl.AtEnd()

		return true, nil
	}

	number, ok := p.current()
	if !ok {
		return false, nil
	}

	switch msg.String() {
	case keyEnter, "right":
		if detail := newPacketPage(m, p.id, number); detail != nil {
			m.push(detail)
		}

		return true, nil
	case "y":
		row, _ := p.tbl.Cursor()
		cells := make([]string, 0, len(row.Cells))

		for i := range row.Cells {
			cells = append(cells, row.Cells[i].Text)
		}

		return true, m.copy("packet", strings.Join(cells, "  "))
	}

	return false, nil
}

func (p *capturePage) view(m *Model) string {
	job, ok := p.job(m)
	if !ok {
		return " " + m.th.Muted.Render("this capture is gone")
	}

	p.restyle(m)

	return p.tbl.View() + "\n" + p.result(m, &job)
}

// result sums the capture up in a line below the packets.
func (p *capturePage) result(m *Model, job *clabinternaltuibackend.JobInfo) string {
	_, level := jobLook(job.State)

	var text string

	switch job.State {
	case clabinternaltuibackend.JobRunning, clabinternaltuibackend.JobCancelling:
		text = "capturing… " + packetCount(job.Packets)

		if !p.follow {
			text += " · paused, f follows the newest packet again"
		}
	case clabinternaltuibackend.JobFailed:
		text = "✗ failed"
		if job.Err != "" {
			text += ": " + clabinternaltuiui.Sanitize(job.Err)
		}
	case clabinternaltuibackend.JobSucceeded, clabinternaltuibackend.JobCancelled:
		text = fmt.Sprintf("✓ capture ended after %s · %s",
			clabinternaltuiui.Duration(job.Duration(m.now())), packetCount(job.Packets))
		level = clabinternaltuitheme.LevelSuccess
	}

	if job.Packets > len(p.packets) && len(p.packets) > 0 {
		text += fmt.Sprintf(" · the last %d are shown", len(p.packets))
	}

	return clabinternaltuiui.Truncate(" "+m.th.LevelStyle(level).Render(text), p.width)
}

func (p *capturePage) hints(m *Model) []hint {
	follow := hint{"f", "follow"}
	if p.follow {
		follow.desc = "pause"
	}

	hints := []hint{{"⏎", "details"}, follow}

	if job, ok := p.job(m); ok && !job.State.Done() {
		hints = append(hints, hint{"C", "stop"})
	}

	return append(hints, hint{"/", "filter"}, hint{"y", "copy"}, hint{"l", "log"},
		hint{keyEsc, "back"}, hint{"?", "help"})
}

// newPacketPage returns the page with the details of a packet of a live capture. It
// returns nil, and tells why, when the packet is not kept any longer.
func newPacketPage(m *Model, id clabinternaltuibackend.JobID, number int) *textPage {
	detail, ok := m.be.PacketDetail(id, number)
	if !ok {
		m.notifyf(clabinternaltuitheme.LevelWarning,
			"packet %d is not kept any longer", number)

		return nil
	}

	var lines, plain []string

	add := func(styled, text string) {
		lines = append(lines, " "+styled)
		plain = append(plain, text)
	}

	head := fmt.Sprintf("Packet %d · %s · %s · %d bytes on the wire", detail.Number,
		detail.Time.Format(packetExactTimeLayout), clabinternaltuiui.Sanitize(detail.Interface),
		detail.Length)
	add(m.th.Text.Render(head), head)

	summary := clabinternaltuiui.Sanitize(fmt.Sprintf("%s → %s  %s  %s",
		detail.Source, detail.Destination, detail.Protocol, detail.Info))
	add(m.th.Muted.Render(summary), summary)

	for i := range detail.Layers {
		layer := &detail.Layers[i]

		add("", "")
		add(m.th.Title.Render(layer.Name), layer.Name)

		nameWidth := 0
		for _, field := range layer.Fields {
			nameWidth = max(nameWidth, clabinternaltuiui.Width(field.Name))
		}

		for _, field := range layer.Fields {
			name := clabinternaltuiui.PadRight(field.Name, nameWidth)
			value := clabinternaltuiui.Sanitize(field.Value)

			add("  "+m.th.Muted.Render(name)+"  "+m.th.Text.Render(value),
				"  "+name+"  "+value)
		}
	}

	title := fmt.Sprintf("Bytes (%d)", len(detail.Data))
	if detail.Captured > len(detail.Data) {
		title = fmt.Sprintf("Bytes (the first %d of %d)", len(detail.Data), detail.Captured)
	}

	add("", "")
	add(m.th.Title.Render(title), title)

	for _, line := range hexDump(detail.Data) {
		add("  "+m.th.Text.Render(line), "  "+line)
	}

	path := []string{"jobs", jobLabel(id), "packet " + strconv.Itoa(number)}

	return &textPage{owner: tabJobs, path: path, lines: lines, plain: strings.Join(plain, "\n")}
}

// hexDump renders bytes the usual way: the offset, the bytes in hexadecimal and what they
// are as text.
func hexDump(data []byte) []string {
	lines := make([]string, 0, len(data)/hexBytesPerLine+1)

	for offset := 0; offset < len(data); offset += hexBytesPerLine {
		chunk := data[offset:min(offset+hexBytesPerLine, len(data))]

		var hex, text strings.Builder

		for i := range hexBytesPerLine {
			// A gap in the middle makes the bytes easier to count.
			if i == hexBytesPerLine/2 {
				hex.WriteByte(' ')
			}

			if i >= len(chunk) {
				hex.WriteString("   ")

				continue
			}

			fmt.Fprintf(&hex, "%02x ", chunk[i])

			if chunk[i] >= ' ' && chunk[i] <= '~' {
				text.WriteByte(chunk[i])
			} else {
				text.WriteByte('.')
			}
		}

		lines = append(lines, fmt.Sprintf("%04x  %s %s", offset, hex.String(), text.String()))
	}

	return lines
}

// remoteCapture hands the user the command that runs a capture from their own computer
// and shows it in the Wireshark there: on the clipboard, and on a page to read it from.
func (m *Model) remoteCapture(req *clabinternaltuibackend.CaptureRequest) tea.Cmd {
	command, err := m.be.RemoteCapture(*req)
	if err != nil {
		m.notifyf(clabinternaltuitheme.LevelError, "%v", err)

		return nil
	}

	render := func(m *Model) []string { return remoteCaptureLines(m, command) }

	m.push(&textPage{
		owner:  m.top().tab(),
		path:   []string{"capture", "on my computer"},
		lines:  render(m),
		reload: render,
		plain:  command,
	})

	return m.copy("capture command", command)
}

// remoteSudo is how the command of a capture from the user's computer runs containerlab
// through sudo.
const remoteSudo = "sudo -n "

// remoteCaptureLines explains the command of a capture from the user's computer. The
// command is wrapped for reading; what is copied is the one line it is.
func remoteCaptureLines(m *Model, command string) []string {
	width := max(minWidth, m.width) - 2*textMargin //nolint:mnd // a margin on each side

	lines := []string{
		" " + m.th.Title.Render("Watch this capture in the Wireshark on your computer"),
		"",
		" " + m.th.Text.Render("Run this command in a terminal on your computer, not in this one."),
		" " + m.th.Text.Render("It is on your clipboard already, y copies it again:"),
		"",
	}

	for _, line := range wrapCommand(command, width) {
		lines = append(lines, strings.Repeat(" ", textMargin)+m.th.Accent.Render(line))
	}

	notes := []string{
		"",
		"The capture runs for as long as that Wireshark stays open.",
		"",
		"The address in the command is the one your SSH session came in on. Use the " +
			"name or address you normally reach this host with, should it differ.",
	}

	if strings.Contains(command, remoteSudo) {
		notes = append(notes, "",
			"The capture runs through sudo, which cannot ask for a password: SSH gives it "+
				"no terminal to ask in. It needs sudo without a password for containerlab on "+
				"this host, or a containerlab installed with the setuid bit, which members "+
				"of the clab_admins group run without sudo.")
	}

	for _, note := range notes {
		for _, line := range wrapWords(note, width) {
			lines = append(lines, " "+m.th.Muted.Render(line))
		}
	}

	return lines
}

// wrapCommand breaks a command line into lines of at most width cells. The program a
// pipe leads into stays in one piece with its arguments, as far as the width allows.
func wrapCommand(command string, width int) []string {
	const glue = "\u00a0"

	head, tail, piped := strings.Cut(command, " | ")
	if piped {
		command = head + " |" + glue + strings.ReplaceAll(tail, " ", glue)
	}

	lines := wrapWords(command, width)
	for i := range lines {
		lines[i] = strings.ReplaceAll(lines[i], glue, " ")
	}

	return lines
}

// wrapWords breaks a text into lines of at most width cells, at spaces where it can.
func wrapWords(text string, width int) []string {
	if text == "" {
		return []string{""}
	}

	var (
		lines []string
		line  string
	)

	for word := range strings.SplitSeq(text, " ") {
		if word == "" {
			continue
		}

		// A word that does not fit a line on its own is cut.
		for clabinternaltuiui.Width(word) > width {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}

			head := ansi.Truncate(word, width, "")
			if head == "" {
				break
			}

			lines = append(lines, head)
			word = word[len(head):]
		}

		switch {
		case line == "":
			line = word
		case clabinternaltuiui.Width(line)+1+clabinternaltuiui.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return lines
}
