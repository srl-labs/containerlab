// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
)

const (
	// maxJobLines is the number of output lines kept per job.
	maxJobLines = 20000
	// maxFinishedJobs is the number of finished jobs that are remembered.
	maxFinishedJobs = 50
	// jobNotifyInterval limits how often the UI is told about new output of a job.
	jobNotifyInterval = 100 * time.Millisecond

	// errorBanner is the heading the command line framework of containerlab prints above
	// the error a command failed with.
	errorBanner = "ERROR"
	// programName is how the containerlab binary is called in the commands shown.
	programName = "containerlab"
)

// jobGraces are the times a job gets to stop on its own before stronger signals are sent.
type jobGraces struct {
	// rollback applies to the commands that remove what they created when interrupted.
	rollback time.Duration
	// capture applies to packet captures, which finish their file.
	capture time.Duration
	other   time.Duration
	// kill is the time between the termination and the kill signal.
	kill time.Duration
}

const (
	rollbackGrace = 30 * time.Second
	captureGrace  = 10 * time.Second
	otherGrace    = 5 * time.Second
	killGrace     = 2 * time.Second
)

var defaultJobGraces = jobGraces{
	rollback: rollbackGrace,
	capture:  captureGrace,
	other:    otherGrace,
	kill:     killGrace,
}

// longest returns the longest a job can take to end once it was asked to stop: the longest
// grace, the one of the termination signal, and the time its output is still read.
func (g *jobGraces) longest() time.Duration {
	return max(g.rollback, g.capture, g.other) + g.kill + outputDrainDelay
}

func (g *jobGraces) of(kind clabinternaltuibackend.JobKind) time.Duration {
	switch kind {
	case clabinternaltuibackend.JobDeploy, clabinternaltuibackend.JobRedeploy:
		return g.rollback
	case clabinternaltuibackend.JobCapture:
		return g.capture
	default:
		return g.other
	}
}

// jobSpec describes a job to start.
type jobSpec struct {
	kind  clabinternaltuibackend.JobKind
	lab   string
	title string
	// topo is the topology file the job works with, if any. The command runs in the
	// directory of the file, as the paths in a topology are relative to it.
	topo string
	args []string
	// then holds the arguments of further runs of the command. They follow one another in
	// the same job, which holds the lock of its lab for all of them.
	then [][]string
	// pipedTo is the command line of the program the output of the command is piped
	// into, if any. It is only used to show what is run.
	pipedTo string
	// shared jobs run next to other jobs of their lab instead of excluding them.
	shared bool
	// errorLinesFail makes logged errors fail the job even if the command exits with
	// success. The save command exits like that when nodes could not be saved.
	errorLinesFail bool
	// output returns the writer that takes the standard output of the command instead of
	// the job's output, for commands that write data to it.
	output func(j *job) io.Writer
	// needsSink marks a job whose output is useless until a consumer was attached to it: if
	// the command exits before that, the job failed.
	needsSink bool
	// live marks a capture whose packets are decoded for the UI.
	live bool
}

// lockKeys names what a job locks: its lab and its topology file. Both are locked, as a
// request may name a lab by either of them, and two jobs must exclude each other as soon
// as they share one.
func (s *jobSpec) lockKeys() []string {
	if s.shared {
		return nil
	}

	var keys []string

	if s.lab != "" {
		keys = append(keys, "lab\x00"+s.lab)
	}

	if s.topo != "" {
		keys = append(keys, "file\x00"+s.topo)
	}

	return keys
}

// job is a containerlab command run on behalf of the UI.
type job struct {
	spec jobSpec
	proc process
	// done is closed once the command exited and the job has its final state.
	done chan struct{}

	mu    sync.Mutex
	info  clabinternaltuibackend.JobInfo
	lines lineRing
	// canceled is set when the user asked to stop the job.
	canceled bool
	// stopping is set once signals are being sent to the command.
	stopping bool
	// lastError is the message of the last error the command logged.
	lastError string
	// tail is the last line of the error output that is no log record.
	tail string
	// inBanner is set while the lines of an error banner are read, bannerStarted once the
	// first of them was seen.
	inBanner, bannerStarted bool
	// failure, when set, is why the job failed regardless of how the command exited.
	failure string
	// sinkAttached is set once the consumer of the command's output runs.
	sinkAttached bool
	// packets holds the most recent packets of a live capture.
	packets ring[livePacket]
}

// record adds a line of output to the job. The lock must be held.
func (j *job) record(now time.Time, stderr bool, text string) {
	line := clabinternaltuibackend.LogLine{Time: now, Stderr: stderr, Text: text}

	if stderr && !j.classify(&line) {
		return
	}

	j.lines.add(line)
	j.info.Lines = j.lines.total
}

// note adds a line of the backend's own to the output of the job.
func (j *job) note(now time.Time, level, text string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.lines.add(clabinternaltuibackend.LogLine{Time: now, Level: level, Text: text})
	j.info.Lines = j.lines.total
}

// announce adds the command that runs next to the output of a job that runs several. What
// was read of the error output of the command before does not carry over to it.
func (j *job) announce(now time.Time, command string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.inBanner = false
	j.tail = ""

	j.lines.add(clabinternaltuibackend.LogLine{Time: now, Level: levelInfo, Text: "$ " + command})
	j.info.Lines = j.lines.total
}

// classify interprets a line of the error output, which is where containerlab logs to. It
// reports whether the line is kept.
func (j *job) classify(line *clabinternaltuibackend.LogLine) bool {
	if tod, level, msg, ok := parseLogLine(line.Text); ok {
		line.Time = atTimeOfDay(line.Time, tod)
		line.Level = level
		line.Text = msg
		j.inBanner = false
		j.tail = ""

		switch {
		case level == levelWarn:
			j.info.Warnings++
		case isErrorLevel(level):
			j.info.Errors++
			j.lastError = msg
		}

		return true
	}

	trimmed := strings.TrimSpace(line.Text)

	switch {
	case trimmed == "":
		return false
	case trimmed == errorBanner:
		j.inBanner, j.bannerStarted = true, false

		return false
	case j.inBanner:
		// The error a command failed with is printed below the banner, wrapped and
		// padded. It counts as one error, however many lines it takes.
		line.Level = levelError
		line.Text = trimmed

		if j.bannerStarted {
			j.lastError += " " + trimmed
		} else {
			j.bannerStarted = true
			j.lastError = trimmed
			j.info.Errors++
		}
	case !unicode.IsSpace(rune(line.Text[0])):
		// Indented lines continue a log record, others stand on their own.
		j.tail = trimmed
	}

	return true
}

// conclude sets the final state of the job. The lock must be held.
func (j *job) conclude(now time.Time, code int, waitErr error) {
	exitedOK := waitErr == nil && code == 0

	j.info.Finished = now
	j.info.ExitCode = code

	switch {
	case j.failure != "":
		j.fail(j.failure)
	case exitedOK && j.spec.errorLinesFail && j.info.Errors > 0:
		j.fail(cmp.Or(j.lastError, "the command reported errors"))
	case exitedOK && j.spec.needsSink && !j.sinkAttached && !j.canceled:
		j.fail(cmp.Or(j.lastError, j.tail, "the command ended before its output was used"))
	case exitedOK:
		// A command that completed did its work, also if it was asked to stop meanwhile.
		j.info.State = clabinternaltuibackend.JobSucceeded
	case j.canceled:
		j.info.State = clabinternaltuibackend.JobCancelled
		j.info.Err = "stopped by the user"
	case waitErr != nil:
		j.fail(cmp.Or(j.lastError, j.tail, waitErr.Error()))
	case code < 0:
		j.fail(cmp.Or(j.lastError, j.tail, "terminated by a signal"))
	default:
		j.fail(cmp.Or(j.lastError, j.tail, fmt.Sprintf("exit status %d", code)))
	}
}

func (j *job) fail(reason string) {
	j.info.State = clabinternaltuibackend.JobFailed
	j.info.Err = reason
}

func (j *job) snapshot() clabinternaltuibackend.JobInfo {
	j.mu.Lock()
	defer j.mu.Unlock()

	return j.info
}

// busySet tells which labs have a job that excludes others.
type busySet struct {
	labs, files map[string]bool
}

// has reports whether the lab of the given name or topology file is busy.
func (s *busySet) has(lab, file string) bool {
	return (lab != "" && s.labs[lab]) || (file != "" && s.files[file])
}

// jobManager runs the jobs and keeps their history.
type jobManager struct {
	// exe is the containerlab binary the jobs run.
	exe string
	// debug makes the commands log at debug level.
	debug   bool
	run     runner
	clock   clock
	environ func() []string
	graces  jobGraces
	// mark tells the UI what changed, spawn starts a goroutine of the backend.
	mark  func(clabinternaltuibackend.Topic)
	spawn func(name string, fn func())
	// finished is called after a job ended.
	finished func(j *job)

	mu   sync.Mutex
	next clabinternaltuibackend.JobID
	// jobs holds the jobs in the order they were started.
	jobs []*job
	// locks holds the jobs that exclude others, by what they lock. The entry of a job
	// that is being started is nil.
	locks map[string]*job

	output   *throttle
	progress *throttle
}

// start runs a job. It fails with ErrLabBusy when the lab has a job that excludes others.
func (m *jobManager) start(spec *jobSpec) (*job, error) {
	keys := spec.lockKeys()

	m.mu.Lock()

	for _, key := range keys {
		if _, held := m.locks[key]; held {
			m.mu.Unlock()

			return nil, clabinternaltuibackend.ErrLabBusy
		}
	}

	m.setLocks(keys, nil)

	m.next++
	id := m.next

	m.mu.Unlock()

	runs := make([][]string, 0, 1+len(spec.then))
	commands := make([]string, 0, cap(runs))

	for _, args := range append([][]string{spec.args}, spec.then...) {
		if m.debug {
			args = append([]string{"--debug"}, args...)
		}

		runs = append(runs, args)
		commands = append(commands, commandLine(programName, args))
	}

	command := strings.Join(commands, "; ")
	if spec.pipedTo != "" {
		command += " | " + spec.pipedTo
	}

	// The job is complete before its command starts: output may arrive right away.
	j := &job{
		spec:    *spec,
		done:    make(chan struct{}),
		lines:   newLineRing(maxJobLines),
		packets: ring[livePacket]{limit: maxLivePackets},
		info: clabinternaltuibackend.JobInfo{
			ID:      id,
			Kind:    spec.kind,
			Lab:     spec.lab,
			Title:   spec.title,
			Command: command,
			State:   clabinternaltuibackend.JobRunning,
			Started: m.clock.Now(),
			Live:    spec.live,
		},
	}

	stderr := &lineWriter{emit: func(line string) { m.addLine(j, true, line) }}
	writers := []*lineWriter{stderr}

	var stdout io.Writer

	if spec.output != nil {
		stdout = spec.output(j)
	} else {
		lines := &lineWriter{emit: func(line string) { m.addLine(j, false, line) }}
		writers = append(writers, lines)
		stdout = lines
	}

	procs := make([]*procSpec, len(runs))
	for i, args := range runs {
		procs[i] = &procSpec{
			path:   m.exe,
			args:   args,
			dir:    workDir(spec.topo),
			env:    childEnv(m.environ()),
			stdout: stdout,
			stderr: stderr,
		}
	}

	// The output of a job that runs several commands tells which one printed what.
	if len(runs) > 1 {
		j.announce(m.clock.Now(), commands[0])
	}

	proc, err := m.run.start(procs[0])
	if err != nil {
		m.mu.Lock()
		m.releaseLocks(keys, nil)
		m.mu.Unlock()

		return nil, fmt.Errorf("failed to run %s: %w", m.exe, err)
	}

	if len(runs) > 1 {
		proc = &sequence{
			run:     m.run,
			rest:    procs[1:],
			current: proc,
			announce: func(i int) {
				// The command before is through, what is left of its output is a last line
				// without line break.
				for _, w := range writers {
					w.close()
				}

				j.announce(m.clock.Now(), commands[i+1])
				m.output.poke()
			},
		}
	}

	j.proc = proc

	m.mu.Lock()
	m.jobs = append(m.jobs, j)
	m.setLocks(keys, j)

	m.forgetOldJobs()
	m.mu.Unlock()

	m.mark(clabinternaltuibackend.TopicJobs | clabinternaltuibackend.TopicLabs)
	m.spawn("job "+spec.title, func() { m.wait(j, writers) })

	return j, nil
}

// workDir returns the directory a command working with a topology file runs in. An empty
// result leaves the command in the working directory of this process.
func workDir(topo string) string {
	if topo == "" {
		return ""
	}

	dir := filepath.Dir(topo)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}

	return dir
}

// forgetOldJobs drops the oldest finished jobs beyond the number to remember. The lock
// must be held.
func (m *jobManager) forgetOldJobs() {
	finished := 0

	for _, j := range m.jobs {
		if j.snapshot().State.Done() {
			finished++
		}
	}

	m.jobs = slices.DeleteFunc(m.jobs, func(j *job) bool {
		if finished <= maxFinishedJobs || !j.snapshot().State.Done() {
			return false
		}

		finished--

		return true
	})
}

// addLine adds a line of the command's output to its job.
func (m *jobManager) addLine(j *job, stderr bool, raw string) {
	now := m.clock.Now()

	j.mu.Lock()
	j.record(now, stderr, clabinternaltuitermtext.Sanitize(raw))
	j.mu.Unlock()

	m.output.poke()
}

// wait follows a job to its end.
func (m *jobManager) wait(j *job, writers []*lineWriter) {
	code, err := j.proc.wait()

	// The output is complete now, what is left is a last line without line break.
	for _, w := range writers {
		w.close()
	}

	j.mu.Lock()
	j.conclude(m.clock.Now(), code, err)
	j.mu.Unlock()

	m.mu.Lock()

	m.releaseLocks(j.spec.lockKeys(), j)
	m.mu.Unlock()

	close(j.done)

	m.mark(clabinternaltuibackend.TopicJobs | clabinternaltuibackend.TopicJobOutput |
		clabinternaltuibackend.TopicLabs)

	if m.finished != nil {
		m.finished(j)
	}
}

// interrupt asks the command of a job to stop: with an interrupt first, which containerlab
// handles by cleaning up after itself, then with stronger signals if it does not exit.
// byUser tells that the user gave up on the job, as opposed to the job having served its
// purpose.
func (m *jobManager) interrupt(j *job, byUser bool) {
	j.mu.Lock()

	if j.info.State.Done() {
		j.mu.Unlock()

		return
	}

	announce := false

	if byUser && j.info.State == clabinternaltuibackend.JobRunning {
		j.info.State = clabinternaltuibackend.JobCancelling
		j.canceled = true
		announce = true
	}

	escalate := !j.stopping
	j.stopping = true

	j.mu.Unlock()

	if announce {
		m.mark(clabinternaltuibackend.TopicJobs)
	}

	if escalate {
		m.spawn("job stop", func() { m.escalate(j) })
	}
}

// escalate sends the signals that stop a command, each one only if the previous one did
// not make the command exit in time.
func (m *jobManager) escalate(j *job) {
	steps := []struct {
		sig   syscall.Signal
		grace time.Duration
	}{
		{syscall.SIGINT, m.graces.of(j.spec.kind)},
		{syscall.SIGTERM, m.graces.kill},
		{syscall.SIGKILL, 0},
	}

	for i, step := range steps {
		if i > 0 {
			j.note(m.clock.Now(), levelWarn,
				fmt.Sprintf("the command did not stop, sending %s", signalName(step.sig)))
			m.output.poke()
		}

		_ = j.proc.signal(step.sig)

		if step.grace == 0 {
			return
		}

		select {
		case <-j.done:
			return
		case <-m.clock.After(step.grace):
		}
	}
}

func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	default:
		return sig.String()
	}
}

// cancelAll asks every running job to stop.
func (m *jobManager) cancelAll() {
	m.mu.Lock()
	jobs := slices.Clone(m.jobs)
	m.mu.Unlock()

	for _, j := range jobs {
		m.interrupt(j, true)
	}
}

// awaitAll waits until every job ended.
func (m *jobManager) awaitAll() {
	m.mu.Lock()
	jobs := slices.Clone(m.jobs)
	m.mu.Unlock()

	for _, j := range jobs {
		<-j.done
	}
}

// setLocks makes the job hold the keys. A nil job reserves them for a job being started.
// The caller holds the mutex.
func (m *jobManager) setLocks(keys []string, j *job) {
	for _, key := range keys {
		m.locks[key] = j
	}
}

// releaseLocks removes the keys the job holds. The caller holds the mutex.
func (m *jobManager) releaseLocks(keys []string, j *job) {
	for _, key := range keys {
		if held, ok := m.locks[key]; ok && held == j {
			delete(m.locks, key)
		}
	}
}

func (m *jobManager) get(id clabinternaltuibackend.JobID) *job {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, j := range m.jobs {
		if j.info.ID == id {
			return j
		}
	}

	return nil
}

// busyLabs returns the labs that have a job excluding others.
func (m *jobManager) busyLabs() busySet {
	m.mu.Lock()
	defer m.mu.Unlock()

	busy := busySet{labs: map[string]bool{}, files: map[string]bool{}}

	for _, j := range m.locks {
		if j == nil {
			continue
		}

		busy.labs[j.spec.lab] = true
		busy.files[j.spec.topo] = true
	}

	return busy
}

// addBytes counts data a job produced.
func (m *jobManager) addBytes(j *job, n int) {
	j.mu.Lock()
	j.info.Bytes += uint64(n) //nolint:gosec // a length is never negative
	j.mu.Unlock()

	m.progress.poke()
}

// Jobs returns the jobs, the most recent first.
func (b *Backend) Jobs() []clabinternaltuibackend.JobInfo {
	b.jobs.mu.Lock()
	jobs := slices.Clone(b.jobs.jobs)
	b.jobs.mu.Unlock()

	infos := make([]clabinternaltuibackend.JobInfo, 0, len(jobs))
	for _, j := range slices.Backward(jobs) {
		infos = append(infos, j.snapshot())
	}

	return infos
}

// Job returns a single job.
func (b *Backend) Job(id clabinternaltuibackend.JobID) (clabinternaltuibackend.JobInfo, bool) {
	j := b.jobs.get(id)
	if j == nil {
		return clabinternaltuibackend.JobInfo{}, false
	}

	return j.snapshot(), true
}

// JobOutput returns the output lines of a job from the given line number on. Lines are
// numbered from zero in the order they were printed; the oldest lines of a job that
// printed more than is kept are gone.
func (b *Backend) JobOutput(
	id clabinternaltuibackend.JobID,
	from int,
) []clabinternaltuibackend.LogLine {
	j := b.jobs.get(id)
	if j == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	return j.lines.from(from)
}

// CancelJob asks a running job to stop.
func (b *Backend) CancelJob(id clabinternaltuibackend.JobID) {
	if j := b.jobs.get(id); j != nil {
		b.jobs.interrupt(j, true)
	}
}

// throttle limits how often something is done: at once when it was not done for the
// interval, otherwise once when the interval is over.
type throttle struct {
	interval time.Duration
	do       func()

	mu    sync.Mutex
	last  time.Time
	armed bool
}

func newThrottle(interval time.Duration, do func()) *throttle {
	return &throttle{interval: interval, do: do}
}

func (t *throttle) poke() {
	t.mu.Lock()

	if t.armed {
		t.mu.Unlock()

		return
	}

	wait := t.interval - time.Since(t.last)
	if wait > 0 {
		t.armed = true
		t.mu.Unlock()

		time.AfterFunc(wait, t.fire)

		return
	}

	t.last = time.Now()
	t.mu.Unlock()

	t.do()
}

func (t *throttle) fire() {
	t.mu.Lock()
	t.armed = false
	t.last = time.Now()
	t.mu.Unlock()

	t.do()
}
