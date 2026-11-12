// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
)

// streamBackend is the demo backend with log streams a test feeds by hand.
type streamBackend struct {
	*clabinternaltuibackendfake.Backend

	streams []*handStream
}

func (b *streamBackend) Logs(
	_ context.Context, node clabinternaltuibackend.NodeRef, _ clabinternaltuibackend.LogOptions,
) (clabinternaltuibackend.LogStream, error) {
	s := &handStream{node: node.Name, ch: make(chan []clabinternaltuibackend.LogLine, 1)}
	b.streams = append(b.streams, s)

	return s, nil
}

// handStream is a log stream that ends when it is closed, with the error of a stream that
// was canceled.
type handStream struct {
	node   string
	ch     chan []clabinternaltuibackend.LogLine
	closed bool
}

func (s *handStream) Lines() <-chan []clabinternaltuibackend.LogLine { return s.ch }

func (s *handStream) Err() error {
	if s.closed {
		return context.Canceled
	}

	return nil
}

func (s *handStream) Close() {
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

func (s *handStream) send(text string) {
	s.ch <- []clabinternaltuibackend.LogLine{{Text: text}}
}

func logText(p *logsPage) string {
	var text strings.Builder

	_, _ = p.log.WriteTo(&text)

	return text.String()
}

// TestLogStreamsDoNotMix replaces the stream of a log view while it still has a batch and
// its end to deliver. Neither may reach what the view shows of the stream that follows.
func TestLogStreamsDoNotMix(t *testing.T) {
	tests := map[string]struct {
		key      string
		wantNode string
	}{
		"reload":        {key: "r", wantNode: "leaf1"},
		"next node":     {key: "]", wantNode: "leaf2"},
		"previous node": {key: "[", wantNode: "spine2"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			be := &streamBackend{Backend: clabinternaltuibackendfake.New(
				clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
			)}
			m := New(be, Options{Now: fixedClock, Home: "/home/demo"})
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})

			// The command that reads from the first stream is kept, to deliver what the
			// stream still has after it was replaced.
			stale := m.openLogs(clabinternaltuibackend.NodeRef{
				Lab: "dc1", Name: "leaf1", Container: "clab-dc1-leaf1",
			})

			page, ok := m.top().(*logsPage)
			if !ok || len(be.streams) != 1 {
				t.Fatalf("no log view was opened: %T, %d streams", m.top(), len(be.streams))
			}

			be.streams[0].send("line of the first stream")
			press(m, tt.key)

			if len(be.streams) != 2 || !be.streams[0].closed ||
				be.streams[1].node != tt.wantNode || page.ref.Name != tt.wantNode {
				t.Fatalf("%s did not open a stream of %s: %d streams, view of %s",
					tt.key, tt.wantNode, len(be.streams), page.ref.Name)
			}

			// The batch that was on its way arrives, then the end of the stream.
			for range 2 {
				msg, isBatch := stale().(logBatchMsg)
				if !isBatch {
					t.Fatal("the first stream delivered no batch")
				}

				// Nothing is read on behalf of a stream that was replaced.
				if _, cmd := m.Update(msg); cmd != nil {
					t.Error("a batch of the replaced stream was followed up")
				}
			}

			if got := logText(page); got != "" {
				t.Errorf("lines of the replaced stream are shown:\n%s", got)
			}

			if page.ended != "" {
				t.Errorf("the end of the replaced stream ended the new one: %q", page.ended)
			}

			// The stream that replaced it is read as usual.
			be.streams[1].send("line of the second stream")
			m.Update(page.next()())

			if got := logText(page); !strings.Contains(got, "line of the second stream") ||
				strings.Contains(got, "first") || page.log.Len() != 1 {
				t.Errorf("unexpected log:\n%s", got)
			}

			// Its own end is reported.
			be.streams[1].Close()
			m.Update(page.next()())

			if want := context.Canceled.Error(); page.ended != want {
				t.Errorf("ended = %q, want %q", page.ended, want)
			}
		})
	}
}

// TestClosedLogViewIgnoresItsStream closes a log view that is reopened by a reload: what
// its last stream still delivers does not come back.
func TestClosedLogViewIgnoresItsStream(t *testing.T) {
	be := &streamBackend{Backend: clabinternaltuibackendfake.New(
		clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
	)}
	m := New(be, Options{Now: fixedClock, Home: "/home/demo"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})

	stale := m.openLogs(clabinternaltuibackend.NodeRef{
		Lab: "dc1", Name: "leaf1", Container: "clab-dc1-leaf1",
	})

	page, ok := m.top().(*logsPage)
	if !ok {
		t.Fatalf("no log view was opened: %T", m.top())
	}

	be.streams[0].send("late line")
	page.close()

	if cmd := page.update(m, stale()); cmd != nil || page.log.Len() != 0 {
		t.Errorf("a closed view took %d lines of its stream", page.log.Len())
	}

	if err := be.streams[0].Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("the stream was not closed: %v", err)
	}
}
