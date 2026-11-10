// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

const (
	// logLimit is the number of lines a log view keeps, the oldest are dropped beyond it.
	logLimit = 50000
	// logScrollStep is the number of columns the view moves sideways per key press.
	logScrollStep = 16

	logTimeLayout = "15:04:05.000"
	logLevelWidth = 5
	stderrMark    = "▎"
)

// levelPattern finds the severity words logs commonly carry.
var levelPattern = regexp.MustCompile(
	`(?i)\b(trace|debug|debu|info|notice|warning|warn|error|erro|err|fatal|fata|critical|crit)\b`)

// LogLine is a line of a log.
type LogLine struct {
	Time time.Time
	// Level is the severity of the line when it is known apart from the text. It is
	// then shown in a column of its own.
	Level  string
	Stderr bool
	Text   string
}

// LogView shows a log: it follows the end as lines arrive, scrolls, wraps or cuts long
// lines, and finds text.
type LogView struct {
	th    *clabinternaltuitheme.Theme
	lines []LogLine

	width  int
	height int
	// top is the first line shown. It is ignored while following.
	top     int
	hscroll int

	Follow     bool
	Wrap       bool
	Timestamps bool

	query string
	hits  []int
	hit   int

	dropped int
}

// NewLogView creates a log view that follows the end of the log.
func NewLogView(th *clabinternaltuitheme.Theme) *LogView {
	return &LogView{th: th, Follow: true}
}

// SetTheme changes the theme the log is drawn with.
func (v *LogView) SetTheme(th *clabinternaltuitheme.Theme) { v.th = th }

// SetSize sets the size of the view.
func (v *LogView) SetSize(width, height int) {
	v.width, v.height = width, height
	v.clamp()
}

// Append adds lines to the end of the log. The text is made safe to show first, since a
// log can contain anything.
func (v *LogView) Append(lines ...LogLine) {
	for i := range lines {
		lines[i].Text = Sanitize(lines[i].Text)
	}

	v.lines = append(v.lines, lines...)

	if over := len(v.lines) - logLimit; over > 0 {
		v.lines = v.lines[over:]
		v.dropped += over
		v.top = max(0, v.top-over)
	}

	if v.query != "" {
		v.search()
	}

	v.clamp()
}

// Reset removes every line.
func (v *LogView) Reset() {
	v.lines, v.hits = nil, nil
	v.top, v.hit, v.dropped, v.hscroll = 0, 0, 0, 0
}

// Len returns the number of lines held.
func (v *LogView) Len() int { return len(v.lines) }

func (v *LogView) clamp() {
	v.top = max(0, min(v.top, len(v.lines)-1))
	v.hscroll = max(0, v.hscroll)
}

// prefixWidth is the width of what precedes the text of a line: the mark for standard
// error, the time and the level.
func (v *LogView) prefixWidth() int {
	w := 1

	if v.Timestamps {
		w += len(logTimeLayout) + 1
	}

	if v.hasLevels() {
		w += logLevelWidth + 1
	}

	return w
}

func (v *LogView) hasLevels() bool {
	return len(v.lines) > 0 && v.lines[len(v.lines)-1].Level != ""
}

func (v *LogView) textWidth() int {
	return max(1, v.width-v.prefixWidth())
}

// rowsOf returns the number of screen rows a line takes.
func (v *LogView) rowsOf(i int) int {
	if !v.Wrap {
		return 1
	}

	w := Width(v.lines[i].Text)
	if w == 0 {
		return 1
	}

	return (w + v.textWidth() - 1) / v.textWidth()
}

// firstVisible returns the first line to draw. While following it is chosen so that the
// last line ends at the bottom of the view.
func (v *LogView) firstVisible() int {
	if !v.Follow {
		return v.top
	}

	rows, first := 0, len(v.lines)

	for first > 0 {
		rows += v.rowsOf(first - 1)
		if rows > v.height {
			break
		}

		first--
	}

	return first
}

// View renders the log as exactly the configured number of lines.
func (v *LogView) View() string {
	if v.width <= 0 || v.height <= 0 {
		return ""
	}

	rows := make([]string, 0, v.height)

	for i := v.firstVisible(); i < len(v.lines) && len(rows) < v.height; i++ {
		rows = append(rows, v.renderLine(i)...)
	}

	if len(v.lines) == 0 {
		rows = append(rows, "", v.th.Muted.Render(Center("no output yet", v.width)))
	}

	return FitLines(strings.Join(rows, "\n"), v.width, v.height)
}

func (v *LogView) renderLine(i int) []string {
	line := &v.lines[i]

	var prefix strings.Builder

	if line.Stderr {
		prefix.WriteString(v.th.Error.Render(stderrMark))
	} else {
		prefix.WriteString(" ")
	}

	if v.Timestamps {
		stamp := strings.Repeat(" ", len(logTimeLayout))
		if !line.Time.IsZero() {
			stamp = line.Time.Format(logTimeLayout)
		}

		prefix.WriteString(v.th.Muted.Render(stamp) + " ")
	}

	if v.hasLevels() {
		prefix.WriteString(
			v.levelStyle(line.Level).Render(PadRight(line.Level, logLevelWidth)) + " ",
		)
	}

	width := v.textWidth()
	isHit := v.query != "" && len(v.hits) > 0 && v.hits[v.hit] == i

	if !v.Wrap {
		text := cutRunes(line.Text, v.hscroll, width)

		return []string{prefix.String() + v.styleText(text, isHit)}
	}

	chunks := chunkRunes(line.Text, width)
	rows := make([]string, 0, len(chunks))
	indent := strings.Repeat(" ", v.prefixWidth())

	for k, chunk := range chunks {
		lead := indent
		if k == 0 {
			lead = prefix.String()
		}

		rows = append(rows, lead+v.styleText(chunk, isHit))
	}

	return rows
}

func (v *LogView) levelStyle(level string) lipgloss.Style {
	switch strings.ToLower(level) {
	case "warn", "warning":
		return v.th.Warning
	case "error", "erro", "err", "fatal", "fata", "critical", "crit":
		return v.th.Error
	case "debug", "debu", "trace":
		return v.th.Muted
	case "info", "notice":
		return v.th.Info
	default:
		return v.th.Muted
	}
}

// styleText colors the severity words of a line and marks what the search found.
func (v *LogView) styleText(text string, currentHit bool) string {
	if v.query != "" {
		if positions := foldPositions(text, v.query); len(positions) > 0 {
			match := v.th.Match
			if currentHit {
				match = match.Reverse(true)
			}

			return HighlightRunes(text, positions, v.th.Text, match)
		}
	}

	loc := levelPattern.FindStringIndex(text)
	if loc == nil {
		return v.th.Text.Render(text)
	}

	return v.th.Text.Render(text[:loc[0]]) +
		v.levelStyle(text[loc[0]:loc[1]]).Render(text[loc[0]:loc[1]]) +
		v.th.Text.Render(text[loc[1]:])
}

// foldPositions returns the rune positions of every occurrence of query in text,
// ignoring case. The query must be lower case.
func foldPositions(text, query string) []int {
	lower := []rune(strings.ToLower(text))
	needle := []rune(query)

	var positions []int

	for at := indexRunes(lower, needle, 0); at >= 0; at = indexRunes(lower, needle, at+len(needle)) {
		for k := range needle {
			positions = append(positions, at+k)
		}
	}

	return positions
}

// cutRunes returns up to width cells of s after skipping the first skip runes.
func cutRunes(s string, skip, width int) string {
	r := []rune(s)
	if skip >= len(r) {
		return ""
	}

	return Truncate(string(r[skip:]), width)
}

// chunkRunes splits s into pieces of at most width runes.
func chunkRunes(s string, width int) []string {
	r := []rune(s)
	if len(r) <= width {
		return []string{s}
	}

	chunks := make([]string, 0, len(r)/width+1)

	for len(r) > width {
		chunks = append(chunks, string(r[:width]))
		r = r[width:]
	}

	return append(chunks, string(r))
}

// scrollTo stops following and shows the given line first.
func (v *LogView) scrollTo(line int) {
	if v.Follow {
		v.top = v.firstVisible()
		v.Follow = false
	}

	// Never scroll past the point where the last line sits at the bottom: the view
	// would show less than it can.
	v.top = min(line, v.lastTop())
	v.clamp()
}

// Scroll moves the view by a number of lines. Reaching the end resumes following.
func (v *LogView) Scroll(delta int) {
	first := v.firstVisible()
	target := first + delta

	if delta > 0 && target >= v.lastTop() {
		v.Follow = true

		return
	}

	v.scrollTo(target)
}

// lastTop returns the first line of the view when it shows the end of the log.
func (v *LogView) lastTop() int {
	follow := v.Follow
	v.Follow = true
	first := v.firstVisible()
	v.Follow = follow

	return first
}

// HandleKey applies a navigation key and reports whether it was one.
func (v *LogView) HandleKey(msg tea.KeyPressMsg) bool {
	page := max(1, v.height-1)

	switch msg.String() {
	case "up", "k":
		v.Scroll(-1)
	case "down", "j":
		v.Scroll(1)
	case "pgup", "ctrl+b":
		v.Scroll(-page)
	case "pgdown", "ctrl+f", "space":
		v.Scroll(page)
	case "home", "g":
		v.scrollTo(0)
	case "end", "G":
		v.Follow = true
	case "left":
		v.hscroll = max(0, v.hscroll-logScrollStep)
	case "right":
		if !v.Wrap {
			v.hscroll += logScrollStep
		}
	case "f":
		v.Follow = !v.Follow
		if !v.Follow {
			v.top = v.lastTop()
		}
	case "w":
		v.Wrap = !v.Wrap
		v.hscroll = 0
	case "T":
		v.Timestamps = !v.Timestamps
	case "n":
		v.NextHit(1)
	case "N":
		v.NextHit(-1)
	default:
		return false
	}

	return true
}

// SetQuery sets the text to find and jumps to the closest occurrence.
func (v *LogView) SetQuery(query string) {
	v.query = strings.ToLower(strings.TrimSpace(query))
	v.search()

	if len(v.hits) > 0 {
		// Start from the hit closest to the end, where the eye is while following.
		v.hit = len(v.hits) - 1
		v.showHit()
	}
}

// Query returns the text being searched for.
func (v *LogView) Query() string { return v.query }

func (v *LogView) search() {
	current := -1
	if v.hit < len(v.hits) {
		current = v.hits[v.hit]
	}

	v.hits = v.hits[:0]
	v.hit = 0

	if v.query == "" {
		return
	}

	for i := range v.lines {
		if strings.Contains(strings.ToLower(v.lines[i].Text), v.query) {
			if i == current {
				v.hit = len(v.hits)
			}

			v.hits = append(v.hits, i)
		}
	}
}

// NextHit moves to the next (or with a negative delta the previous) occurrence.
func (v *LogView) NextHit(delta int) {
	if len(v.hits) == 0 {
		return
	}

	v.hit = (v.hit + delta + len(v.hits)) % len(v.hits)
	v.showHit()
}

func (v *LogView) showHit() {
	const context = 3 // lines kept visible above the hit

	v.scrollTo(max(0, v.hits[v.hit]-context))
}

// Status describes the state of the view for a title line.
func (v *LogView) Status() []string {
	var status []string

	if v.Follow {
		status = append(status, "following")
	} else {
		status = append(status, "paused")
	}

	if v.Wrap {
		status = append(status, "wrap")
	}

	if v.query != "" {
		if len(v.hits) == 0 {
			status = append(status, fmt.Sprintf("find: %s (none)", v.query))
		} else {
			status = append(status, fmt.Sprintf("find: %s %d/%d", v.query, v.hit+1, len(v.hits)))
		}
	}

	lines := fmt.Sprintf("%d lines", len(v.lines))
	if v.dropped > 0 {
		lines += fmt.Sprintf(" (%d dropped)", v.dropped)
	}

	return append(status, lines)
}

// WriteTo writes the log as plain text, one line per log line.
func (v *LogView) WriteTo(w io.Writer) (int64, error) {
	var total int64

	for i := range v.lines {
		line := &v.lines[i]

		var b strings.Builder

		if !line.Time.IsZero() {
			b.WriteString(line.Time.Format(time.RFC3339Nano) + " ")
		}

		if line.Level != "" {
			b.WriteString(line.Level + " ")
		}

		b.WriteString(line.Text + "\n")

		n, err := io.WriteString(w, b.String())
		total += int64(n)

		if err != nil {
			return total, err
		}
	}

	return total, nil
}
