// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	// Log lines are handed to the UI in batches: a chatty node would otherwise cause a
	// redraw per line.
	logBatchLines    = 200
	logBatchInterval = 50 * time.Millisecond
	// logBacklog is the number of lines that may wait for the batcher.
	logBacklog = 1024
)

// logStream is the log of a container being read.
type logStream struct {
	batches chan []clabinternaltuibackend.LogLine
	cancel  context.CancelFunc

	mu  sync.Mutex
	err error
}

// Lines implements clabinternaltuibackend.LogStream.
func (s *logStream) Lines() <-chan []clabinternaltuibackend.LogLine { return s.batches }

// Err implements clabinternaltuibackend.LogStream. A stream that was closed by its reader
// ended without an error.
func (s *logStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

// Close implements clabinternaltuibackend.LogStream.
func (s *logStream) Close() { s.cancel() }

func (s *logStream) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.err = err
}

// Logs streams the log of a node's container, read through the runtime API.
func (b *Backend) Logs(
	ctx context.Context,
	ref clabinternaltuibackend.NodeRef,
	opts clabinternaltuibackend.LogOptions,
) (clabinternaltuibackend.LogStream, error) {
	writer, ok := b.rt.(clabruntime.ContainerLogWriter)
	if !ok {
		return nil, fmt.Errorf("%w: the %s runtime cannot read container logs",
			clabinternaltuibackend.ErrUnavailable, b.rtName)
	}

	_, node, _ := b.inv.find(ref)
	if node == nil {
		return nil, fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)
	}

	logOpts := clabruntime.ContainerLogOptions{Follow: opts.Follow, Timestamps: opts.Timestamps}

	if opts.Tail >= 0 {
		tail := int64(opts.Tail)
		logOpts.Tail = &tail
	}

	ctx, cancel := context.WithCancel(ctx)
	// The stream also ends when the backend is closed.
	stop := context.AfterFunc(b.ctx, cancel)

	stream := &logStream{
		batches: make(chan []clabinternaltuibackend.LogLine, 1),
		cancel:  cancel,
	}
	lines := make(chan clabinternaltuibackend.LogLine, logBacklog)
	container := node.Container

	b.spawn("log reader", func() {
		// Closing the lines ends the batcher, which must see the error by then.
		defer close(lines)

		stdout := newLogLineWriter(ctx, lines, opts.Timestamps, false)
		stderr := newLogLineWriter(ctx, lines, opts.Timestamps, true)

		err := writer.WriteContainerLogs(ctx, container, logOpts, stdout, stderr)

		stdout.close()
		stderr.close()

		if err != nil && ctx.Err() == nil {
			stream.setErr(err)
		}
	})

	b.spawn("log batcher", func() {
		// The context is only released after the last batch went out, it would be dropped
		// otherwise.
		defer stop()
		defer cancel()

		batchLogLines(ctx, lines, stream.batches)
	})

	return stream, nil
}

// newLogLineWriter returns a writer that turns the output of a container into log lines.
// Lines are dropped once ctx ended, so that the writer never blocks a reader that is gone.
func newLogLineWriter(
	ctx context.Context,
	lines chan<- clabinternaltuibackend.LogLine,
	timestamps, stderr bool,
) *lineWriter {
	return &lineWriter{emit: func(raw string) {
		line := parseContainerLogLine(raw, timestamps)
		line.Stderr = stderr

		select {
		case lines <- line:
		case <-ctx.Done():
		}
	}}
}

// parseContainerLogLine turns a line of container output into a log line. With timestamps
// requested the runtime starts every line with the time it was written at.
func parseContainerLogLine(raw string, timestamps bool) clabinternaltuibackend.LogLine {
	var line clabinternaltuibackend.LogLine

	if stamp, rest, found := strings.Cut(raw, " "); timestamps && found {
		if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			line.Time = at
			raw = rest
		}
	}

	line.Text = sanitize(raw)
	line.Level = detectLevel(line.Text)

	return line
}

// batchLogLines collects log lines into batches until the lines end, and closes the
// batches then. A batch is passed on when it is full or a moment after its first line
// arrived. Once ctx ended the batches are no longer delivered, only drained.
func batchLogLines(
	ctx context.Context,
	lines <-chan clabinternaltuibackend.LogLine,
	batches chan<- []clabinternaltuibackend.LogLine,
) {
	defer close(batches)

	var batch []clabinternaltuibackend.LogLine

	timer := time.NewTimer(logBatchInterval)
	defer timer.Stop()

	timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		select {
		case batches <- batch:
		case <-ctx.Done():
		}

		batch = nil
	}

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				flush()

				return
			}

			if len(batch) == 0 {
				timer.Reset(logBatchInterval)
			}

			batch = append(batch, line)
			if len(batch) >= logBatchLines {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}
