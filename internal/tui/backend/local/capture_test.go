// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// wiresharkCapture starts a capture of the fabric lab into Wireshark and returns the job
// with the capture process.
func wiresharkCapture(t *testing.T, b *testBackend) (clabinternaltuibackend.JobID, *fakeProc) {
	t.Helper()

	b.setLabs(fabricContainers(t)...)

	id, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab: "fabric",
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1", "e1-2"}},
		},
		Wireshark: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	return id, b.run.proc(t, 0)
}

// openWireshark lets the time pass after which Wireshark is opened and returns its process.
func openWireshark(t *testing.T, b *testBackend) *fakeProc {
	t.Helper()

	b.clock.awaitTimer(t)
	b.clock.advance(wiresharkDelay)

	return b.run.proc(t, 1)
}

func TestCaptureIntoWireshark(t *testing.T) {
	b := newTestBackend(t)
	id, capture := wiresharkCapture(t, b)
	topo := absTestdata(t, "fabric.clab.yml")

	wantArgs := argv("capture -r docker -t " + topo + " --target leaf1:e1-1,e1-2 -w -")
	if diff := cmp.Diff(wantArgs, capture.spec.args); diff != "" {
		t.Errorf("unexpected capture arguments (-want +got):\n%s", diff)
	}

	info, _ := b.Job(id)
	wantCommand := "containerlab capture -r docker -t " + topo +
		" --target leaf1:e1-1,e1-2 -w - | wireshark -k -i -"

	if info.Command != wantCommand || info.Kind != clabinternaltuibackend.JobCapture ||
		info.Title != "capture leaf1:e1-1,e1-2" {
		t.Errorf("unexpected job: %+v", info)
	}

	// A capture leaves its lab free for other jobs.
	if busy := b.jobs.busyLabs(); busy.has("fabric", topo) {
		t.Error("a capture marks its lab busy")
	}

	// What is captured before Wireshark runs is held back, not lost.
	capture.stdout("pcapng-header ")
	capture.stderr("10:00:00 INFO Capturing interfaces=2\n")

	if b.run.count() != 1 {
		t.Fatal("Wireshark was started before the capture proved to run")
	}

	wireshark := openWireshark(t, b)

	wantSpec := procSpec{
		path: "/usr/bin/wireshark", args: argv("-k -i -"), stdin: true, session: true,
		env: []string{"PATH=/usr/bin", "CLAB_TOPO=x.clab.yml"},
	}
	if diff := cmp.Diff(wantSpec, *wireshark.spec, cmp.AllowUnexported(procSpec{})); diff != "" {
		t.Errorf("unexpected Wireshark process (-want +got):\n%s", diff)
	}

	eventually(t, "the held back data to reach Wireshark", func() bool {
		return wireshark.in.content() == "pcapng-header "
	})

	capture.stdout("packet-1 packet-2")

	if got := wireshark.in.content(); got != "pcapng-header packet-1 packet-2" {
		t.Errorf("Wireshark received %q", got)
	}

	eventually(t, "the captured bytes to be counted", func() bool {
		info, _ = b.Job(id)

		return info.Bytes == uint64(len("pcapng-header packet-1 packet-2"))
	})

	// The binary data is no output of the job.
	if out := b.JobOutput(id, 0); len(out) != 1 || out[0].Text != "Capturing interfaces=2" {
		t.Errorf("unexpected job output: %+v", out)
	}

	// Closing Wireshark ends the capture, which is its regular end.
	capture.onSignal = func(syscall.Signal) {
		capture.stdout("last-packet")
		capture.exitWith(0)
	}

	wireshark.exitWith(0)

	info = b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobSucceeded || info.Err != "" {
		t.Errorf("unexpected job after Wireshark was closed: %+v", info)
	}

	if diff := cmp.Diff([]syscall.Signal{syscall.SIGINT}, capture.received()); diff != "" {
		t.Errorf("unexpected signals to the capture (-want +got):\n%s", diff)
	}

	// The capture could finish writing although nobody reads any more.
	if info.Bytes != uint64(len("pcapng-header packet-1 packet-2last-packet")) {
		t.Errorf("Bytes = %d", info.Bytes)
	}

	if len(wireshark.received()) != 0 {
		t.Errorf("Wireshark was signaled: %v", wireshark.received())
	}
}

func TestCaptureEndsWhileWiresharkIsOpen(t *testing.T) {
	b := newTestBackend(t)
	id, capture := wiresharkCapture(t, b)

	// The capture runs out, ie. by reaching its packet limit.
	wireshark := openWireshark(t, b)

	capture.stdout("all of it")
	capture.exitWith(0)

	if info := b.awaitJob(t, id); info.State != clabinternaltuibackend.JobSucceeded {
		t.Errorf("unexpected job: %+v", info)
	}

	// Wireshark is told that the stream ended and is left open for the user to look at it.
	eventually(t, "the stream to Wireshark to be closed", wireshark.in.isClosed)

	if wireshark.in.content() != "all of it" || len(wireshark.received()) != 0 {
		t.Errorf(
			"Wireshark got %q and the signals %v",
			wireshark.in.content(),
			wireshark.received(),
		)
	}
}

func TestCaptureThatFailsRightAway(t *testing.T) {
	tests := map[string]struct {
		stderr  string
		code    int
		wantErr string
	}{
		"with an error": {
			stderr:  "10:00:00 ERRO node \"leaf9\" not found\n",
			code:    1,
			wantErr: `node "leaf9" not found`,
		},
		"without telling why": {
			wantErr: "the command ended before its output was used",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t)
			id, capture := wiresharkCapture(t, b)

			capture.stderr(tt.stderr)
			capture.exitWith(tt.code)

			info := b.awaitJob(t, id)
			if info.State != clabinternaltuibackend.JobFailed || info.Err != tt.wantErr {
				t.Errorf("unexpected job: %+v", info)
			}

			// No window opens for a capture that is over.
			b.clock.advance(time.Minute)
			time.Sleep(10 * time.Millisecond)

			if b.run.count() != 1 {
				t.Error("Wireshark was started for a capture that had ended")
			}
		})
	}
}

func TestCaptureWhenWiresharkFails(t *testing.T) {
	t.Run("it cannot be started", func(t *testing.T) {
		b := newTestBackend(t)
		b.run.errFor = func(spec *procSpec) error {
			if strings.HasSuffix(spec.path, wiresharkBinary) {
				return errors.New("permission denied")
			}

			return nil
		}

		id, capture := wiresharkCapture(t, b)
		capture.onSignal = func(syscall.Signal) { capture.exitWith(0) }

		b.clock.awaitTimer(t)
		b.clock.advance(wiresharkDelay)

		info := b.awaitJob(t, id)
		if info.State != clabinternaltuibackend.JobFailed ||
			info.Err != "failed to start Wireshark: permission denied" {
			t.Errorf("unexpected job: %+v", info)
		}

		if len(capture.received()) == 0 {
			t.Error("the capture was left running")
		}
	})

	t.Run("it exits with an error", func(t *testing.T) {
		b := newTestBackend(t)
		id, capture := wiresharkCapture(t, b)
		capture.onSignal = func(syscall.Signal) { capture.exitWith(0) }

		wireshark := openWireshark(t, b)
		wireshark.exitWith(1)

		info := b.awaitJob(t, id)
		if info.State != clabinternaltuibackend.JobFailed ||
			info.Err != "Wireshark exited with status 1" {
			t.Errorf("unexpected job: %+v", info)
		}
	})

	t.Run("it stops reading", func(t *testing.T) {
		b := newTestBackend(t)
		id, capture := wiresharkCapture(t, b)
		capture.onSignal = func(syscall.Signal) { capture.exitWith(0) }

		wireshark := openWireshark(t, b)

		capture.stdout("first packet")

		eventually(t, "the capture to reach Wireshark", func() bool {
			return wireshark.in.content() == "first packet"
		})

		wireshark.in.breakPipe()
		capture.stdout("packet nobody reads")

		b.awaitJob(t, id)

		if diff := cmp.Diff([]syscall.Signal{syscall.SIGINT}, capture.received()); diff != "" {
			t.Errorf("unexpected signals to the capture (-want +got):\n%s", diff)
		}

		// Writing more must not fail or block.
		capture.stdout("more")
	})
}

func TestStoppedCaptureIsASuccess(t *testing.T) {
	b := newTestBackend(t)
	id, capture := wiresharkCapture(t, b)
	capture.onSignal = func(syscall.Signal) { capture.exitWith(0) }

	wireshark := openWireshark(t, b)

	b.CancelJob(id)

	// Stopping a capture is how it normally ends: what was captured is complete.
	if info := b.awaitJob(t, id); info.State != clabinternaltuibackend.JobSucceeded {
		t.Errorf("unexpected job: %+v", info)
	}

	eventually(t, "the stream to Wireshark to be closed", wireshark.in.isClosed)
}

func TestCaptureIntoFile(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	topo := absTestdata(t, "fabric.clab.yml")

	// The working directory is reported with symbolic links resolved.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	id, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab:  "fabric",
		File: "out.pcapng",
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1"}},
			{Node: "spine1", Interfaces: []string{"e1-1"}},
		},
		Duration:    30 * time.Second,
		PacketLimit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The command runs next to the topology file, the capture file is named from here.
	file := filepath.Join(dir, "out.pcapng")
	proc := b.run.proc(t, 0)

	want := argv("capture -r docker -t " + topo + " --target leaf1:e1-1 --target spine1:e1-1 " +
		"--duration 30s --packet-limit 100 -w " + file)
	if diff := cmp.Diff(want, proc.spec.args); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	if proc.spec.dir != filepath.Dir(topo) {
		t.Errorf("the command runs in %q", proc.spec.dir)
	}

	writeFile(t, file, strings.Repeat("x", 4096))
	proc.stderr("10:00:00 INFO capture done\n")
	proc.exitWith(0)

	b.awaitJob(t, id)

	eventually(t, "the size of the capture file", func() bool {
		info, _ := b.Job(id)

		return info.Bytes == 4096
	})

	if b.run.count() != 1 {
		t.Error("a capture into a file started a second process")
	}
}

func TestCaptureRequestsThatCannotRun(t *testing.T) {
	target := []clabinternaltuibackend.CaptureTarget{{Node: "leaf1", Interfaces: []string{"e1-1"}}}

	tests := map[string]struct {
		adjust  func(o *Options, d *deps)
		req     clabinternaltuibackend.CaptureRequest
		wantErr string
		// unavailable is set when the environment is why the capture cannot run.
		unavailable bool
	}{
		"no targets": {
			req:     clabinternaltuibackend.CaptureRequest{Lab: "fabric", Wireshark: true},
			wantErr: "no interfaces selected",
		},
		"a target without interfaces": {
			req: clabinternaltuibackend.CaptureRequest{
				Lab: "fabric", Wireshark: true,
				Targets: []clabinternaltuibackend.CaptureTarget{{Node: "leaf1"}},
			},
			wantErr: "at least one interface",
		},
		"neither Wireshark nor a file": {
			req:     clabinternaltuibackend.CaptureRequest{Lab: "fabric", Targets: target},
			wantErr: "no capture file given",
		},
		"an unknown lab": {
			req: clabinternaltuibackend.CaptureRequest{
				Lab: "nope", Targets: target, Wireshark: true,
			},
			wantErr: "no topology file known",
		},
		"Wireshark is not installed": {
			adjust: func(_ *Options, d *deps) {
				d.lookPath = func(string) (string, error) { return "", errors.New("not found") }
			},
			req: clabinternaltuibackend.CaptureRequest{
				Lab: "fabric", Targets: target, Wireshark: true,
			},
			wantErr:     "wireshark is not installed",
			unavailable: true,
		},
		"no display": {
			adjust: func(_ *Options, d *deps) { d.getenv = func(string) string { return "" } },
			req: clabinternaltuibackend.CaptureRequest{
				Lab: "fabric", Targets: target, Wireshark: true,
			},
			wantErr:     "no graphical display",
			unavailable: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var adjust []func(o *Options, d *deps)
			if tt.adjust != nil {
				adjust = append(adjust, tt.adjust)
			}

			b := newTestBackend(t, adjust...)
			b.setLabs(fabricContainers(t)...)

			_, err := b.Capture(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want one about %q", err, tt.wantErr)
			}

			if got := errors.Is(err, clabinternaltuibackend.ErrUnavailable); got != tt.unavailable {
				t.Errorf("errors.Is(err, ErrUnavailable) = %v, want %v", got, tt.unavailable)
			}

			if b.run.count() != 0 {
				t.Error("a process was started for a request that cannot run")
			}
		})
	}
}

func TestCaptureRelay(t *testing.T) {
	newRelay := func() (*captureRelay, *int, *int) {
		counted, lost := 0, 0

		return &captureRelay{
			count: func(n int) { counted += n },
			lost:  func() { lost++ },
		}, &counted, &lost
	}

	t.Run("data is dropped once the receiver is gone", func(t *testing.T) {
		relay, counted, lost := newRelay()
		sink := &fakeStdin{}

		relay.attach(sink)
		_, _ = relay.Write([]byte("abc"))

		sink.breakPipe()

		for range 3 {
			if n, err := relay.Write([]byte("def")); n != 3 || err != nil {
				t.Errorf("Write() = %d, %v, want it to take the data", n, err)
			}
		}

		if sink.content() != "abc" || *counted != 12 || *lost != 1 || !sink.isClosed() {
			t.Errorf("received %q, counted %d, lost %d times", sink.content(), *counted, *lost)
		}
	})

	t.Run("a receiver that never comes does not make the data pile up", func(t *testing.T) {
		relay, _, lost := newRelay()
		chunk := make([]byte, maxRelayBacklog/4)

		for range 6 {
			_, _ = relay.Write(chunk)
		}

		if *lost != 1 || relay.backlog.Len() != 0 {
			t.Errorf("lost %d times with %d bytes held back", *lost, relay.backlog.Len())
		}

		// A receiver that shows up late gets nothing and is closed.
		sink := &fakeStdin{}
		relay.attach(sink)

		if !sink.isClosed() || sink.content() != "" {
			t.Errorf("a late receiver got %q", sink.content())
		}
	})

	t.Run("a receiver that is gone when it is attached", func(t *testing.T) {
		relay, _, lost := newRelay()
		_, _ = relay.Write([]byte("held back"))

		sink := &fakeStdin{}
		sink.breakPipe()
		relay.attach(sink)

		if *lost != 1 || !sink.isClosed() {
			t.Errorf("lost %d times, closed=%v", *lost, sink.isClosed())
		}
	})
}

func TestInvokingUser(t *testing.T) {
	uid, gid := os.Getuid(), os.Getgid()
	env := map[string]string{"SUDO_UID": strconv.Itoa(uid), "SUDO_GID": strconv.Itoa(gid)}
	getenv := func(key string) string { return env[key] }

	// Without root privileges there is nobody else to run a program as.
	if who := invokingUser(uid+1, getenv); who != nil {
		t.Errorf("a user to switch to although the process is not root: %+v", who)
	}

	if who := invokingUser(0, func(string) string { return "" }); who != nil {
		t.Errorf("a user to switch to although sudo was not used: %+v", who)
	}

	who := invokingUser(0, getenv)
	if uid == 0 {
		// Root calling sudo stays root.
		if who != nil {
			t.Errorf("unexpected user for root: %+v", who)
		}

		return
	}

	if who == nil || who.cred.Uid != uint32(uid) || who.cred.Gid != uint32(gid) {
		t.Fatalf("unexpected user: %+v", who)
	}

	account, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		t.Skipf("the user running the tests has no account: %v", err)
	}

	for _, want := range []string{"HOME=" + account.HomeDir, "USER=" + account.Username} {
		if !slices.Contains(who.env, want) {
			t.Errorf("the environment of the user lacks %s: %v", want, who.env)
		}
	}
}
