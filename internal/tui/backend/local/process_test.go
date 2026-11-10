// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	"golang.org/x/sys/unix"
)

// childModeEnv makes the test binary act as a child command instead of running the tests.
// The tests of real processes run the test binary itself that way.
const childModeEnv = "CLAB_TUI_TEST_CHILD"

// childReportEnv names the file a child writes its observations to.
const childReportEnv = "CLAB_TUI_TEST_REPORT"

func TestMain(m *testing.M) {
	if mode := os.Getenv(childModeEnv); mode != "" {
		os.Exit(runChild(mode))
	}

	os.Exit(m.Run())
}

// runChild behaves like a containerlab command, as far as the backend can tell.
func runChild(mode string) int {
	switch mode {
	case "report":
		return childReport()
	case "fail":
		return childFail()
	case "interrupt":
		return childWaitForSignal(false)
	case "stubborn":
		return childWaitForSignal(true)
	case "capture":
		// The same binary plays the capture command and the program its output is piped
		// into. They are told apart by their arguments.
		if len(os.Args) > 1 && os.Args[1] == "capture" {
			return childCapture()
		}

		return childViewer()
	case "version":
		if strings.Join(os.Args[1:], " ") != "version --short" {
			return 2
		}

		fmt.Println("0.83.0")

		return 0
	case "partial":
		// A last line without line break, and data that is no text.
		fmt.Fprint(os.Stdout, "\x00\x01binary\xff")
		fmt.Fprint(os.Stderr, "10:00:00 INFO no line break")

		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)

		return 99
	}
}

// childReport prints what a child sees of its environment.
func childReport() int {
	cwd, _ := os.Getwd()
	stdin, _ := os.Stdin.Stat()
	null, _ := os.Stat(os.DevNull)

	fmt.Println("args=" + strings.Join(os.Args[1:], " "))
	fmt.Println("cwd=" + cwd)
	fmt.Println("PWD=" + os.Getenv("PWD"))
	fmt.Printf("stdin-is-null=%v\n", os.SameFile(stdin, null))
	fmt.Printf("leads-its-process-group=%v\n", syscall.Getpgrp() == os.Getpid())

	for _, name := range []string{"CLAB_TOPO", "CLAB_NAME", "CLAB_VERSION_CHECK", "CLAB_KEPT"} {
		value, set := os.LookupEnv(name)
		fmt.Printf("%s=%s set=%v\n", name, value, set)
	}

	fmt.Fprintln(os.Stderr, "10:00:00 INFO \x1b[1mreported\x1b[0m")

	return 0
}

// childFail prints the output of a command that fails the way containerlab does.
func childFail() int {
	fmt.Fprintln(os.Stderr, "2026/03/02 10:00:00 DEBU early line with a date")
	fmt.Fprintln(os.Stderr, "10:00:01 \x1b[32mINFO\x1b[0m Parsing & checking topology")
	fmt.Fprintln(os.Stderr, "10:00:02 WARN something looks odd")
	fmt.Fprintln(os.Stderr, "10:00:03 ERRO a node failed")
	fmt.Fprintln(os.Stdout, "partial result")
	fmt.Fprintln(os.Stderr, "          ")
	fmt.Fprintln(os.Stderr, "   \x1b[1;41mERROR\x1b[0m  ")
	fmt.Fprintln(os.Stderr, "          ")
	fmt.Fprintln(os.Stderr, "  the deployment failed.          ")
	fmt.Fprintln(os.Stderr)

	return 1
}

// childCapture writes data to its standard output until it is interrupted, and finishes
// the data then.
func childCapture() int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	fmt.Fprint(os.Stdout, "header;")

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-signals:
			fmt.Fprint(os.Stdout, "trailer;")
			fmt.Fprintln(os.Stderr, "10:00:01 INFO capture finished")

			return 0
		case <-ticker.C:
			fmt.Fprint(os.Stdout, "packet;")
		}
	}
}

// childViewer reads what is piped into it until it has seen enough, reports what it saw to
// the file named in its environment and exits, like a user closing the window.
func childViewer() int {
	const enough = 10

	var seen []string

	reader := bufio.NewReader(os.Stdin)

	for len(seen) < enough {
		record, err := reader.ReadString(';')
		if err != nil {
			break
		}

		seen = append(seen, record)
	}

	sid, _ := unix.Getsid(0)
	stdout, _ := os.Stdout.Stat()
	null, _ := os.Stat(os.DevNull)

	report := fmt.Sprintf("args=%s\nfirst=%s\nrecords=%d\nsession-leader=%v\nstdout-is-null=%v\n",
		strings.Join(os.Args[1:], " "), seen[0], len(seen), sid == os.Getpid(),
		os.SameFile(stdout, null))

	if err := os.WriteFile(os.Getenv(childReportEnv), []byte(report), 0o600); err != nil {
		return 1
	}

	return 0
}

// childWaitForSignal waits to be interrupted. A stubborn child ignores everything it can.
func childWaitForSignal(stubborn bool) int {
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	fmt.Fprintln(os.Stderr, "10:00:00 INFO ready")

	for sig := range signals {
		fmt.Fprintf(os.Stderr, "10:00:01 INFO received %s\n", sig)

		if !stubborn {
			break
		}
	}

	fmt.Fprintln(os.Stderr, "10:00:02 INFO cleaned up")

	return 0
}

// newProcessBackend returns a backend that runs the test binary as its child command.
func newProcessBackend(t *testing.T, mode string) *testBackend {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	return newTestBackend(t, func(_ *Options, d *deps) {
		d.exe = self
		d.run = execRunner{}
		d.clock = systemClock{}
		d.environ = func() []string {
			return append(os.Environ(), childModeEnv+"="+mode,
				"CLAB_TOPO=/elsewhere/other.clab.yml", "CLAB_NAME=other", "CLAB_KEPT=yes")
		}
	})
}

func outputTexts(lines []clabinternaltuibackend.LogLine, stderr bool) []string {
	var texts []string

	for _, line := range lines {
		if line.Stderr == stderr {
			texts = append(texts, line.Text)
		}
	}

	return texts
}

func TestProcessEnvironment(t *testing.T) {
	b := newProcessBackend(t, "report")
	req := deployFile(t, "lab1")

	id, err := b.Deploy(req)
	if err != nil {
		t.Fatal(err)
	}

	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobSucceeded {
		t.Fatalf("unexpected job: %+v\n%v", info, b.JobOutput(id, 0))
	}

	dir, err := filepath.EvalSymlinks(filepath.Dir(req.TopoFile))
	if err != nil {
		t.Fatal(err)
	}

	out := b.JobOutput(id, 0)
	want := []string{
		"args=deploy -r docker -t " + req.TopoFile + " --format json",
		"cwd=" + dir,
		"PWD=" + filepath.Dir(req.TopoFile),
		"stdin-is-null=true",
		"leads-its-process-group=true",
		"CLAB_TOPO= set=false",
		"CLAB_NAME= set=false",
		"CLAB_VERSION_CHECK=disable set=true",
		"CLAB_KEPT=yes set=true",
	}

	if diff := cmp.Diff(want, outputTexts(out, false)); diff != "" {
		t.Errorf("unexpected standard output (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"reported"}, outputTexts(out, true)); diff != "" {
		t.Errorf("unexpected error output (-want +got):\n%s", diff)
	}
}

func TestProcessFailure(t *testing.T) {
	b := newProcessBackend(t, "fail")

	j, err := b.jobs.start(&jobSpec{
		kind: clabinternaltuibackend.JobDeploy, lab: "lab1", args: []string{"deploy"},
	})
	if err != nil {
		t.Fatal(err)
	}

	info := b.awaitJob(t, j.info.ID)
	if info.State != clabinternaltuibackend.JobFailed || info.ExitCode != 1 ||
		info.Err != "the deployment failed." || info.Warnings != 1 || info.Errors != 2 {
		t.Errorf("unexpected job: %+v", info)
	}

	type line struct{ level, text string }

	var got []line

	out := b.JobOutput(info.ID, 0)
	for _, l := range out {
		if l.Stderr {
			got = append(got, line{l.Level, l.Text})
		}
	}

	want := []line{
		{levelDebug, "early line with a date"},
		{levelInfo, "Parsing & checking topology"},
		{levelWarn, "something looks odd"},
		{levelError, "a node failed"},
		{levelError, "the deployment failed."},
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(line{})); diff != "" {
		t.Errorf("unexpected error output (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"partial result"}, outputTexts(out, false)); diff != "" {
		t.Errorf("unexpected standard output (-want +got):\n%s", diff)
	}
}

func TestProcessOutputWithoutLineBreak(t *testing.T) {
	b := newProcessBackend(t, "partial")

	j, err := b.jobs.start(&jobSpec{lab: "lab1", args: []string{"save"}})
	if err != nil {
		t.Fatal(err)
	}

	b.awaitJob(t, j.info.ID)

	out := b.JobOutput(j.info.ID, 0)
	if diff := cmp.Diff([]string{"no line break"}, outputTexts(out, true)); diff != "" {
		t.Errorf("unexpected error output (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"binary�"}, outputTexts(out, false)); diff != "" {
		t.Errorf("unexpected standard output (-want +got):\n%s", diff)
	}
}

// awaitOutput blocks until the job printed a line with the given text.
func awaitOutput(t *testing.T, b *testBackend, id clabinternaltuibackend.JobID, text string) {
	t.Helper()

	eventually(t, fmt.Sprintf("the output %q", text), func() bool {
		for _, line := range b.JobOutput(id, 0) {
			if line.Text == text {
				return true
			}
		}

		return false
	})
}

func TestProcessInterrupt(t *testing.T) {
	b := newProcessBackend(t, "interrupt")

	j, err := b.jobs.start(&jobSpec{
		kind: clabinternaltuibackend.JobCapture, lab: "lab1", args: []string{"capture"},
	})
	if err != nil {
		t.Fatal(err)
	}

	id := j.info.ID

	// Signals sent before the handlers are installed would end the process.
	awaitOutput(t, b, id, "ready")
	b.CancelJob(id)

	// The command cleaned up and exited with success, which makes the job one.
	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobSucceeded || info.ExitCode != 0 {
		t.Errorf("unexpected job: %+v", info)
	}

	want := []string{"ready", "received interrupt", "cleaned up"}
	if diff := cmp.Diff(want, outputTexts(b.JobOutput(id, 0), true)); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}

func TestProcessKilledWhenItIgnoresSignals(t *testing.T) {
	b := newProcessBackend(t, "stubborn")
	b.jobs.graces = jobGraces{
		rollback: time.Minute,
		capture:  time.Minute,
		other:    500 * time.Millisecond,
		kill:     500 * time.Millisecond,
	}

	j, err := b.jobs.start(&jobSpec{
		kind: clabinternaltuibackend.JobDestroy, lab: "lab1", args: []string{"destroy"},
	})
	if err != nil {
		t.Fatal(err)
	}

	id := j.info.ID

	awaitOutput(t, b, id, "ready")
	b.CancelJob(id)

	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobCancelled || info.ExitCode != -1 {
		t.Errorf("unexpected job: %+v", info)
	}

	want := []string{
		"ready",
		"received interrupt",
		"the command did not stop, sending SIGTERM",
		"received terminated",
		"the command did not stop, sending SIGKILL",
	}

	out := b.JobOutput(id, 0)
	got := make([]string, 0, len(out))

	for _, line := range out {
		got = append(got, line.Text)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}

func TestProcessThatDoesNotExist(t *testing.T) {
	b := newProcessBackend(t, "report")
	b.jobs.exe = filepath.Join(t.TempDir(), "missing")

	if _, err := b.jobs.start(&jobSpec{lab: "lab1", args: []string{"deploy"}}); err == nil {
		t.Fatal("starting a missing binary did not fail")
	}

	if busy := b.jobs.busyLabs(); len(b.Jobs()) != 0 || busy.has("lab1", "") {
		t.Errorf("a job that never started left traces: %+v", b.Jobs())
	}
}

func TestProcessCaptureIntoViewer(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	report := filepath.Join(t.TempDir(), "report")

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.exe = self
		d.run = execRunner{}
		d.clock = systemClock{}
		// The test binary stands in for Wireshark as well.
		d.lookPath = func(string) (string, error) { return self, nil }
		d.environ = func() []string {
			return append(os.Environ(), childModeEnv+"=capture", childReportEnv+"="+report)
		}
	})
	b.setLabs(fabricContainers(t)...)

	id, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab:       "fabric",
		Wireshark: true,
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The viewer exits once it has seen enough, which ends the capture.
	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobSucceeded || info.Bytes == 0 {
		t.Errorf("unexpected job: %+v\n%v", info, b.JobOutput(id, 0))
	}

	got, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}

	// The viewer got the stream from its start, although it was started later, and runs
	// detached from the terminal.
	want := "args=-k -i -\nfirst=header;\nrecords=10\nsession-leader=true\nstdout-is-null=true\n"
	if string(got) != want {
		t.Errorf("the viewer reported\n%s\nwant\n%s", got, want)
	}

	out := outputTexts(b.JobOutput(id, 0), true)
	if len(out) != 1 || out[0] != "capture finished" {
		t.Errorf("unexpected output of the capture: %v", out)
	}
}
