// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// deployFile creates a topology file and returns a request to deploy it.
func deployFile(t *testing.T, lab string) clabinternaltuibackend.DeployRequest {
	t.Helper()

	file := filepath.Join(t.TempDir(), lab+".clab.yml")
	writeFile(t, file, "name: "+lab+"\n")

	return clabinternaltuibackend.DeployRequest{Lab: lab, TopoFile: file}
}

func TestJobRunsToSuccess(t *testing.T) {
	b := newTestBackend(t)
	req := deployFile(t, "lab1")

	id, err := b.Deploy(req)
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)

	wantSpec := procSpec{
		path: testExe,
		args: argv("deploy -r docker -t " + req.TopoFile + " --format json"),
		dir:  filepath.Dir(req.TopoFile),
		env:  []string{"PATH=/usr/bin", "CLAB_VERSION_CHECK=disable"},
	}
	gotSpec := procSpec{
		path: proc.spec.path,
		args: proc.spec.args,
		dir:  proc.spec.dir,
		env:  proc.spec.env,
	}

	if diff := cmp.Diff(wantSpec, gotSpec, cmp.AllowUnexported(procSpec{})); diff != "" {
		t.Errorf("unexpected process (-want +got):\n%s", diff)
	}

	if proc.spec.stdin || proc.spec.session || proc.spec.cred != nil {
		t.Errorf("a job must run detached from any input and as the same user: %+v", proc.spec)
	}

	info, _ := b.Job(id)
	if info.State != clabinternaltuibackend.JobRunning ||
		info.Kind != clabinternaltuibackend.JobDeploy ||
		info.Lab != "lab1" || info.Title != "deploy lab1" ||
		!info.Started.Equal(b.clock.Now()) {
		t.Errorf("unexpected running job: %+v", info)
	}

	wantCommand := "containerlab deploy -r docker -t " + req.TopoFile + " --format json"
	if info.Command != wantCommand {
		t.Errorf("command = %q, want %q", info.Command, wantCommand)
	}

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicJobs) ||
		!topics.Has(clabinternaltuibackend.TopicLabs) {
		t.Errorf("starting a job signaled %b", topics)
	}

	proc.stderr("10:00:01 INFO Containerlab started version=0.83.0\n")
	proc.stderr("10:00:02 WARN \x1b[33mslow image pull\x1b[0m\n")
	proc.stdout("{\"lab1\": []}\n")
	b.clock.advance(3 * time.Second)
	proc.exitWith(0)

	info = b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobSucceeded || info.Err != "" || info.ExitCode != 0 {
		t.Errorf("unexpected finished job: %+v", info)
	}

	if info.Lines != 3 || info.Warnings != 1 || info.Errors != 0 {
		t.Errorf(
			"lines=%d warnings=%d errors=%d, want 3, 1, 0",
			info.Lines,
			info.Warnings,
			info.Errors,
		)
	}

	if got := info.Duration(time.Time{}); got != 3*time.Second {
		t.Errorf("duration = %s, want 3s", got)
	}

	day := b.clock.Now().Truncate(24 * time.Hour)
	want := []clabinternaltuibackend.LogLine{
		{
			Time: day.Add(10*time.Hour + time.Second), Stderr: true, Level: levelInfo,
			Text: "Containerlab started version=0.83.0",
		},
		{
			Time: day.Add(10*time.Hour + 2*time.Second), Stderr: true, Level: levelWarn,
			Text: "slow image pull",
		},
		{Time: day.Add(10 * time.Hour), Text: `{"lab1": []}`},
	}

	if diff := cmp.Diff(want, b.JobOutput(id, 0)); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(want[2:], b.JobOutput(id, 2)); diff != "" {
		t.Errorf("unexpected output from line 2 (-want +got):\n%s", diff)
	}

	if got := b.JobOutput(id, 3); len(got) != 0 {
		t.Errorf("output past the end: %v", got)
	}
}

func TestJobFailure(t *testing.T) {
	tests := map[string]struct {
		stderr string
		code   int
		// save makes the job one that fails on logged errors.
		save       bool
		wantState  clabinternaltuibackend.JobState
		wantErr    string
		wantErrors int
		wantLevels []string
	}{
		"the last logged error explains the failure": {
			stderr: "10:00:00 ERRO first problem\n10:00:01 INFO still going\n" +
				"10:00:02 ERRO second problem\n10:00:03 INFO giving up\n",
			code:       1,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "second problem",
			wantErrors: 2,
			wantLevels: []string{levelError, levelInfo, levelError, levelInfo},
		},
		"the error below the banner explains the failure": {
			stderr: "10:00:00 INFO Parsing & checking topology file=a.clab.yml\n" +
				"          \n   ERROR  \n          \n" +
				"  Lab 'lab1' not found - no running                    \n" +
				"  containers.                                          \n\n",
			code:       1,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "Lab 'lab1' not found - no running containers.",
			wantErrors: 1,
			wantLevels: []string{levelInfo, levelError, levelError},
		},
		"a banner outweighs earlier logged errors": {
			stderr:     "10:00:00 ERRO node failed\n   ERROR  \n  deployment failed.\n",
			code:       1,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "deployment failed.",
			wantErrors: 2,
			wantLevels: []string{levelError, levelError},
		},
		"a plain last line explains the failure": {
			stderr:     "10:00:00 INFO working\nunknown flag: --frmat\n",
			code:       1,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "unknown flag: --frmat",
			wantLevels: []string{levelInfo, ""},
		},
		"continuation lines are no explanation": {
			stderr:     "10:00:00 INFO working\n  │ detail of the record\n",
			code:       3,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "exit status 3",
			wantLevels: []string{levelInfo, ""},
		},
		"a fatal error counts as error": {
			stderr:     "10:00:00 FATA cannot go on\n",
			code:       1,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    "cannot go on",
			wantErrors: 1,
			wantLevels: []string{levelFatal},
		},
		"killed by a signal": {
			code:      -1,
			wantState: clabinternaltuibackend.JobFailed,
			wantErr:   "terminated by a signal",
		},
		"logged errors do not fail a command that succeeded": {
			stderr:     "10:00:00 ERRO could not set an ACL\n",
			wantState:  clabinternaltuibackend.JobSucceeded,
			wantErrors: 1,
			wantLevels: []string{levelError},
		},
		"unless it is the save command": {
			stderr: "10:00:00 INFO saved node=leaf1\n" +
				"10:00:01 ERRO node \"leaf2\" save failed: not running\n",
			save:       true,
			wantState:  clabinternaltuibackend.JobFailed,
			wantErr:    `node "leaf2" save failed: not running`,
			wantErrors: 1,
			wantLevels: []string{levelInfo, levelError},
		},
		"a clean save succeeds": {
			stderr:     "10:00:00 INFO saved node=leaf1\n",
			save:       true,
			wantState:  clabinternaltuibackend.JobSucceeded,
			wantLevels: []string{levelInfo},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t)

			j, err := b.jobs.start(&jobSpec{
				kind:           clabinternaltuibackend.JobDestroy,
				lab:            "lab1",
				title:          "destroy lab1",
				args:           []string{"destroy"},
				errorLinesFail: tt.save,
			})
			if err != nil {
				t.Fatal(err)
			}

			proc := b.run.proc(t, 0)
			proc.stderr(tt.stderr)
			proc.exitWith(tt.code)

			info := b.awaitJob(t, j.info.ID)
			if info.State != tt.wantState || info.Err != tt.wantErr ||
				info.Errors != tt.wantErrors || info.ExitCode != tt.code {
				t.Errorf("state=%s err=%q errors=%d code=%d, want %s, %q, %d, %d", info.State,
					info.Err, info.Errors, info.ExitCode, tt.wantState, tt.wantErr,
					tt.wantErrors, tt.code)
			}

			var levels []string
			for _, line := range b.JobOutput(info.ID, 0) {
				levels = append(levels, line.Level)
			}

			if diff := cmp.Diff(tt.wantLevels, levels); diff != "" {
				t.Errorf("unexpected line levels (-want +got):\n%s", diff)
			}
		})
	}
}

func TestJobWithoutLineBreakAtTheEnd(t *testing.T) {
	b := newTestBackend(t)

	j, err := b.jobs.start(&jobSpec{lab: "lab1", args: []string{"destroy"}})
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	proc.stderr("10:00:00 ERRO cut off")
	proc.exitWith(1)

	if info := b.awaitJob(t, j.info.ID); info.Err != "cut off" || info.Lines != 1 {
		t.Errorf("the last line was lost: %+v", info)
	}
}

func TestOneJobPerLab(t *testing.T) {
	b := newTestBackend(t)
	req := deployFile(t, "lab1")

	first, err := b.Deploy(req)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := b.Deploy(req); !errors.Is(err, clabinternaltuibackend.ErrLabBusy) {
		t.Fatalf("second job for the lab: err = %v, want ErrLabBusy", err)
	}

	destroy := clabinternaltuibackend.DestroyRequest{Lab: "lab1"}
	if _, err := b.Destroy(destroy); !errors.Is(err, clabinternaltuibackend.ErrLabBusy) {
		t.Fatalf("job of another kind for the lab: err = %v, want ErrLabBusy", err)
	}

	// The file is busy as well, also for a request that does not name the lab.
	unnamed := clabinternaltuibackend.DeployRequest{TopoFile: req.TopoFile}
	if _, err := b.Deploy(unnamed); !errors.Is(err, clabinternaltuibackend.ErrLabBusy) {
		t.Fatalf("job for the same file: err = %v, want ErrLabBusy", err)
	}

	// A request that names the file only holds a lock of its own on it: the deployment
	// of the lab holds the same one.
	redeploy := clabinternaltuibackend.RedeployRequest{TopoFile: req.TopoFile}
	if _, err := b.Redeploy(redeploy); !errors.Is(err, clabinternaltuibackend.ErrLabBusy) {
		t.Fatalf("redeploy of the file: err = %v, want ErrLabBusy", err)
	}

	other, err := b.Deploy(deployFile(t, "lab2"))
	if err != nil {
		t.Fatalf("job for another lab: %v", err)
	}

	if busy := b.jobs.busyLabs(); !busy.has("lab1", "") || !busy.has("", req.TopoFile) ||
		!busy.has("lab2", "") || busy.has("lab3", "/elsewhere.clab.yml") {
		t.Errorf("unexpected busy labs: %+v", busy)
	}

	b.run.proc(t, 0).exitWith(0)
	b.awaitJob(t, first)

	if busy := b.jobs.busyLabs(); busy.has("lab1", req.TopoFile) {
		t.Errorf("lab1 is still busy after its job: %+v", busy)
	}

	again, err := b.Deploy(req)
	if err != nil {
		t.Fatalf("job for the lab after the first one finished: %v", err)
	}

	jobs := b.Jobs()
	ids := make([]clabinternaltuibackend.JobID, 0, len(jobs))

	for _, info := range jobs {
		ids = append(ids, info.ID)
	}

	if diff := cmp.Diff([]clabinternaltuibackend.JobID{again, other, first}, ids); diff != "" {
		t.Errorf("jobs are not listed most recent first (-want +got):\n%s", diff)
	}
}

func TestJobThatCannotStart(t *testing.T) {
	b := newTestBackend(t)
	b.run.err = errors.New("exec format error")
	req := deployFile(t, "lab1")

	if _, err := b.Deploy(req); err == nil || !strings.Contains(err.Error(), "exec format error") {
		t.Fatalf("err = %v, want the start error", err)
	}

	if len(b.Jobs()) != 0 {
		t.Errorf("a job that never started is listed: %+v", b.Jobs())
	}

	// The lab is not left locked.
	b.run.err = nil

	if _, err := b.Deploy(req); err != nil {
		t.Errorf("job after a failed start: %v", err)
	}
}

func TestCancelEscalation(t *testing.T) {
	tests := map[string]struct {
		start func(t *testing.T, b *testBackend) (clabinternaltuibackend.JobID, error)
		grace time.Duration
	}{
		"a deployment gets time to roll back": {
			start: func(t *testing.T, b *testBackend) (clabinternaltuibackend.JobID, error) {
				t.Helper()

				return b.Deploy(deployFile(t, "lab1"))
			},
			grace: 30 * time.Second,
		},
		"a redeployment as well": {
			start: func(t *testing.T, b *testBackend) (clabinternaltuibackend.JobID, error) {
				t.Helper()

				req := deployFile(t, "lab1")

				return b.Redeploy(clabinternaltuibackend.RedeployRequest{
					Lab: req.Lab, TopoFile: req.TopoFile,
				})
			},
			grace: 30 * time.Second,
		},
		"other jobs get a short time": {
			start: func(_ *testing.T, b *testBackend) (clabinternaltuibackend.JobID, error) {
				return b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "lab1"})
			},
			grace: 5 * time.Second,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t)

			id, err := tt.start(t, b)
			if err != nil {
				t.Fatal(err)
			}

			proc := b.run.proc(t, 0)

			b.CancelJob(id)
			proc.awaitSignals(t, 1)

			if info, _ := b.Job(id); info.State != clabinternaltuibackend.JobCancelling {
				t.Errorf("state after the request to stop = %s", info.State)
			}

			// A second cancel request does not start a second round of signals.
			b.CancelJob(id)

			// Nothing more happens before the grace period is over.
			b.clock.awaitTimer(t)
			b.clock.advance(tt.grace - time.Second)

			if got := proc.received(); len(got) != 1 {
				t.Fatalf("signals before the grace period ended: %v", got)
			}

			b.clock.advance(time.Second)
			proc.awaitSignals(t, 2)

			b.clock.awaitTimer(t)
			b.clock.advance(2 * time.Second)
			proc.awaitSignals(t, 3)

			want := []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL}
			if diff := cmp.Diff(want, proc.received()); diff != "" {
				t.Errorf("unexpected signals (-want +got):\n%s", diff)
			}

			proc.exitWith(-1)

			info := b.awaitJob(t, id)
			if info.State != clabinternaltuibackend.JobCancelled || info.Err == "" {
				t.Errorf("unexpected job after the kill: %+v", info)
			}

			// A cancel request for a finished job is ignored.
			b.CancelJob(id)

			if got := proc.received(); len(got) != 3 {
				t.Errorf("signals sent to a finished job: %v", got)
			}
		})
	}
}

func TestCancelStopsEscalatingOnceTheJobEnded(t *testing.T) {
	b := newTestBackend(t)

	id, err := b.Deploy(deployFile(t, "lab1"))
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	proc.onSignal = func(syscall.Signal) {
		proc.stderr("10:00:00 ERRO received signal \"interrupt\", canceling context\n")
		proc.exitWith(1)
	}

	b.CancelJob(id)

	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobCancelled || info.Err != "stopped by the user" {
		t.Errorf("unexpected job: %+v", info)
	}

	// Time passing must not bring further signals.
	b.clock.advance(time.Hour)
	time.Sleep(10 * time.Millisecond)

	if diff := cmp.Diff([]syscall.Signal{syscall.SIGINT}, proc.received()); diff != "" {
		t.Errorf("unexpected signals (-want +got):\n%s", diff)
	}
}

func TestJobAskedToStopThatCompletes(t *testing.T) {
	b := newTestBackend(t)

	id, err := b.Deploy(deployFile(t, "lab1"))
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	proc.onSignal = func(syscall.Signal) { proc.exitWith(0) }

	b.CancelJob(id)

	if info := b.awaitJob(
		t,
		id,
	); info.State != clabinternaltuibackend.JobSucceeded ||
		info.Err != "" {
		t.Errorf("a job that completed is not a success: %+v", info)
	}
}

func TestCloseCancelsJobs(t *testing.T) {
	b := newTestBackend(t)

	id, err := b.Deploy(deployFile(t, "lab1"))
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	proc.onSignal = func(syscall.Signal) { proc.exitWith(1) }

	b.Close()

	if info, _ := b.Job(id); info.State != clabinternaltuibackend.JobCancelled {
		t.Errorf("job after Close: %+v", info)
	}

	// A closed backend starts nothing new, but still answers.
	if _, err := b.Deploy(deployFile(t, "lab2")); err == nil || b.run.count() != 1 {
		t.Errorf("a job was started after Close: err = %v", err)
	}

	done := make(chan struct{})

	b.spawn("late goroutine", func() { close(done) })

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Error("a goroutine started after Close did not run")
	}
}

func TestJobOutputIsBounded(t *testing.T) {
	b := newTestBackend(t)

	j, err := b.jobs.start(&jobSpec{lab: "lab1", args: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)

	const extra = 10

	for i := range maxJobLines + extra {
		proc.stdout(fmt.Sprintf("line %d\n", i))
	}

	proc.exitWith(0)

	info := b.awaitJob(t, j.info.ID)
	if info.Lines != maxJobLines+extra {
		t.Errorf("Lines = %d, want %d", info.Lines, maxJobLines+extra)
	}

	out := b.JobOutput(info.ID, 0)
	if len(out) != maxJobLines || out[0].Text != fmt.Sprintf("line %d", extra) {
		t.Errorf("kept %d lines starting with %q", len(out), out[0].Text)
	}

	last := b.JobOutput(info.ID, info.Lines-1)
	if len(last) != 1 || last[0].Text != fmt.Sprintf("line %d", maxJobLines+extra-1) {
		t.Errorf("unexpected last line: %+v", last)
	}
}

func TestFinishedJobsAreForgotten(t *testing.T) {
	b := newTestBackend(t)

	const extra = 3

	for i := range maxFinishedJobs + extra {
		j, err := b.jobs.start(&jobSpec{lab: fmt.Sprintf("lab%d", i), args: []string{"deploy"}})
		if err != nil {
			t.Fatal(err)
		}

		b.run.proc(t, i).exitWith(0)
		b.awaitJob(t, j.info.ID)
	}

	// The limit is applied when the next job starts.
	running, err := b.jobs.start(&jobSpec{lab: "last", args: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}

	jobs := b.Jobs()
	if len(jobs) != maxFinishedJobs+1 {
		t.Fatalf("%d jobs are kept, want %d", len(jobs), maxFinishedJobs+1)
	}

	if jobs[0].ID != running.info.ID || jobs[len(jobs)-1].ID != extra+1 {
		t.Errorf("kept the jobs %d to %d", jobs[len(jobs)-1].ID, jobs[0].ID)
	}

	if _, ok := b.Job(1); ok {
		t.Error("the oldest job is still known")
	}
}

func TestDebugIsPassedToCommands(t *testing.T) {
	b := newTestBackend(t, func(o *Options, _ *deps) { o.Debug = true })

	id, err := b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "lab1"})
	if err != nil {
		t.Fatal(err)
	}

	want := argv("--debug destroy -r docker --name lab1")
	if diff := cmp.Diff(want, b.run.proc(t, 0).spec.args); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	if info, _ := b.Job(id); info.Command != "containerlab --debug destroy -r docker --name lab1" {
		t.Errorf("command = %q", info.Command)
	}
}

func TestJobOutputIsSignaled(t *testing.T) {
	b := newTestBackend(t)

	j, err := b.jobs.start(&jobSpec{lab: "lab1", args: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}

	b.drainTopics()

	proc := b.run.proc(t, 0)
	proc.stdout("one\n")

	// The first line is signaled at once, the ones right after it a moment later.
	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicJobOutput) {
		t.Errorf("the first line signaled %b", topics)
	}

	proc.stdout("two\nthree\n")

	eventually(t, "the later lines to be signaled", func() bool {
		return b.drainTopics().Has(clabinternaltuibackend.TopicJobOutput)
	})

	proc.exitWith(0)
	b.awaitJob(t, j.info.ID)

	eventually(t, "the end of the job to be signaled", func() bool {
		return b.drainTopics().Has(clabinternaltuibackend.TopicJobs)
	})
}

// TestCloseWaitsForRollback quits while a deployment runs. The deployment is rolled back,
// which takes longer than the other goroutines get: its output must still be read until
// it ended, or the command would be ended by the next line it prints.
func TestCloseWaitsForRollback(t *testing.T) {
	b := newTestBackend(t)

	id, err := b.Deploy(deployFile(t, "lab1"))
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, 0)
	closed := make(chan struct{})

	go func() {
		b.Close()
		close(closed)
	}()

	proc.awaitSignals(t, 1)

	if got := proc.received()[0]; got != syscall.SIGINT {
		t.Fatalf("the deployment got %v, want SIGINT", got)
	}

	select {
	case <-closed:
		t.Fatal("Close returned while the deployment was rolled back")
	case <-time.After(closeWait + 200*time.Millisecond):
	}

	proc.stderr("10:00:00 INFO Removing containers\n")
	proc.exitWith(1)

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the deployment ended")
	}

	lines := b.JobOutput(id, 0)
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1].Text, "Removing containers") {
		t.Errorf("the output of the rollback was not read: %+v", lines)
	}
}
