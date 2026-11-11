// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// Log levels as containerlab prints them.
const (
	levelDebug = "DEBU"
	levelInfo  = "INFO"
	levelWarn  = "WARN"
	levelError = "ERRO"
	levelFatal = "FATA"
)

const (
	// maxLineBytes bounds a line of output. A longer line, ie. binary data without line
	// breaks, is cut into pieces of this size.
	maxLineBytes = 64 << 10
	// tabWidth is the distance between tab stops when tabs are expanded.
	tabWidth = 8

	escape = 0x1b
	bell   = 0x07
	// c1CSI is the single character form of the control sequence introducer.
	c1CSI = 0x9b
)

// ring keeps the most recent items of a sequence and remembers how many were added in
// total, so that readers can address items by their absolute number.
type ring[T any] struct {
	buf   []T
	limit int
	// start is the position of the oldest item once the buffer is full.
	start int
	total int
}

// lineRing keeps the most recent lines of an output.
type lineRing = ring[clabinternaltuibackend.LogLine]

func newLineRing(limit int) lineRing { return lineRing{limit: limit} }

func (r *ring[T]) add(item T) {
	if len(r.buf) < r.limit {
		r.buf = append(r.buf, item)
	} else {
		r.buf[r.start] = item
		r.start = (r.start + 1) % r.limit
	}

	r.total++
}

// from returns a copy of the items numbered n and up. Items that were dropped already are
// skipped.
func (r *ring[T]) from(n int) []T {
	oldest := r.total - len(r.buf)
	n = max(n, oldest)

	if n >= r.total {
		return nil
	}

	out := make([]T, 0, r.total-n)
	for i := n; i < r.total; i++ {
		out = append(out, r.buf[(r.start+i-oldest)%len(r.buf)])
	}

	return out
}

// at returns the item numbered n, if it is still kept.
func (r *ring[T]) at(n int) (item T, ok bool) {
	oldest := r.total - len(r.buf)
	if n < oldest || n >= r.total {
		return item, false
	}

	return r.buf[(r.start+n-oldest)%len(r.buf)], true
}

// lineWriter splits what is written to it into lines and hands them to emit without the
// line break. It is not safe for concurrent use.
type lineWriter struct {
	buf  []byte
	emit func(line string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	n := len(p)

	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			w.buf = append(w.buf, p...)

			break
		}

		w.buf = append(w.buf, p[:i]...)
		w.flush()

		p = p[i+1:]
	}

	for len(w.buf) >= maxLineBytes {
		w.emit(string(w.buf[:maxLineBytes]))
		w.buf = w.buf[:copy(w.buf, w.buf[maxLineBytes:])]
	}

	return n, nil
}

// flush emits the text that is not terminated by a line break yet.
func (w *lineWriter) flush() {
	for len(w.buf) > maxLineBytes {
		w.emit(string(w.buf[:maxLineBytes]))
		w.buf = w.buf[:copy(w.buf, w.buf[maxLineBytes:])]
	}

	w.emit(string(w.buf))
	w.buf = w.buf[:0]
}

// close emits a last line that has no line break, if there is one.
func (w *lineWriter) close() {
	if len(w.buf) > 0 {
		w.flush()
	}
}

// sanitize makes a line of untrusted output safe to show in a terminal: escape sequences
// and control characters are removed, tabs are expanded and invalid UTF-8 is replaced. Of a
// line that was rewritten with carriage returns only the last version is kept.
func sanitize(s string) string {
	s = strings.TrimRight(s, "\r")
	if !needsSanitizing(s) {
		return s
	}

	var (
		out strings.Builder
		col int
	)

	out.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])

		switch {
		case r == escape:
			i += escapeLen(s[i:])

			continue
		case r == c1CSI:
			i += size + csiLen(s[i+size:])

			continue
		case r == '\r':
			out.Reset()

			col = 0
		case r == '\t':
			pad := tabWidth - col%tabWidth
			out.WriteString(strings.Repeat(" ", pad))

			col += pad
		case unicode.IsControl(r):
		default:
			// An invalid byte decodes to the replacement character, which is what is kept.
			out.WriteRune(r)

			col++
		}

		i += size
	}

	return out.String()
}

func needsSanitizing(s string) bool {
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return true
		}
	}

	return false
}

// escapeLen returns the length of the escape sequence s starts with.
func escapeLen(s string) int {
	if len(s) < 2 { //nolint:mnd // the escape character and what it introduces
		return len(s)
	}

	switch s[1] {
	case '[':
		return 2 + csiLen(s[2:]) //nolint:mnd // the introducer is two bytes long
	case ']', 'P', 'X', '^', '_':
		return 2 + stringSeqLen(s[2:]) //nolint:mnd // the introducer is two bytes long
	}

	// A plain escape sequence: optional intermediate bytes and one final byte.
	n := 1
	for n < len(s) && s[n] >= 0x20 && s[n] <= 0x2f {
		n++
	}

	if n < len(s) {
		n++
	}

	return n
}

// csiLen returns the length of the control sequence that follows its introducer: parameter
// and intermediate bytes up to the final byte.
func csiLen(s string) int {
	n := 0
	for n < len(s) && s[n] >= 0x20 && s[n] <= 0x3f {
		n++
	}

	if n < len(s) && s[n] >= 0x40 && s[n] <= 0x7e {
		n++
	}

	return n
}

// stringSeqLen returns the length of the payload of an operating system command or a device
// control string, which lasts until a bell or a string terminator.
func stringSeqLen(s string) int {
	for n := range len(s) {
		switch {
		case s[n] == bell:
			return n + 1
		case s[n] == escape && n+1 < len(s) && s[n+1] == '\\':
			return n + 2 //nolint:mnd // the terminator is two bytes long
		}
	}

	return len(s)
}

// parseLogLine splits a line of the containerlab log, "15:04:05 INFO message", into the
// time of day, the level and the message. The lines logged before the command set its time
// format also carry the date, which is skipped.
func parseLogLine(line string) (tod time.Duration, level, msg string, ok bool) {
	const (
		dateLayout = "2006/01/02"
		timeLen    = len(time.TimeOnly)
		levelLen   = 4
		headLen    = timeLen + 1 + levelLen
	)

	if len(line) > len(dateLayout) && line[len(dateLayout)] == ' ' {
		if _, err := time.Parse(dateLayout, line[:len(dateLayout)]); err == nil {
			line = line[len(dateLayout)+1:]
		}
	}

	if len(line) < headLen || line[timeLen] != ' ' {
		return 0, "", "", false
	}

	if len(line) > headLen && line[headLen] != ' ' {
		return 0, "", "", false
	}

	at, err := time.Parse(time.TimeOnly, line[:timeLen])
	if err != nil {
		return 0, "", "", false
	}

	level = line[timeLen+1 : headLen]
	switch level {
	case levelDebug, levelInfo, levelWarn, levelError, levelFatal:
	default:
		return 0, "", "", false
	}

	tod = time.Duration(at.Hour())*time.Hour + time.Duration(at.Minute())*time.Minute +
		time.Duration(at.Second())*time.Second

	return tod, level, strings.TrimPrefix(line[headLen:], " "), true
}

// atTimeOfDay returns the moment a line carrying only a time of day was logged, given the
// time it was received at. A line logged just before midnight and received after it belongs
// to the previous day.
func atTimeOfDay(received time.Time, tod time.Duration) time.Time {
	const halfDay = 12 * time.Hour

	year, month, day := received.Date()
	at := time.Date(year, month, day, 0, 0, 0, 0, received.Location()).Add(tod)

	if at.Sub(received) > halfDay {
		at = at.AddDate(0, 0, -1)
	}

	return at
}

// isErrorLevel reports whether a level stands for a failure.
func isErrorLevel(level string) bool { return level == levelError || level == levelFatal }

// logLevelWords maps the severities found in the logs of network operating systems to the
// levels containerlab uses.
var logLevelWords = map[string]string{
	"trace":    levelDebug,
	"debug":    levelDebug,
	"debu":     levelDebug,
	"info":     levelInfo,
	"notice":   levelInfo,
	"warn":     levelWarn,
	"warning":  levelWarn,
	"err":      levelError,
	"erro":     levelError,
	"error":    levelError,
	"crit":     levelFatal,
	"critical": levelFatal,
	"fatal":    levelFatal,
	"fata":     levelFatal,
	"panic":    levelFatal,
}

// detectLevel looks for a severity at the start of a log line, such as "[WARN ]", "INFO:" or
// "level=error". A severity counts when it is written in upper case or follows a level key;
// in any other form the word is more likely part of the message.
func detectLevel(line string) string {
	const (
		scanLen  = 64
		maxWords = 8
	)

	if len(line) > scanLen {
		line = line[:scanLen]
	}

	notLetter := func(r rune) bool { return !unicode.IsLetter(r) }
	afterKey := false

	for range maxWords {
		start := strings.IndexFunc(line, unicode.IsLetter)
		if start < 0 {
			break
		}

		line = line[start:]

		end := strings.IndexFunc(line, notLetter)
		if end < 0 {
			end = len(line)
		}

		word := line[:end]
		line = line[end:]
		lower := strings.ToLower(word)

		if lower == "level" || lower == "lvl" || lower == "severity" {
			afterKey = true

			continue
		}

		level, known := logLevelWords[lower]
		if known && (afterKey || word == strings.ToUpper(word)) {
			return level
		}

		afterKey = false
	}

	return ""
}
