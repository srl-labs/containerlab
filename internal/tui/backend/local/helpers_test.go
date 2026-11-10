// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	testTimeout = 10 * time.Second
	testRuntime = "docker"
	testExe     = "/usr/bin/containerlab"
)

// fakeClock is a clock that only moves when a test advances it.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeTimer
}

type fakeTimer struct {
	at time.Time
	ch chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.March, 2, 10, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan time.Time, 1)
	c.waiters = append(c.waiters, fakeTimer{at: c.now.Add(d), ch: ch})

	return ch
}

// advance moves the clock and fires the timers that are due.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)

	pending := c.waiters[:0]

	for _, w := range c.waiters {
		if w.at.After(c.now) {
			pending = append(pending, w)

			continue
		}

		w.ch <- c.now
	}

	c.waiters = pending
}

// awaitTimer blocks until a timer waits for the clock to advance.
func (c *fakeClock) awaitTimer(t *testing.T) {
	t.Helper()

	eventually(t, "a timer to be armed", func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()

		return len(c.waiters) > 0
	})
}

// eventually polls cond until it holds and fails the test if it does not in time.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(testTimeout)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(time.Millisecond)
	}
}

// fakeRunner starts processes that only exist in the test.
type fakeRunner struct {
	mu sync.Mutex
	// err makes every start fail, errFor the ones it returns an error for.
	err    error
	errFor func(spec *procSpec) error
	procs  []*fakeProc
}

func (r *fakeRunner) start(spec *procSpec) (process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return nil, r.err
	}

	if r.errFor != nil {
		if err := r.errFor(spec); err != nil {
			return nil, err
		}
	}

	p := &fakeProc{spec: spec, exit: make(chan int, 1)}
	if spec.stdin {
		p.in = &fakeStdin{}
	}

	r.procs = append(r.procs, p)

	return p, nil
}

// proc returns the n-th process that was started, waiting for it if need be.
func (r *fakeRunner) proc(t *testing.T, n int) *fakeProc {
	t.Helper()

	eventually(t, "a process to be started", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()

		return len(r.procs) > n
	})

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.procs[n]
}

func (r *fakeRunner) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.procs)
}

// endAll makes every process exit that a test left running.
func (r *fakeRunner) endAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.procs {
		select {
		case p.exit <- -1:
		default:
		}
	}
}

// fakeProc is a process whose output and exit a test controls.
type fakeProc struct {
	spec *procSpec
	exit chan int
	in   *fakeStdin

	mu      sync.Mutex
	signals []syscall.Signal
	// onSignal, when set, is called for every signal the process receives.
	onSignal func(sig syscall.Signal)
}

func (p *fakeProc) stdin() io.WriteCloser {
	if p.in == nil {
		return nil
	}

	return p.in
}

func (p *fakeProc) signal(sig syscall.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, sig)
	onSignal := p.onSignal
	p.mu.Unlock()

	if onSignal != nil {
		onSignal(sig)
	}

	return nil
}

func (p *fakeProc) wait() (int, error) { return <-p.exit, nil }

// exitWith makes the process exit with the given code.
func (p *fakeProc) exitWith(code int) { p.exit <- code }

func (p *fakeProc) received() []syscall.Signal {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]syscall.Signal(nil), p.signals...)
}

// awaitSignals blocks until the process received n signals.
func (p *fakeProc) awaitSignals(t *testing.T, n int) {
	t.Helper()

	eventually(t, "signals to be sent", func() bool { return len(p.received()) >= n })
}

func (p *fakeProc) stderr(text string) { _, _ = io.WriteString(p.spec.stderr, text) }

func (p *fakeProc) stdout(text string) { _, _ = io.WriteString(p.spec.stdout, text) }

// fakeStdin records what is written to the standard input of a fake process.
type fakeStdin struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
	// broken makes writes fail, as they do once the reading process is gone.
	broken bool
}

func (s *fakeStdin) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.broken || s.closed {
		return 0, syscall.EPIPE
	}

	return s.buf.Write(p)
}

func (s *fakeStdin) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	return nil
}

func (s *fakeStdin) content() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buf.String()
}

func (s *fakeStdin) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func (s *fakeStdin) breakPipe() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.broken = true
}

// testBackend is a backend wired to fakes.
type testBackend struct {
	*Backend

	run   *fakeRunner
	clock *fakeClock
}

// newTestBackend returns a privileged backend that runs fake processes on a fake clock. The
// given functions adjust the options and dependencies before the backend is built.
func newTestBackend(t *testing.T, adjust ...func(o *Options, d *deps)) *testBackend {
	t.Helper()

	run, clock := &fakeRunner{}, newFakeClock()
	opts := Options{}
	d := deps{
		clab:     &clabcore.CLab{Runtimes: map[string]clabruntime.ContainerRuntime{}},
		rtName:   testRuntime,
		exe:      testExe,
		priv:     privilege{ok: true},
		run:      run,
		clock:    clock,
		fsRoot:   t.TempDir(),
		lookPath: func(file string) (string, error) { return "/usr/bin/" + file, nil },
		getenv:   desktopEnv,
		environ:  func() []string { return []string{"PATH=/usr/bin", "CLAB_TOPO=x.clab.yml"} },
		version:  func(context.Context) (string, error) { return "0.0.0", nil },
		loadTopo: func(string) (*clabcore.CLab, error) {
			return nil, errors.New("no topology parser in this test")
		},
	}

	for _, fn := range adjust {
		fn(&opts, &d)
	}

	b := newBackend(&opts, &d)

	t.Cleanup(func() {
		run.endAll()
		b.Close()
	})

	return &testBackend{Backend: b, run: run, clock: clock}
}

// desktopEnv is the environment of a desktop session, as far as the backend looks at it.
func desktopEnv(key string) string {
	if key == "WAYLAND_DISPLAY" {
		return "wayland-0"
	}

	return ""
}

// setLabs replaces the inventory with labs built from the given containers.
func (b *testBackend) setLabs(containers ...clabruntime.GenericContainer) {
	b.inv.replace(buildLabs(containers, &inventoryEnv{
		hostname: "host",
		exists:   fileExists,
		sshxLink: b.shares.link,
	}))
}

// awaitJob blocks until a job finished and returns it.
func (b *testBackend) awaitJob(
	t *testing.T,
	id clabinternaltuibackend.JobID,
) clabinternaltuibackend.JobInfo {
	t.Helper()

	var info clabinternaltuibackend.JobInfo

	eventually(t, "the job to finish", func() bool {
		info, _ = b.Job(id)

		return info.State.Done()
	})

	return info
}

// drainTopics returns the topics signaled so far.
func (b *testBackend) drainTopics() clabinternaltuibackend.Topic {
	select {
	case <-b.Notify():
	default:
	}

	return b.Changed()
}

// nodeContainer returns a container the way the runtime lists the node of a lab.
func nodeContainer(lab, node, topoFile string) clabruntime.GenericContainer {
	name := "clab-" + lab + "-" + node

	return clabruntime.GenericContainer{
		Names:   []string{name},
		ID:      name + "-0123456789abcdef0123456789abcdef0123456789abcdef",
		ShortID: "0123456789ab",
		Image:   "ghcr.io/nokia/srlinux:latest",
		State:   stateRunning,
		Status:  "Up 5 minutes",
		Pid:     4242,
		Labels: map[string]string{
			clabconstants.Containerlab: lab,
			clabconstants.NodeName:     node,
			clabconstants.LongName:     name,
			clabconstants.NodeKind:     "nokia_srlinux",
			clabconstants.TopoFile:     topoFile,
		},
	}
}

// writeFile creates a file with the given content and the directories leading to it.
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
