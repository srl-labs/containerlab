// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func TestParseLogLine(t *testing.T) {
	at := func(h, m, s int) time.Duration {
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute +
			time.Duration(s)*time.Second
	}

	tests := map[string]struct {
		line  string
		ok    bool
		tod   time.Duration
		level string
		msg   string
	}{
		"info": {
			line: "15:04:05 INFO Containerlab started version=0.83.0",
			ok:   true, tod: at(15, 4, 5), level: levelInfo,
			msg: "Containerlab started version=0.83.0",
		},
		"debug": {
			line:  "00:00:01 DEBU x",
			ok:    true,
			tod:   at(0, 0, 1),
			level: levelDebug,
			msg:   "x",
		},
		"warning": {
			line:  "23:59:59 WARN y",
			ok:    true,
			tod:   at(23, 59, 59),
			level: levelWarn,
			msg:   "y",
		},
		"error": {
			line:  "09:00:00 ERRO z",
			ok:    true,
			tod:   at(9, 0, 0),
			level: levelError,
			msg:   "z",
		},
		"fatal": {
			line:  "09:00:00 FATA z",
			ok:    true,
			tod:   at(9, 0, 0),
			level: levelFatal,
			msg:   "z",
		},
		"no message": {
			line: "09:00:00 INFO", ok: true, tod: at(9, 0, 0), level: levelInfo,
		},
		"message keeps its inner spacing": {
			line: "09:00:00 INFO   indented  text", ok: true, tod: at(9, 0, 0),
			level: levelInfo, msg: "  indented  text",
		},
		"dated line from before the time format is set": {
			line: "2026/11/10 18:39:52 DEBU Env: CLAB_VERSION_CHECK=disable",
			ok:   true, tod: at(18, 39, 52), level: levelDebug,
			msg: "Env: CLAB_VERSION_CHECK=disable",
		},
		"unknown level":        {line: "09:00:00 NOTE z"},
		"level glued to text":  {line: "09:00:00 INFOz"},
		"lower case level":     {line: "09:00:00 info z"},
		"invalid time":         {line: "25:00:00 INFO z"},
		"no time":              {line: "INFO z"},
		"continuation line":    {line: "  │ more of the previous record"},
		"plain error":          {line: "lab 'x' not found"},
		"empty":                {line: ""},
		"date without a time":  {line: "2026/11/10 something else"},
		"table output":         {line: "+------+------+"},
		"time without a level": {line: "09:00:00 something"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tod, level, msg, ok := parseLogLine(tt.line)
			if ok != tt.ok || tod != tt.tod || level != tt.level || msg != tt.msg {
				t.Errorf("parseLogLine(%q) = (%v, %q, %q, %v), want (%v, %q, %q, %v)",
					tt.line, tod, level, msg, ok, tt.tod, tt.level, tt.msg, tt.ok)
			}
		})
	}
}

func TestAtTimeOfDay(t *testing.T) {
	loc := time.FixedZone("test", 3600)
	tod := func(h, m int) time.Duration {
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
	}

	tests := map[string]struct {
		received time.Time
		tod      time.Duration
		want     time.Time
	}{
		"same day": {
			received: time.Date(2026, 3, 2, 10, 0, 1, 0, loc),
			tod:      tod(10, 0),
			want:     time.Date(2026, 3, 2, 10, 0, 0, 0, loc),
		},
		"logged before midnight, received after": {
			received: time.Date(2026, 3, 2, 0, 0, 1, 0, loc),
			tod:      tod(23, 59),
			want:     time.Date(2026, 3, 1, 23, 59, 0, 0, loc),
		},
		"slightly ahead of the receiver": {
			received: time.Date(2026, 3, 2, 10, 0, 0, 0, loc),
			tod:      tod(10, 1),
			want:     time.Date(2026, 3, 2, 10, 1, 0, 0, loc),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := atTimeOfDay(tt.received, tt.tod); !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectLevel(t *testing.T) {
	tests := map[string]string{
		"[INFO ] sr_linux_mgr: chassis manager ready":        levelInfo,
		"2026-03-02T10:00:00Z WARN neighbor down":            levelWarn,
		"ERROR: connection refused":                          levelError,
		"<DEBUG> programmed 12 routes":                       levelDebug,
		`time="2026-03-02" level=error msg="boom"`:           levelError,
		`{"level":"warning","msg":"slow"}`:                   levelWarn,
		"lvl=crit something broke":                           levelFatal,
		"Mar  2 10:00:00 leaf1 bgpd[42]: FATAL no router id": levelFatal,
		"NOTICE: configuration saved":                        levelInfo,
		"10:00:00 TRACE packet in":                           levelDebug,
		"the interface reported an error and went down":      "",
		"Error while loading shared libraries":               "",
		"starting information service":                       "",
		"":                                                   "",
		"some text that is longer than the part that is looked at for a level: ERROR": "",
		"one two three four five six seven eight nine ERROR":                          "",
	}

	for line, want := range tests {
		if got := detectLevel(line); got != want {
			t.Errorf("detectLevel(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestLineWriter(t *testing.T) {
	var lines []string

	w := &lineWriter{emit: func(line string) { lines = append(lines, line) }}

	for _, chunk := range []string{"first\nsec", "ond\n\nthi", "rd"} {
		if n, err := w.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}

	if diff := cmp.Diff([]string{"first", "second", ""}, lines); diff != "" {
		t.Fatalf("unexpected lines before the end (-want +got):\n%s", diff)
	}

	w.close()
	w.close()

	if diff := cmp.Diff([]string{"first", "second", "", "third"}, lines); diff != "" {
		t.Errorf("unexpected lines (-want +got):\n%s", diff)
	}
}

func TestLineWriterCutsEndlessLines(t *testing.T) {
	var sizes []int

	w := &lineWriter{emit: func(line string) { sizes = append(sizes, len(line)) }}

	// Data without line breaks must not pile up.
	chunk := []byte(strings.Repeat("x", maxLineBytes/2+1))
	for range 5 {
		_, _ = w.Write(chunk)
	}

	_, _ = w.Write([]byte("\n"))

	total := 0

	for _, size := range sizes {
		if size > maxLineBytes {
			t.Errorf("line of %d bytes exceeds the limit of %d", size, maxLineBytes)
		}

		total += size
	}

	if want := 5 * len(chunk); total != want {
		t.Errorf("%d bytes came out, %d went in", total, want)
	}

	if len(sizes) < 3 {
		t.Errorf("the data was cut into %d lines, expected at least 3", len(sizes))
	}
}

func TestLineRing(t *testing.T) {
	texts := func(lines []clabinternaltuibackend.LogLine) []string {
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			out = append(out, line.Text)
		}

		return out
	}

	ring := newLineRing(3)

	if got := ring.from(0); got != nil {
		t.Errorf("an empty ring returned %v", got)
	}

	for _, text := range []string{"a", "b"} {
		ring.add(clabinternaltuibackend.LogLine{Text: text})
	}

	if diff := cmp.Diff([]string{"a", "b"}, texts(ring.from(0))); diff != "" {
		t.Errorf("before wrapping (-want +got):\n%s", diff)
	}

	for _, text := range []string{"c", "d", "e"} {
		ring.add(clabinternaltuibackend.LogLine{Text: text})
	}

	if ring.total != 5 {
		t.Errorf("total = %d, want 5", ring.total)
	}

	tests := map[int][]string{
		// The lines 0 and 1 were dropped.
		0:  {"c", "d", "e"},
		2:  {"c", "d", "e"},
		3:  {"d", "e"},
		4:  {"e"},
		5:  {},
		9:  {},
		-1: {"c", "d", "e"},
	}

	for from, want := range tests {
		if diff := cmp.Diff(want, texts(ring.from(from))); diff != "" {
			t.Errorf("from(%d) (-want +got):\n%s", from, diff)
		}
	}
}
