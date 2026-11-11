// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// liveCapture starts a live capture of the first link of the dc1 lab and returns its page.
func liveCapture(t *testing.T, m *Model) *capturePage {
	t.Helper()

	// The link list, the capture dialog, the choice of where the packets go, start.
	press(m, "enter", "4", "C", "tab", "space", "enter")

	page, ok := m.top().(*capturePage)
	if !ok {
		t.Fatalf("expected the live view of the capture, got %T", m.top())
	}

	return page
}

func TestScreenCaptureDialog(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "4", "C", "tab", "space")
	snapshot(t, m, "capture_dialog_120x35")
}

func TestScreenLiveCapture(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	liveCapture(t, m)
	snapshot(t, m, "capture_waiting_120x35")

	advance(m, be, 5)
	snapshot(t, m, "capture_live_120x35")

	press(m, "enter")
	snapshot(t, m, "capture_packet_120x35")
}

func TestScreenLiveCaptureNarrow(t *testing.T) {
	m, be := newTestModel(t, 80, 24)
	liveCapture(t, m)
	advance(m, be, 8)
	snapshot(t, m, "capture_live_80x24")
}

func TestScreenRemoteCapture(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	// The last choice is one step to the left of the first.
	press(m, "enter", "4", "C", "tab", "left", "enter")
	snapshot(t, m, "capture_remote_120x35")
}

// TestLiveCaptureFitsAnySize renders the live view at many window sizes.
func TestLiveCaptureFitsAnySize(t *testing.T) {
	for width := minWidth; width <= 200; width += 13 {
		for _, height := range []int{minHeight, 20, 35} {
			m, be := newTestModel(t, width, height)
			liveCapture(t, m)
			advance(m, be, 4)
			assertScreenFits(t, m, m.render())

			press(m, "enter")
			assertScreenFits(t, m, m.render())
		}
	}
}

func TestLiveCaptureFollowsUntilTheCursorMoves(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	page := liveCapture(t, m)

	current := func() int {
		t.Helper()

		number, ok := page.current()
		if !ok {
			t.Fatal("no packet under the cursor")
		}

		return number
	}

	advance(m, be, 2)

	if got := current(); got != 8 {
		t.Fatalf("the cursor is on packet %d, want the newest one, 8", got)
	}

	// Moving up pauses: the cursor stays on its packet while more arrive.
	press(m, "up", "up")
	advance(m, be, 2)

	if got := current(); got != 6 || page.follow {
		t.Errorf("after moving up the cursor is on packet %d (following: %v), want 6",
			got, page.follow)
	}

	if screen := ansi.Strip(m.render()); !strings.Contains(screen, "paused") {
		t.Error("the screen does not tell that the view is paused")
	}

	// Going to the end follows again, and so does f from anywhere.
	press(m, "G")
	advance(m, be, 1)

	if got := current(); got != 20 || !page.follow {
		t.Errorf("after G the cursor is on packet %d (following: %v), want 20", got, page.follow)
	}

	press(m, "f")

	if page.follow {
		t.Error("f does not pause while following")
	}

	press(m, "g", "f")
	advance(m, be, 1)

	if got := current(); got != 24 || !page.follow {
		t.Errorf("after f the cursor is on packet %d (following: %v), want 24", got, page.follow)
	}
}

func TestLiveCaptureFilterAndStop(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	page := liveCapture(t, m)
	advance(m, be, 6)

	press(m, "/")
	typeText(m, "bgp")
	press(m, "enter")

	shown, total := page.tbl.Counts()
	if total != 24 || shown != 8 {
		t.Errorf("the filter shows %d of %d packets, want 8 of 24", shown, total)
	}

	// The capture is stopped without a question: nothing is lost by it.
	press(m, "C")
	advance(m, be, 1)

	job, _ := be.Job(page.id)
	if !job.State.Done() {
		t.Fatalf("the capture still runs: %+v", job)
	}

	screen := ansi.Strip(m.render())
	if !strings.Contains(screen, "capture ended") || !strings.Contains(screen, "24 packets") {
		t.Errorf("the screen does not sum the capture up:\n%s", screen)
	}

	// The packets stay to look at, and the jobs view leads back to them.
	press(m, "6", "enter")

	again, ok := m.top().(*capturePage)
	if !ok {
		t.Fatalf("the job of a live capture opens %T", m.top())
	}

	if _, total := again.tbl.Counts(); total != 24 {
		t.Errorf("the reopened capture shows %d packets, want 24", total)
	}

	press(m, "l")

	if _, ok := m.top().(*jobPage); !ok {
		t.Errorf("l opens %T, want the output of the job", m.top())
	}
}

func TestLiveCaptureKeepsTheNewestPackets(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	page := liveCapture(t, m)

	// More packets than the view keeps, read in one go and bit by bit.
	for range maxShownPackets/4 + 10 {
		be.Tick()
	}

	advance(m, be, 5)

	if len(page.packets) != maxShownPackets || len(page.rows) != maxShownPackets {
		t.Fatalf("the view keeps %d packets and %d rows, want %d", len(page.packets),
			len(page.rows), maxShownPackets)
	}

	job, _ := be.Job(page.id)
	if last := page.packets[len(page.packets)-1].Number; last != job.Packets {
		t.Errorf("the newest packet shown is %d, the capture is at %d", last, job.Packets)
	}

	if screen := ansi.Strip(m.render()); !strings.Contains(screen, "the last 10000 are shown") {
		t.Errorf("the screen does not tell that packets were dropped:\n%s", screen)
	}

	// A packet that is gone has no details any more.
	if detail := newPacketPage(m, page.id, 1); detail != nil {
		t.Error("a dropped packet still has details")
	}
}

func TestCaptureFormOffersWhatTheHostCanDo(t *testing.T) {
	names := func(sinks []captureSink) string {
		out := make([]string, len(sinks))
		for i := range sinks {
			out[i] = sinks[i].name
		}

		return strings.Join(out, " | ")
	}

	m, _ := newTestModel(t, 120, 35)

	if got, want := names(m.captureSinks()),
		"wireshark | live view | live view + file | file | my computer"; got != want {
		t.Errorf("with Wireshark and SSH the sinks are %q, want %q", got, want)
	}

	// Without Wireshark the live view comes first, without SSH there is no other computer.
	m.host.Caps.Wireshark = false
	m.host.SSH = ""

	if got, want := names(m.captureSinks()), "live view | live view + file | file"; got != want {
		t.Errorf("on a plain host the sinks are %q, want %q", got, want)
	}
}

func TestCaptureFormRequests(t *testing.T) {
	type job = clabinternaltuibackend.JobInfo

	toStdout := func(j *job) bool { return strings.HasSuffix(j.Command, "-w -") }
	toFile := func(j *job) bool { return strings.HasSuffix(j.Command, ".pcapng") }

	tests := map[string]struct {
		keys []string
		want func(j *job) bool
	}{
		"wireshark": {nil, func(j *job) bool { return !j.Live && toStdout(j) }},
		"live view": {[]string{"space"}, func(j *job) bool { return j.Live && toStdout(j) }},
		"live view and a file": {
			[]string{"space", "space"},
			func(j *job) bool { return j.Live && toStdout(j) },
		},
		"a file": {
			[]string{"space", "space", "space"},
			func(j *job) bool { return !j.Live && toFile(j) },
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, be := newTestModel(t, 120, 35)
			press(m, "enter", "4", "C", "tab")
			press(m, tc.keys...)
			press(m, "enter")

			jobs := be.Jobs()
			if len(jobs) != 1 || !tc.want(&jobs[0]) {
				t.Errorf("unexpected jobs: %+v", jobs)
			}
		})
	}
}

func TestWrapWords(t *testing.T) {
	tests := map[string]struct {
		text  string
		width int
		want  []string
	}{
		"fits": {"ssh host", 20, []string{"ssh host"}},
		"breaks": {
			"ssh host 'containerlab capture' | wireshark",
			16,
			[]string{"ssh host", "'containerlab", "capture' |", "wireshark"},
		},
		"long word":   {"a /very/long/path b", 6, []string{"a", "/very/", "long/p", "ath b"}},
		"empty":       {"", 10, []string{""}},
		"exact width": {"abc def", 7, []string{"abc def"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := wrapWords(tc.text, tc.width)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("wrapWords(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
			}
		})
	}
}

func TestWrapCommand(t *testing.T) {
	command := "ssh alice@192.0.2.10 'containerlab capture --name lab -w -' | wireshark -k -i -"

	tests := map[int][]string{
		200: {command},
		// The line breaks before the pipe rather than inside what follows it.
		70: {"ssh alice@192.0.2.10 'containerlab capture --name lab -w -'", "| wireshark -k -i -"},
		50: {"ssh alice@192.0.2.10 'containerlab capture --name", "lab -w -' | wireshark -k -i -"},
	}

	for width, want := range tests {
		got := wrapCommand(command, width)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("wrapCommand at %d:\n%s", width, strings.Join(got, "\n"))
		}

		// However it is broken, the lines put together are the command.
		if strings.Join(got, " ") != command {
			t.Errorf("wrapCommand at %d changes the command: %q", width, got)
		}
	}
}

func TestHexDump(t *testing.T) {
	got := hexDump([]byte("containerlab captures\x00\x01"))
	want := []string{
		"0000  63 6f 6e 74 61 69 6e 65  72 6c 61 62 20 63 61 70  containerlab cap",
		"0010  74 75 72 65 73 00 01                              tures..",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("unexpected hex dump:\n%s", strings.Join(got, "\n"))
	}
}
