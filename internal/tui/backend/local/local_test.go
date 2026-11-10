// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func TestNewRejectsWhatCannotWork(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "containerlab")

	if _, err := New(Options{Executable: missing}); err == nil ||
		!strings.Contains(err.Error(), "containerlab binary") {
		t.Errorf("a missing binary: err = %v", err)
	}

	if _, err := New(Options{Runtime: "no-such-runtime"}); err == nil ||
		!strings.Contains(err.Error(), "no-such-runtime") {
		t.Errorf("an unknown runtime: err = %v", err)
	}
}

func TestResolveExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	if got, err := resolveExecutable(""); err != nil || got != self {
		t.Errorf("the default is %q, %v, want the binary of this process", got, err)
	}

	// A relative path is made absolute, the commands run in other directories.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "clab"), "")
	t.Chdir(dir)

	got, err := resolveExecutable("clab")
	if err != nil || !filepath.IsAbs(got) || filepath.Base(got) != "clab" {
		t.Errorf("resolveExecutable() = %q, %v", got, err)
	}
}

func TestChangesAreSignaled(t *testing.T) {
	b := newTestBackend(t)

	select {
	case <-b.Notify():
		t.Fatal("a signal without a change")
	default:
	}

	if got := b.Changed(); got != 0 {
		t.Errorf("Changed() = %b without a change", got)
	}

	// Several changes make one signal and add up.
	b.mark(clabinternaltuibackend.TopicStats)
	b.mark(clabinternaltuibackend.TopicLabs)
	b.mark(clabinternaltuibackend.TopicStats)
	b.mark(0)

	select {
	case <-b.Notify():
	default:
		t.Fatal("no signal after a change")
	}

	select {
	case <-b.Notify():
		t.Fatal("more than one signal for changes that were not read yet")
	default:
	}

	want := clabinternaltuibackend.TopicStats | clabinternaltuibackend.TopicLabs
	if got := b.Changed(); got != want {
		t.Errorf("Changed() = %b, want %b", got, want)
	}

	if got := b.Changed(); got != 0 {
		t.Errorf("Changed() = %b after the changes were read", got)
	}
}

func TestMessages(t *testing.T) {
	b := newTestBackend(t)

	b.logf(levelWarn, "first %s\nsecond \x1b[31mline\x1b[0m", "line")

	messages := b.Messages()
	if len(messages) != 1 || messages[0].Level != levelWarn ||
		messages[0].Text != "first line second line" || !messages[0].Time.Equal(b.clock.Now()) {
		t.Errorf("unexpected messages: %+v", messages)
	}

	if topics := b.drainTopics(); topics != clabinternaltuibackend.TopicMessages {
		t.Errorf("a message signaled %b", topics)
	}

	// Only the most recent messages are kept.
	for i := range maxMessages + 20 {
		b.logf(levelInfo, "message %d", i)
	}

	messages = b.Messages()
	if len(messages) != maxMessages || messages[0].Text != "message 20" ||
		messages[maxMessages-1].Text != fmt.Sprintf("message %d", maxMessages+19) {
		t.Errorf("%d messages from %q to %q", len(messages), messages[0].Text,
			messages[len(messages)-1].Text)
	}
}

func TestPanicsAreReported(t *testing.T) {
	b := newTestBackend(t)

	if ok := b.guard("fine", func() {}); !ok {
		t.Error("a function that returned was reported as panicked")
	}

	if ok := b.guard("broken", func() { panic("boom") }); ok {
		t.Error("a panic went unnoticed")
	}

	done := make(chan struct{})

	b.spawn("worker", func() {
		defer close(done)

		panic(fmt.Errorf("worker failed after %d items", 3))
	})

	<-done

	eventually(t, "the panics to be reported", func() bool {
		var reported []string

		for _, msg := range b.Messages() {
			if msg.Level == levelError {
				reported = append(reported, msg.Text)
			}
		}

		return len(reported) == 2 && reported[0] == "panic in broken: boom" &&
			reported[1] == "panic in worker: worker failed after 3 items"
	})

	// Where it happened is told as well.
	found := false

	for _, msg := range b.Messages() {
		if msg.Level == levelDebug && strings.Contains(msg.Text, "local_test.go") {
			found = true
		}
	}

	if !found {
		t.Errorf("no stack trace in the messages: %+v", b.Messages())
	}
}

func TestLoopRestartsAfterAPanic(t *testing.T) {
	b := newTestBackend(t)

	var runs atomic.Int32

	b.loop("flaky loop", func(ctx context.Context) {
		if runs.Add(1) == 1 {
			panic("first run fails")
		}

		<-ctx.Done()
	})

	eventually(t, "the loop to run again", func() bool { return runs.Load() == 2 })

	// A loop that returns is not started again.
	b.loop("short loop", func(context.Context) { runs.Add(10) })

	eventually(t, "the short loop to run", func() bool { return runs.Load() == 12 })
	time.Sleep(10 * time.Millisecond)

	if got := runs.Load(); got != 12 {
		t.Errorf("the loops ran %d times in total, want 12", got)
	}
}

func TestHost(t *testing.T) {
	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.getenv = func(key string) string {
			if key == "SUDO_USER" {
				return "alice"
			}

			return ""
		}
	})

	host := b.Host()
	if host.Runtime != testRuntime || host.Hostname == "" || host.User != "alice" ||
		!host.Privileged || host.Demo || host.Version != "" {
		t.Errorf("unexpected host: %+v", host)
	}

	// The version is the one of the binary that runs the commands, asked in the background.
	b.lookupVersion()

	if got := b.Host().Version; got != "0.0.0" {
		t.Errorf("Version = %q", got)
	}

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicHost) {
		t.Errorf("the version signaled %b", topics)
	}

	if user := newTestBackend(t).Host().User; user == "" {
		t.Error("no user without sudo")
	}
}

func TestVersionOfTheBinary(t *testing.T) {
	// The test binary stands in for containerlab, see TestMain.
	b := newProcessBackend(t, "version")

	got, err := b.executableVersion(context.Background())
	if err != nil || got != "0.83.0" {
		t.Errorf("executableVersion() = %q, %v", got, err)
	}

	b = newProcessBackend(t, "fail")
	if _, err := b.executableVersion(context.Background()); err == nil {
		t.Error("a failing version command was not reported")
	}
}

// twoRunningLabs sets up two labs with a running node each and the files a sampler reads
// for them.
func twoRunningLabs(t *testing.T, b *testBackend) {
	t.Helper()

	running := nodeContainer("lab2", "n1", "/labs/lab2.clab.yml")
	running.Pid = 5000

	stopped := nodeContainer("lab2", "n2", "/labs/lab2.clab.yml")
	stopped.State = "exited"
	stopped.Pid = 0

	b.setLabs(nodeContainer("lab1", "n1", "/labs/lab1.clab.yml"), running, stopped)

	for _, pid := range []int{4242, 5000} {
		c := newFakeContainer(t, b.stats.root)
		c.pid = pid
		c.setNetDev(netDevLine("eth0", 1000, 10, 0, 0))
	}
}

func TestSamplingFollowsTheFocus(t *testing.T) {
	b := newTestBackend(t)
	sampledAt := func(container string) time.Time {
		b.stats.mu.RLock()
		defer b.stats.mu.RUnlock()

		if node := b.stats.nodes[container]; node != nil {
			return node.netAt
		}

		return time.Time{}
	}

	// Nothing is deployed: there is nothing to sample and nothing to tell.
	b.sampleOnce(true)

	if topics := b.drainTopics(); topics != 0 {
		t.Errorf("sampling without labs signaled %b", topics)
	}

	twoRunningLabs(t, b)

	// A full round samples every running container.
	start := b.clock.Now()
	b.sampleOnce(true)

	if !sampledAt("clab-lab1-n1").Equal(start) || !sampledAt("clab-lab2-n1").Equal(start) ||
		!sampledAt("clab-lab2-n2").IsZero() {
		t.Error("a full round did not sample exactly the running containers")
	}

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicStats) ||
		!topics.Has(clabinternaltuibackend.TopicInterfaces) {
		t.Errorf("the first samples signaled %b", topics)
	}

	// The rounds in between only happen for a lab whose rates are on screen.
	b.clock.advance(time.Second)
	b.sampleOnce(false)

	b.SetFocus(clabinternaltuibackend.Focus{Lab: "lab2"})
	b.sampleOnce(false)

	b.SetFocus(clabinternaltuibackend.Focus{Rates: true})
	b.sampleOnce(false)

	if !sampledAt("clab-lab1-n1").Equal(start) || !sampledAt("clab-lab2-n1").Equal(start) {
		t.Error("a round in between sampled without rates being shown for a lab")
	}

	b.SetFocus(clabinternaltuibackend.Focus{Lab: "lab2", Rates: true})
	b.sampleOnce(false)

	if !sampledAt("clab-lab1-n1").Equal(start) ||
		!sampledAt("clab-lab2-n1").Equal(start.Add(time.Second)) {
		t.Error("a round in between did not sample exactly the lab in focus")
	}

	// Later samples only tell about the values.
	if topics := b.drainTopics(); topics != clabinternaltuibackend.TopicStats {
		t.Errorf("a later sample signaled %b", topics)
	}

	labs := b.Labs()
	if got := labs[1].Nodes[0]; got.Name != "n1" || got.Lab != "lab2" || len(got.RxSeries) != 1 {
		t.Errorf("the sampled values are not part of the labs: %+v", got)
	}
}
