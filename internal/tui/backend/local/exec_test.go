// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"go.uber.org/mock/gomock"
)

// withRuntime makes the containers belong to the given runtime, as listed ones do.
func withRuntime(
	rt clabruntime.ContainerRuntime,
	containers []clabruntime.GenericContainer,
) []clabruntime.GenericContainer {
	for i := range containers {
		containers[i].SetRuntime(rt)
	}

	return containers
}

func TestExec(t *testing.T) {
	b, rt := newRuntimeBackend(t)
	b.setLabs(withRuntime(rt, fabricContainers(t))...)

	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, name string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			if diff := cmp.Diff(
				[]string{"sh", "-c", "echo 'hello world'"},
				cmd.GetCmd(),
			); diff != "" {
				t.Errorf("unexpected command (-want +got):\n%s", diff)
			}

			if name == "clab-fabric-leaf2" {
				return nil, errors.New("container is not running")
			}

			result := clabexec.NewExecResult(cmd)
			result.SetStdOut([]byte("hello from " + name + "\n\x1b[32mgreen\x1b[0m\n"))

			if name == "clab-fabric-spine1" {
				result.SetStdErr([]byte("sh: warning\n"))
				result.SetReturnCode(3)
			}

			return result, nil
		}).Times(3)

	results, err := b.Exec(context.Background(), []clabinternaltuibackend.NodeRef{
		{Lab: "fabric", Name: "leaf1"},
		{Container: "clab-fabric-leaf2"},
		{Lab: "fabric", Name: "spine1"},
		{Lab: "fabric", Name: "nope"},
	}, `sh -c "echo 'hello world'"`)
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]clabinternaltuibackend.ExecResult{}
	for result := range results {
		got[result.Node.Name] = result
	}

	if len(got) != 4 {
		t.Fatalf("%d results, want 4: %+v", len(got), got)
	}

	leaf1 := got["leaf1"]
	if leaf1.Stdout != "hello from clab-fabric-leaf1\ngreen\n" || leaf1.Stderr != "" ||
		leaf1.ExitCode != 0 || leaf1.Err != nil || leaf1.Node.Container != "clab-fabric-leaf1" {
		t.Errorf("unexpected result of leaf1: %+v", leaf1)
	}

	// The reference is completed from the inventory.
	if leaf2 := got["leaf2"]; leaf2.Err == nil || leaf2.Node.Lab != "fabric" {
		t.Errorf("unexpected result of leaf2: %+v", leaf2)
	}

	if spine := got["spine1"]; spine.ExitCode != 3 || spine.Stderr != "sh: warning\n" {
		t.Errorf("unexpected result of spine1: %+v", spine)
	}

	if nope := got["nope"]; nope.Err == nil || !strings.Contains(nope.Err.Error(), "not found") {
		t.Errorf("unexpected result of an unknown node: %+v", nope)
	}
}

func TestExecRejectsBrokenCommands(t *testing.T) {
	b := newTestBackend(t)

	for _, command := range []string{"", "   ", `echo "unterminated`} {
		if _, err := b.Exec(context.Background(), nil, command); err == nil {
			t.Errorf("command %q was accepted", command)
		}
	}
}

func TestExecRunsAFewNodesAtATime(t *testing.T) {
	const nodes = 3 * execWorkers

	b, rt := newRuntimeBackend(t)

	containers := make([]clabruntime.GenericContainer, 0, nodes)
	refs := make([]clabinternaltuibackend.NodeRef, 0, nodes)

	for i := range nodes {
		name := fmt.Sprintf("n%d", i)
		containers = append(containers, nodeContainer("big", name, "/labs/big.clab.yml"))
		refs = append(refs, clabinternaltuibackend.NodeRef{Lab: "big", Name: name})
	}

	b.setLabs(withRuntime(rt, containers)...)

	var running, peak atomic.Int32

	release := make(chan struct{})

	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			now := running.Add(1)
			defer running.Add(-1)

			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}

			select {
			case <-release:
				return clabexec.NewExecResult(cmd), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}).AnyTimes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := b.Exec(ctx, refs, "true")
	if err != nil {
		t.Fatal(err)
	}

	eventually(t, "the workers to be busy", func() bool { return running.Load() == execWorkers })

	// Let a few finish, then give up on the rest.
	for range execWorkers {
		release <- struct{}{}
	}

	done := 0

	for range execWorkers {
		if result := <-results; result.Err == nil {
			done++
		}
	}

	cancel()

	canceled := 0

	for result := range results {
		if errors.Is(result.Err, context.Canceled) {
			canceled++
		}
	}

	if done != execWorkers || canceled != nodes-execWorkers {
		t.Errorf("%d results and %d cancellations, want %d and %d", done, canceled,
			execWorkers, nodes-execWorkers)
	}

	if got := peak.Load(); got != execWorkers {
		t.Errorf("up to %d commands ran at the same time, want %d", got, execWorkers)
	}
}

func TestCleanOutput(t *testing.T) {
	if got := cleanOutput(
		"one\r\ntwo\x1b[1m bold\x1b[0m\n\tthree\n",
	); got != "one\ntwo bold\n        three\n" {
		t.Errorf("got %q", got)
	}

	// Long output is cut, at a character boundary.
	long := strings.Repeat("x", maxExecOutput-1) + "é" + strings.Repeat("y", 100)

	got := cleanOutput(long)
	if !strings.HasSuffix(got, "x"+truncationNote) ||
		len(got) != maxExecOutput-1+len(truncationNote) {
		t.Errorf("output of %d bytes ending in %q", len(got), got[len(got)-30:])
	}

	exact := strings.Repeat("x", maxExecOutput)
	if cleanOutput(exact) != exact {
		t.Error("output at the limit was changed")
	}
}

// logRuntime is a container runtime that can write container logs.
type logRuntime struct {
	*clabmocksmockruntime.MockContainerRuntime

	write func(
		ctx context.Context, name string, opts clabruntime.ContainerLogOptions,
		stdout, stderr io.Writer,
	) error
}

func (r *logRuntime) WriteContainerLogs(
	ctx context.Context,
	name string,
	opts clabruntime.ContainerLogOptions,
	stdout, stderr io.Writer,
) error {
	return r.write(ctx, name, opts, stdout, stderr)
}

// newLogBackend returns a backend whose runtime writes container logs with the function.
func newLogBackend(t *testing.T, rt *logRuntime) *testBackend {
	t.Helper()

	rt.MockContainerRuntime = clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.rt = rt
		d.clab = &clabcore.CLab{Runtimes: map[string]clabruntime.ContainerRuntime{testRuntime: rt}}
	})
	b.setLabs(fabricContainers(t)...)

	return b
}

// collectLogs reads a log stream to its end.
func collectLogs(
	t *testing.T,
	stream clabinternaltuibackend.LogStream,
) []clabinternaltuibackend.LogLine {
	t.Helper()

	var lines []clabinternaltuibackend.LogLine

	timeout := time.After(testTimeout)

	for {
		select {
		case batch, ok := <-stream.Lines():
			if !ok {
				return lines
			}

			lines = append(lines, batch...)
		case <-timeout:
			t.Fatal("the log stream did not end")
		}
	}
}

func TestLogs(t *testing.T) {
	var gotOpts clabruntime.ContainerLogOptions

	b := newLogBackend(t, &logRuntime{write: func(
		_ context.Context, name string, opts clabruntime.ContainerLogOptions,
		stdout, stderr io.Writer,
	) error {
		if name != "clab-fabric-leaf1" {
			t.Errorf("logs of %q were asked for", name)
		}

		gotOpts = opts

		fmt.Fprint(stdout, "2026-03-02T10:00:00.123456789Z [INFO ] chassis ready\r\n")
		fmt.Fprint(stderr, "2026-03-02T10:00:01Z \x1b[31mlevel=error msg=\"link down\"\x1b[0m\n")
		fmt.Fprint(stdout, "no timestamp here\n2026-03-02T10:00:02+01:00 last line, no line break")

		return nil
	}})

	stream, err := b.Logs(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
		clabinternaltuibackend.LogOptions{Tail: 100, Timestamps: true})
	if err != nil {
		t.Fatal(err)
	}

	want := []clabinternaltuibackend.LogLine{
		{
			Time:  time.Date(2026, 3, 2, 10, 0, 0, 123456789, time.UTC),
			Level: levelInfo, Text: "[INFO ] chassis ready",
		},
		{
			Time:   time.Date(2026, 3, 2, 10, 0, 1, 0, time.UTC),
			Stderr: true, Level: levelError, Text: `level=error msg="link down"`,
		},
		{Text: "no timestamp here"},
		{Time: time.Date(2026, 3, 2, 9, 0, 2, 0, time.UTC), Text: "last line, no line break"},
	}

	if diff := cmp.Diff(want, collectLogs(t, stream)); diff != "" {
		t.Errorf("unexpected lines (-want +got):\n%s", diff)
	}

	if err := stream.Err(); err != nil {
		t.Errorf("Err() = %v", err)
	}

	if gotOpts.Tail == nil || *gotOpts.Tail != 100 || gotOpts.Follow || !gotOpts.Timestamps {
		t.Errorf("unexpected options: %+v", gotOpts)
	}

	stream.Close()
}

func TestLogsWithoutTimestamps(t *testing.T) {
	var gotOpts clabruntime.ContainerLogOptions

	b := newLogBackend(t, &logRuntime{write: func(
		_ context.Context, _ string, opts clabruntime.ContainerLogOptions, stdout, _ io.Writer,
	) error {
		gotOpts = opts

		// Without timestamps a line that starts with one is taken as it is.
		fmt.Fprint(stdout, "2026-03-02T10:00:00Z is part of the text\n")

		return errors.New("connection reset")
	}})

	stream, err := b.Logs(context.Background(),
		clabinternaltuibackend.NodeRef{Container: "clab-fabric-leaf1"},
		clabinternaltuibackend.LogOptions{Tail: -1, Follow: true})
	if err != nil {
		t.Fatal(err)
	}

	lines := collectLogs(t, stream)
	if len(lines) != 1 || lines[0].Text != "2026-03-02T10:00:00Z is part of the text" ||
		!lines[0].Time.IsZero() {
		t.Errorf("unexpected lines: %+v", lines)
	}

	// The whole log was asked for, and the reason the stream ended is known.
	if gotOpts.Tail != nil || !gotOpts.Follow || gotOpts.Timestamps {
		t.Errorf("unexpected options: %+v", gotOpts)
	}

	if err := stream.Err(); err == nil || err.Error() != "connection reset" {
		t.Errorf("Err() = %v", err)
	}
}

func TestLogsAreBatched(t *testing.T) {
	const lines = 3*logBatchLines + 10

	b := newLogBackend(t, &logRuntime{write: func(
		_ context.Context, _ string, _ clabruntime.ContainerLogOptions, stdout, _ io.Writer,
	) error {
		for i := range lines {
			fmt.Fprintf(stdout, "line %d\n", i)
		}

		return nil
	}})

	stream, err := b.Logs(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
		clabinternaltuibackend.LogOptions{})
	if err != nil {
		t.Fatal(err)
	}

	total, batches := 0, 0

	for batch := range stream.Lines() {
		if len(batch) == 0 || len(batch) > logBatchLines {
			t.Errorf("a batch of %d lines", len(batch))
		}

		if batch[0].Text != fmt.Sprintf("line %d", total) {
			t.Errorf("batch %d starts with %q", batches, batch[0].Text)
		}

		total += len(batch)
		batches++
	}

	if total != lines || batches < 4 || batches > lines/2 {
		t.Errorf("%d lines in %d batches, want %d lines in a few batches", total, batches, lines)
	}
}

func TestLogsFollowUntilClosed(t *testing.T) {
	var (
		mu      sync.Mutex
		stopped bool
	)

	b := newLogBackend(t, &logRuntime{write: func(
		ctx context.Context, _ string, _ clabruntime.ContainerLogOptions, stdout, _ io.Writer,
	) error {
		fmt.Fprint(stdout, "first\n")

		<-ctx.Done()

		// What is written after the reader left goes nowhere and does not block.
		for range 10 * logBacklog {
			fmt.Fprint(stdout, "late\n")
		}

		mu.Lock()
		stopped = true
		mu.Unlock()

		return ctx.Err()
	}})

	stream, err := b.Logs(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
		clabinternaltuibackend.LogOptions{Follow: true})
	if err != nil {
		t.Fatal(err)
	}

	// A single line is delivered without waiting for a batch to fill up.
	select {
	case batch := <-stream.Lines():
		if len(batch) != 1 || batch[0].Text != "first" {
			t.Errorf("unexpected batch: %+v", batch)
		}
	case <-time.After(testTimeout):
		t.Fatal("no line arrived")
	}

	stream.Close()
	collectLogs(t, stream)

	mu.Lock()
	defer mu.Unlock()

	if !stopped {
		t.Error("the stream was closed before the runtime stopped writing")
	}

	if err := stream.Err(); err != nil {
		t.Errorf("Err() = %v after the reader closed the stream", err)
	}
}

func TestLogsThatCannotBeRead(t *testing.T) {
	// A runtime without the capability.
	plain, _ := newRuntimeBackend(t)
	plain.setLabs(fabricContainers(t)...)

	_, err := plain.Logs(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
		clabinternaltuibackend.LogOptions{})
	if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}

	b := newLogBackend(t, &logRuntime{})

	_, err = b.Logs(context.Background(), clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "x"},
		clabinternaltuibackend.LogOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want the node not to be found", err)
	}
}

func TestConnectMethods(t *testing.T) {
	registry, err := clabcore.NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}

	b := newTestBackend(t, func(_ *Options, d *deps) { d.clab = registry })

	labels := func(node *clabinternaltuibackend.Node) []string {
		methods := b.ConnectMethods(node)
		out := make([]string, 0, len(methods))

		for _, method := range methods {
			out = append(out, method.Label)
		}

		return out
	}

	tests := map[string][]string{
		"nokia_srlinux": {
			"/opt/srlinux/bin/sr_cli", "ssh", "/bin/bash", "/bin/bash -l", "/bin/sh", "/bin/ash",
		},
		"arista_ceos": {
			"/usr/bin/Cli -p 15", "ssh", "/bin/bash", "/bin/bash -l", "/bin/sh", "/bin/ash",
		},
		// The default shell is not listed twice.
		"linux":        {"/bin/sh", "ssh", "/bin/bash", "/bin/bash -l", "/bin/ash"},
		"":             {"/bin/sh", "ssh", "/bin/bash", "/bin/bash -l", "/bin/ash"},
		"no_such_kind": {"/bin/sh", "ssh", "/bin/bash", "/bin/bash -l", "/bin/ash"},
	}

	for kind, want := range tests {
		// Asking twice gives the same answer: the kinds are only looked up once.
		for range 2 {
			if diff := cmp.Diff(
				want,
				labels(&clabinternaltuibackend.Node{Kind: kind}),
			); diff != "" {
				t.Errorf("kind %q: unexpected methods (-want +got):\n%s", kind, diff)
			}
		}
	}

	methods := b.ConnectMethods(&clabinternaltuibackend.Node{Kind: "arista_ceos"})
	if diff := cmp.Diff([]string{"/usr/bin/Cli", "-p", "15"}, methods[0].Shell); diff != "" {
		t.Errorf("unexpected default shell (-want +got):\n%s", diff)
	}

	if !methods[1].SSH || methods[0].SSH ||
		!strings.Contains(methods[0].Description, "arista_ceos") {
		t.Errorf("unexpected methods: %+v", methods[:2])
	}

	// What is handed out is a copy.
	methods[0].Shell[0] = "changed"

	if again := b.ConnectMethods(
		&clabinternaltuibackend.Node{Kind: "arista_ceos"},
	); again[0].Shell[0] != "/usr/bin/Cli" {
		t.Error("a change of a returned shell reached the backend")
	}

	// A backend without a kind registry still offers the generic methods.
	bare := newTestBackend(t)
	if got := bare.ConnectMethods(
		&clabinternaltuibackend.Node{Kind: "nokia_srlinux"},
	); got[0].Label != "/bin/sh" {
		t.Errorf("unexpected methods without a registry: %+v", got)
	}
}

func TestInteractive(t *testing.T) {
	tests := map[string]struct {
		runtime string
		ref     clabinternaltuibackend.NodeRef
		method  clabinternaltuibackend.ConnectMethod
		want    string
	}{
		"shell through docker": {
			runtime: "docker",
			ref:     clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
			method: clabinternaltuibackend.ConnectMethod{
				Shell: []string{"/usr/bin/Cli", "-p", "15"},
			},
			want: "/usr/bin/docker exec -it clab-fabric-leaf1 /usr/bin/Cli -p 15",
		},
		"shell through podman": {
			runtime: "podman",
			ref:     clabinternaltuibackend.NodeRef{Container: "clab-fabric-leaf2"},
			method:  clabinternaltuibackend.ConnectMethod{Shell: []string{"/bin/bash", "-l"}},
			want:    "/usr/bin/podman exec -it clab-fabric-leaf2 /bin/bash -l",
		},
		"no shell given": {
			runtime: "docker",
			ref:     clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
			want:    "/usr/bin/docker exec -it clab-fabric-leaf1 /bin/sh",
		},
		"ssh": {
			runtime: "docker",
			ref:     clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
			method: clabinternaltuibackend.ConnectMethod{
				SSH: true, Shell: []string{"ignored"},
			},
			want: "/usr/bin/ssh clab-fabric-leaf1",
		},
		// A container the inventory does not know yet can still be connected to.
		"unlisted container": {
			runtime: "docker",
			ref:     clabinternaltuibackend.NodeRef{Container: "clab-new-n1"},
			want:    "/usr/bin/docker exec -it clab-new-n1 /bin/sh",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t, func(_ *Options, d *deps) { d.rtName = tt.runtime })
			b.setLabs(fabricContainers(t)...)

			session, err := b.Interactive(tt.ref, tt.method)
			if err != nil {
				t.Fatal(err)
			}

			command, ok := session.(*clabinternaltuibackend.CommandSession)
			if !ok {
				t.Fatalf("the session is a %T, not a command", session)
			}

			cmd := command.Cmd

			if got := strings.Join(cmd.Args, " "); got != tt.want || cmd.Path != cmd.Args[0] {
				t.Errorf("got %q (path %q), want %q", got, cmd.Path, tt.want)
			}

			// The caller hands the terminal over, nothing may be connected yet.
			if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
				t.Error("the standard streams of the command are set")
			}
		})
	}

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.lookPath = func(file string) (string, error) {
			return "", errors.New(file + " not found")
		}
	})

	if _, err := b.Interactive(clabinternaltuibackend.NodeRef{Container: "c"},
		clabinternaltuibackend.ConnectMethod{}); err == nil {
		t.Error("a missing runtime client was not reported")
	}

	if _, err := b.Interactive(clabinternaltuibackend.NodeRef{Container: "c"},
		clabinternaltuibackend.ConnectMethod{SSH: true}); err == nil {
		t.Error("a missing ssh client was not reported")
	}

	if _, err := b.Interactive(clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "nope"},
		clabinternaltuibackend.ConnectMethod{}); err == nil {
		t.Error("an unknown node was not reported")
	}

	if !slices.Equal(
		[]string{runtimeClient("docker"), runtimeClient("podman"), runtimeClient("nerdctl")},
		[]string{"docker", "podman", "nerdctl"},
	) {
		t.Error("unexpected runtime clients")
	}
}
