// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	wiresharkBinary = "wireshark"
	// wiresharkDelay is how long a capture has to run before Wireshark is opened for it.
	// A capture that cannot start fails within that time, and a window that opens only to
	// show nothing is worse than a short wait.
	wiresharkDelay = 1500 * time.Millisecond
	// maxRelayBacklog bounds the capture data held back until Wireshark runs.
	maxRelayBacklog = 64 << 20
	// captureSizeInterval is the pace the size of a capture file is looked up at.
	captureSizeInterval = time.Second
)

// captureRelay passes the capture data a command writes to its standard output on to the
// program that displays it. Data that arrives before the program runs is held back. Once
// the program is gone the data is dropped, but still taken: the command must be able to
// finish writing while it shuts down.
type captureRelay struct {
	// count is told the size of every piece of data.
	count func(n int)
	// lost is called when the data can no longer be delivered.
	lost func()

	mu      sync.Mutex
	backlog bytes.Buffer
	sink    io.WriteCloser
	// ended is set once no data is delivered any more.
	ended bool
}

// Write implements io.Writer. It never fails.
func (r *captureRelay) Write(p []byte) (int, error) {
	r.count(len(p))

	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case r.ended:
	case r.sink == nil:
		if r.backlog.Len()+len(p) > maxRelayBacklog {
			r.end()
			r.lost()

			break
		}

		r.backlog.Write(p)
	default:
		if _, err := r.sink.Write(p); err != nil {
			r.end()
			r.lost()
		}
	}

	return len(p), nil
}

// attach sets where the data goes and delivers what was held back.
func (r *captureRelay) attach(sink io.WriteCloser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ended {
		sink.Close()

		return
	}

	r.sink = sink

	if _, err := sink.Write(r.backlog.Bytes()); err != nil {
		r.end()
		r.lost()
	}

	r.backlog = bytes.Buffer{}
}

// close ends the delivery, which tells the receiver that the data is complete.
func (r *captureRelay) close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.end()
}

// end stops the delivery. The lock must be held.
func (r *captureRelay) end() {
	r.ended = true
	r.backlog = bytes.Buffer{}

	if r.sink != nil {
		r.sink.Close()
		r.sink = nil
	}
}

// captureToWireshark starts a capture whose data is shown live in Wireshark.
func (b *Backend) captureToWireshark(
	req *clabinternaltuibackend.CaptureRequest,
	spec *jobSpec,
) (clabinternaltuibackend.JobID, error) {
	if reason := b.wiresharkUnavailable(); reason != "" {
		return 0, fmt.Errorf("%w: %s", clabinternaltuibackend.ErrUnavailable, reason)
	}

	wireshark, err := b.lookPath(wiresharkBinary)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", clabinternaltuibackend.ErrUnavailable, err)
	}

	relay := &captureRelay{}

	spec.args = captureArgs(b.rtName, req, "-")
	spec.pipedTo = commandLine(wiresharkBinary, wiresharkArgs())
	spec.needsSink = true
	spec.output = func(j *job) io.Writer {
		relay.count = func(n int) { b.jobs.addBytes(j, n) }
		relay.lost = func() { b.jobs.interrupt(j, false) }

		return relay
	}

	j, err := b.jobs.start(spec)
	if err != nil {
		return 0, err
	}

	b.spawn("wireshark", func() { b.runWireshark(j, relay, wireshark) })

	return j.info.ID, nil
}

// runWireshark opens Wireshark on the data of a capture job once the capture proved to
// run, and ties the two together: closing Wireshark stops the capture, and the end of the
// capture ends the stream Wireshark reads, which leaves its window open on what was
// captured.
func (b *Backend) runWireshark(j *job, relay *captureRelay, wireshark string) {
	defer relay.close()

	select {
	case <-j.done:
		return
	case <-b.clock.After(wiresharkDelay):
	}

	j.mu.Lock()
	over := j.info.State.Done()
	j.sinkAttached = !over
	j.mu.Unlock()

	if over {
		return
	}

	proc, err := b.run.start(b.wiresharkSpec(wireshark))
	if err != nil {
		b.jobs.abort(j, fmt.Sprintf("failed to start Wireshark: %v", err))

		return
	}

	relay.attach(proc.stdin())

	closed := make(chan struct{})

	b.spawn("wireshark exit", func() {
		defer close(closed)

		if code, err := proc.wait(); err != nil || code != 0 {
			b.jobs.abort(j, fmt.Sprintf("Wireshark exited with status %d", code))
		}
	})

	select {
	case <-closed:
		// Nobody looks at the capture any longer.
		b.jobs.interrupt(j, false)
	case <-j.done:
	}
}

// wiresharkSpec describes the Wireshark process: detached from the terminal the UI owns,
// reading the capture from its standard input, and run as the user who called sudo when
// that is how this process became root, as it belongs to that user's desktop session.
func (b *Backend) wiresharkSpec(wireshark string) *procSpec {
	spec := &procSpec{
		path:    wireshark,
		args:    wiresharkArgs(),
		env:     b.environ(),
		stdin:   true,
		session: true,
	}

	if who := invokingUser(os.Geteuid(), b.getenv); who != nil {
		spec.cred = who.cred
		spec.env = withEnv(spec.env, who.env)
	}

	return spec
}

// abort makes a job fail for the given reason and stops its command.
func (m *jobManager) abort(j *job, reason string) {
	j.mu.Lock()

	if j.info.State.Done() || j.failure != "" {
		j.mu.Unlock()

		return
	}

	j.failure = reason
	j.mu.Unlock()

	j.note(m.clock.Now(), levelError, reason)
	m.output.poke()
	m.interrupt(j, false)
}

// followCaptureFile keeps the size of the file a capture job writes current.
func (b *Backend) followCaptureFile(j *job, file string) {
	ticker := time.NewTicker(captureSizeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-j.done:
			b.jobs.setBytes(j, fileSize(file))

			return
		case <-ticker.C:
			b.jobs.setBytes(j, fileSize(file))
		}
	}
}

func fileSize(path string) uint64 {
	info, err := os.Stat(path)
	if err != nil || info.Size() < 0 {
		return 0
	}

	return uint64(info.Size())
}

// setBytes records how much data a job produced so far.
func (m *jobManager) setBytes(j *job, n uint64) {
	j.mu.Lock()
	changed := j.info.Bytes != n
	j.info.Bytes = n
	j.mu.Unlock()

	if changed {
		m.progress.poke()
	}
}
