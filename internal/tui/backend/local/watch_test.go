// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestDebounce(t *testing.T) {
	start := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return start.Add(time.Duration(ms) * time.Millisecond) }
	d := debounce{quiet: 250 * time.Millisecond, limit: time.Second}

	// A single trigger is acted on once things were quiet for a while.
	d.trigger(at(0))

	if !d.pending || !d.due().Equal(at(250)) {
		t.Errorf("after one trigger: pending=%v due=%v", d.pending, d.due())
	}

	// Every further trigger pushes the action out...
	d.trigger(at(200))

	if !d.due().Equal(at(450)) {
		t.Errorf("after a second trigger: due=%v", d.due())
	}

	// ...but not beyond the limit counted from the first one.
	for ms := 400; ms <= 900; ms += 100 {
		d.trigger(at(ms))
	}

	if !d.due().Equal(at(1000)) {
		t.Errorf("after a burst: due=%v, want the limit", d.due())
	}

	// The next burst starts counting anew.
	d.reset()
	d.trigger(at(5000))

	if !d.due().Equal(at(5250)) {
		t.Errorf("after a reset: due=%v", d.due())
	}
}

func TestBackoff(t *testing.T) {
	b := newBackoff(time.Second, 30*time.Second)

	var got []time.Duration
	for range 7 {
		got = append(got, b.next())
	}

	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second,
		30 * time.Second, 30 * time.Second,
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected delays (-want +got):\n%s", diff)
	}

	b.reset()

	if got := b.next(); got != time.Second {
		t.Errorf("delay after a reset = %s", got)
	}
}

func TestIsStateEvent(t *testing.T) {
	tests := map[string]bool{
		"start":                     true,
		"die":                       true,
		"destroy":                   true,
		"create":                    true,
		"health_status: healthy":    true,
		"rename":                    true,
		"exec_create: sh -c health": false,
		"exec_start: sh -c health":  false,
		"exec_die":                  false,
		"EXEC_START: bash":          false,
		"":                          true,
	}

	for action, want := range tests {
		ev := clabruntime.ContainerEvent{Type: clabruntime.EventTypeContainer, Action: action}
		if got := isStateEvent(&ev); got != want {
			t.Errorf("isStateEvent(%q) = %v, want %v", action, got, want)
		}
	}
}

// pendingReload reports whether a reload was requested and takes the request.
func (b *testBackend) pendingReload() bool {
	select {
	case <-b.reloadCh:
		return true
	default:
		return false
	}
}

func TestConsumeEvents(t *testing.T) {
	b := newTestBackend(t)

	events := make(chan clabruntime.ContainerEvent, 4)
	errs := make(chan error, 1)

	// The events of commands run in containers, as health checks cause them all the
	// time, do not stand for a change. The stream then ends with an error, the way the
	// Docker runtime ends it.
	events <- clabruntime.ContainerEvent{Action: "exec_start: sh -c health"}

	events <- clabruntime.ContainerEvent{Action: "exec_die"}

	errs <- errors.New("unexpected EOF")

	close(errs)
	close(events)

	err := b.consumeEvents(context.Background(), events, errs)
	if err == nil || err.Error() != "unexpected EOF" {
		t.Errorf("err = %v, want the stream error", err)
	}

	if b.pendingReload() {
		t.Error("the events of a command asked for a reload")
	}

	events = make(chan clabruntime.ContainerEvent, 4)
	errs = make(chan error)

	events <- clabruntime.ContainerEvent{Action: "start"}

	events <- clabruntime.ContainerEvent{Action: "die"}

	close(events)
	close(errs)

	if err := b.consumeEvents(context.Background(), events, errs); err == nil {
		t.Error("a stream that just ended reported no reason")
	}

	if !b.pendingReload() {
		t.Error("the start of a container asked for no reload")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	open := make(chan clabruntime.ContainerEvent)
	if err := b.consumeEvents(ctx, open, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the context error", err)
	}
}

func TestStreamEventsFallsBackToPolling(t *testing.T) {
	b, rt := newRuntimeBackend(t, func(_ *Options, d *deps) { d.rtName = "podman" })

	// A runtime that cannot stream is asked once, not over and over.
	rt.EXPECT().StreamEvents(gomock.Any(), gomock.Any()).
		Return(nil, nil, errors.New("StreamEvents is not implemented for Podman runtime")).Times(1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		b.streamEvents(ctx)
	}()

	eventually(t, "the fallback to be reported", func() bool {
		for _, msg := range b.Messages() {
			if strings.Contains(msg.Text, "polling instead") {
				return true
			}
		}

		return false
	})

	cancel()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("polling did not stop")
	}
}

// liveRuntime mocks a runtime whose containers change during a test.
type liveRuntime struct {
	mu         sync.Mutex
	containers []clabruntime.GenericContainer
	lists      int
	events     chan clabruntime.ContainerEvent
	errs       chan error
}

func (r *liveRuntime) set(containers ...clabruntime.GenericContainer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.containers = containers
}

func (r *liveRuntime) listed() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lists
}

func (r *liveRuntime) list(
	context.Context, []*clabtypes.GenericFilter,
) ([]clabruntime.GenericContainer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lists++

	return append([]clabruntime.GenericContainer(nil), r.containers...), nil
}

func TestStartKeepsTheInventoryCurrent(t *testing.T) {
	dir := t.TempDir()
	topo := filepath.Join(dir, "lab1.clab.yml")
	writeFile(t, topo, "name: lab1\ntopology:\n  nodes: {n1: {}, n2: {}}\n")

	// The scan for topology files looks into the home directory.
	t.Setenv("HOME", t.TempDir())

	live := &liveRuntime{
		events: make(chan clabruntime.ContainerEvent, 8),
		errs:   make(chan error, 1),
	}
	live.set(nodeContainer("lab1", "n1", topo))

	// Without privileges no interface state stream is started.
	b, rt := newRuntimeBackend(t, func(_ *Options, d *deps) {
		d.priv = privilege{reason: reasonNeedsRoot}
	})

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(live.list).AnyTimes()
	rt.EXPECT().StreamEvents(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, opts clabruntime.EventStreamOptions,
		) (<-chan clabruntime.ContainerEvent, <-chan error, error) {
			want := map[string]string{clabconstants.Containerlab: ""}
			if diff := cmp.Diff(want, opts.Labels); diff != "" {
				t.Errorf("unexpected event filter (-want +got):\n%s", diff)
			}

			return live.events, live.errs, nil
		}).AnyTimes()

	if err := b.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := b.Start(context.Background()); err == nil {
		t.Error("a second Start did not fail")
	}

	// The inventory is there when Start returns.
	labs := b.Labs()
	if len(labs) == 0 || labs[0].Name != "lab1" || len(labs[0].Nodes) != 1 {
		t.Fatalf("unexpected labs after Start: %+v", labs)
	}

	// The rest follows in the background: the version, and the topology files, which
	// tell how many nodes the lab defines.
	eventually(t, "the version", func() bool { return b.Host().Version == "0.0.0" })
	eventually(t, "the topology files", func() bool { return b.Labs()[0].DefinedNodes == 2 })
	eventually(t, "the first reload after connecting", func() bool { return live.listed() >= 2 })

	// A container starts: the runtime tells, the inventory follows.
	live.set(nodeContainer("lab1", "n1", topo), nodeContainer("lab1", "n2", topo))

	live.events <- clabruntime.ContainerEvent{Type: "container", Action: "start"}

	eventually(t, "the new node", func() bool { return len(b.Labs()[0].Nodes) == 2 })

	// Events of commands run in containers are not worth a reload.
	lists := live.listed()
	live.events <- clabruntime.ContainerEvent{Type: "container", Action: "exec_start: true"}

	time.Sleep(2 * reloadQuiet)

	if got := live.listed(); got != lists {
		t.Errorf("an exec event caused %d reloads", got-lists)
	}

	// A refresh reloads at once, also without any event.
	live.set(nodeContainer("lab1", "n1", topo))
	b.Refresh()

	eventually(t, "the refreshed inventory", func() bool { return len(b.Labs()[0].Nodes) == 1 })

	b.Close()
	b.Close()

	// Nothing is watched after Close.
	lists = live.listed()

	select {
	case live.events <- clabruntime.ContainerEvent{Type: "container", Action: "die"}:
	default:
	}

	time.Sleep(2 * reloadQuiet)

	if got := live.listed(); got != lists {
		t.Errorf("%d reloads after Close", got-lists)
	}
}

func TestStartStopsWithItsContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	b, rt := newRuntimeBackend(t)
	events := make(chan clabruntime.ContainerEvent)

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	rt.EXPECT().StreamEvents(gomock.Any(), gomock.Any()).Return(events, nil, nil).AnyTimes()

	ctx, cancel := context.WithCancel(context.Background())

	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// With privileges the stream of interface states is started.
	stream := b.run.proc(t, 0)
	if diff := cmp.Diff(
		argv("events -r docker --format json --initial-state"),
		stream.spec.args,
	); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	eventually(t, "the link state capability", func() bool { return b.Host().Caps.LinkState })

	stream.onSignal = func(syscall.Signal) { stream.exitWith(0) }

	cancel()

	eventually(t, "the backend to stop", func() bool { return b.ctx.Err() != nil })
	eventually(t, "the stream to be stopped", func() bool { return len(stream.received()) == 1 })
}

func TestRuntimeEventStreamReconnects(t *testing.T) {
	b, rt := newRuntimeBackend(t)

	first := make(chan clabruntime.ContainerEvent)
	firstErrs := make(chan error, 1)
	second := make(chan clabruntime.ContainerEvent)

	gomock.InOrder(
		rt.EXPECT().StreamEvents(gomock.Any(), gomock.Any()).Return(first, firstErrs, nil),
		rt.EXPECT().StreamEvents(gomock.Any(), gomock.Any()).Return(second, nil, nil),
	)

	go b.streamEvents(t.Context())

	eventually(t, "a reload after connecting", b.pendingReload)

	// The daemon goes away, ie. is restarted.
	firstErrs <- errors.New("unexpected EOF")

	close(firstErrs)
	close(first)

	// After a pause the stream is opened again, and the inventory reloaded as whatever
	// happened in between went unnoticed.
	eventually(t, "a reload after reconnecting", b.pendingReload)

	found := false

	for _, msg := range b.Messages() {
		if msg.Level == levelWarn &&
			strings.Contains(msg.Text, "reconnecting in 1s: unexpected EOF") {
			found = true
		}
	}

	if !found {
		t.Errorf("the lost stream was not reported: %+v", b.Messages())
	}

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicMessages) {
		t.Errorf("the message signaled %b", topics)
	}
}

func TestReloadFailuresAreReportedOnce(t *testing.T) {
	b, rt := newRuntimeBackend(t)
	down := errors.New("cannot connect to the Docker daemon")

	gomock.InOrder(
		rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(nil, down).Times(3),
		rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2),
	)

	for range 5 {
		b.reloadLogged(context.Background())
	}

	logged := b.Messages()
	got := make([]string, 0, len(logged))

	for _, msg := range logged {
		got = append(got, msg.Level+" "+msg.Text)
	}

	want := []string{
		"ERRO failed to list the lab containers: could not list containers: " +
			"cannot connect to the Docker daemon",
		"INFO listing the lab containers works again",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected messages (-want +got):\n%s", diff)
	}
}
