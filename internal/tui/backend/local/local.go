// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package local implements the TUI backend for the labs this host manages: the ones of its
// container runtime, or the ones a lab runtime such as c9s runs in a Kubernetes cluster.
//
// Reading is done in this process and needs no more than access to the runtime: the
// inventory comes from the runtime API, resource usage and traffic counters from /proc and
// the cgroup file system. Whatever changes a lab or needs root privileges is delegated to
// containerlab commands run as child processes, which obtain their privileges the way they
// do when run from a shell. The process itself never changes the user it runs as.
//
// With a lab runtime the nodes are not containers of this host, so what is read from the
// host otherwise is asked from the runtime: the inventory, the state and the counters of
// the interfaces, the resource usage, the logs and the sessions. Nothing needs root
// privileges then, and the commands are the same ones, run for that runtime.
package local

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	// defaultTimeout is the timeout of runtime API requests, the same the commands use.
	defaultTimeout = 2 * time.Minute
	// defaultScanDepth is how deep directories are searched for topology files.
	defaultScanDepth = 3
	// maxMessages is the number of backend messages kept.
	maxMessages = 500
	// closeWait is how long Close waits for the goroutines of the backend to end.
	closeWait = 3 * time.Second
	// versionTimeout bounds the command that tells the version of containerlab.
	versionTimeout = 10 * time.Second
	// panicRestartDelay is the pause before a loop that panicked is started again.
	panicRestartDelay = time.Second
	// maxPanicStackLines bounds how much of a stack trace is added to the messages.
	maxPanicStackLines = 30
)

// Options configures the backend.
type Options struct {
	// Runtime is the name of the container runtime or of the lab runtime, the default
	// container runtime when empty.
	Runtime string
	// Namespace limits a lab runtime to the labs of one Kubernetes namespace, which is
	// also where it deploys labs to then. It is the --namespace option of the commands.
	Namespace string
	// Timeout is the timeout of runtime API requests.
	Timeout time.Duration
	// Executable is the containerlab binary run for child commands, the binary of this
	// process when empty.
	Executable string
	// LabsDirs are directories searched for topology files, on top of the usual places.
	LabsDirs []string
	// ScanDepth is how deep directories are searched for topology files, 3 when zero.
	ScanDepth int
	// ReadOnly refuses every operation that changes state.
	ReadOnly bool
	// Debug makes the runtime client and the child commands log at debug level.
	Debug bool
}

// Backend is the clabinternaltuibackend.Backend of the local container runtime.
type Backend struct {
	opts   Options
	exe    string
	rtName string
	clab   *clabcore.CLab
	rt     clabruntime.ContainerRuntime
	// lrt is the lab runtime that runs the labs, nil when they are the ones of the
	// container runtime of this host.
	lrt  clablabruntime.LabRuntime
	priv privilege
	// hostname is the name of the host, as used in links to services published on it.
	hostname string
	// fsRoot is where the file systems of the kernel are found, the root directory
	// outside of tests.
	fsRoot string

	run      runner
	clock    clock
	lookPath func(file string) (string, error)
	getenv   func(key string) string
	environ  func() []string
	// loadTopo parses a topology file.
	loadTopo func(file string) (*clabcore.CLab, error)
	// version asks the containerlab binary for its version.
	version func(ctx context.Context) (string, error)

	// ctx ends when the backend is closed.
	ctx       context.Context //nolint:containedctx // it is the lifetime of the backend
	cancel    context.CancelFunc
	started   atomic.Bool
	closeOnce sync.Once

	// wg counts the goroutines Close waits for. Goroutines started once Close began are
	// not counted, as told by closing, which wgMu guards together with the additions to wg.
	wg      sync.WaitGroup
	wgMu    sync.Mutex
	closing bool

	notify chan struct{}

	// mu guards the fields below.
	mu      sync.Mutex
	changed clabinternaltuibackend.Topic
	host    clabinternaltuibackend.Host
	focus   clabinternaltuibackend.Focus

	msgMu    sync.Mutex
	messages lineRing

	inv    inventory
	shares shareLinks
	stats  *sampler
	ifaces ifaceStates
	topo   topologies
	// links holds the links a lab runtime reported for its labs.
	links runtimeLinks
	files topoFiles
	jobs  *jobManager

	// reloadCh carries the requests for a debounced reload of the inventory, refreshCh the
	// ones for an immediate reload. reloadMu serializes the reloads.
	reloadCh  chan struct{}
	refreshCh chan struct{}
	reloadMu  sync.Mutex
	// reloadFailing and scanTruncated remember what was reported already. Each belongs to
	// the one goroutine that reports it.
	reloadFailing bool
	scanTruncated bool

	// parseMu serializes the parsing of topology files.
	parseMu sync.Mutex
	// drawMu serializes the changes of drawing files.
	drawMu sync.Mutex

	shellMu sync.Mutex
	shells  map[string][]string
}

var _ clabinternaltuibackend.Backend = (*Backend)(nil)

// deps are the parts of a backend that tests replace.
type deps struct {
	clab     *clabcore.CLab
	rt       clabruntime.ContainerRuntime
	lrt      clablabruntime.LabRuntime
	rtName   string
	exe      string
	priv     privilege
	run      runner
	clock    clock
	fsRoot   string
	lookPath func(file string) (string, error)
	getenv   func(key string) string
	environ  func() []string
	loadTopo func(file string) (*clabcore.CLab, error)
	version  func(ctx context.Context) (string, error)
}

// New returns a backend for the labs of the local container runtime, or of a lab runtime
// when the options name one. It connects to the container runtime and fails when that is
// not possible; a lab runtime is only contacted when the backend is started.
func New(o Options) (*Backend, error) {
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}

	exe, err := resolveExecutable(o.Executable)
	if err != nil {
		return nil, err
	}

	c, err := clabcore.NewContainerLab(
		clabcore.WithTimeout(o.Timeout),
		clabcore.WithRuntime(o.Runtime, &clabruntime.RuntimeConfig{
			Timeout:      o.Timeout,
			Debug:        o.Debug,
			LabNamespace: o.Namespace,
		}),
	)
	if err != nil {
		return nil, err
	}

	if c.LabRuntime != nil {
		silenceClusterClient()

		// A lab runtime is reached through the API of its cluster as the user, which is
		// all the commands need as well.
		return newBackend(&o, &deps{
			clab:   c,
			lrt:    c.LabRuntime,
			rtName: labRuntimeName(o.Runtime),
			exe:    exe,
			priv:   privilege{ok: true, setuid: isSetuidRoot(exe)},
		}), nil
	}

	// The option initialized exactly one runtime, under its resolved name.
	for name, rt := range c.Runtimes {
		return newBackend(&o, &deps{
			clab:   c,
			rt:     rt,
			rtName: name,
			exe:    exe,
			priv:   currentPrivilege(exe),
		}), nil
	}

	return nil, fmt.Errorf("runtime %q is not a local container runtime", o.Runtime)
}

// resolveExecutable returns the absolute path of the containerlab binary to run.
func resolveExecutable(path string) (string, error) {
	if path == "" {
		self, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("cannot tell the containerlab binary to run: %w", err)
		}

		path = self
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("containerlab binary: %w", err)
	}

	return path, nil
}

// newBackend assembles a backend. Whatever d leaves unset gets its regular implementation.
func newBackend(opts *Options, d *deps) *Backend {
	o := *opts

	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}

	if o.ScanDepth <= 0 {
		o.ScanDepth = defaultScanDepth
	}

	b := &Backend{
		opts:      o,
		exe:       d.exe,
		rtName:    d.rtName,
		clab:      d.clab,
		rt:        d.rt,
		lrt:       d.lrt,
		priv:      d.priv,
		run:       d.run,
		clock:     d.clock,
		lookPath:  d.lookPath,
		getenv:    d.getenv,
		environ:   d.environ,
		loadTopo:  d.loadTopo,
		version:   d.version,
		notify:    make(chan struct{}, 1),
		messages:  newLineRing(maxMessages),
		fsRoot:    cmp.Or(d.fsRoot, string(filepath.Separator)),
		stats:     newSampler(cmp.Or(d.fsRoot, string(filepath.Separator))),
		files:     topoFiles{wanted: make(chan struct{}, 1)},
		reloadCh:  make(chan struct{}, 1),
		refreshCh: make(chan struct{}, 1),
		shells:    map[string][]string{},
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.setDefaults()

	b.hostname, _ = os.Hostname()
	if b.hostname == "" {
		b.hostname = "localhost"
	}

	b.host = clabinternaltuibackend.Host{
		Runtime:    b.rtName,
		Hostname:   b.hostname,
		User:       currentUser(b.getenv),
		Privileged: b.priv.ok,
		Cluster:    b.lrt != nil,
		Caps:       b.capabilities(),
	}
	b.host.SSH = sshDestination(b.host.User, b.sshConnection())

	graces := defaultJobGraces
	if b.lrt != nil {
		// Rolling a deployment back means waiting for the cluster to remove it.
		graces.rollback = labRuntimeRollbackGrace
	}

	b.jobs = &jobManager{
		exe:      b.exe,
		debug:    o.Debug,
		run:      b.run,
		clock:    b.clock,
		environ:  b.environ,
		graces:   graces,
		mark:     b.mark,
		spawn:    b.spawn,
		finished: b.jobFinished,
		locks:    map[string]*job{},
		output:   b.newThrottle("job output", clabinternaltuibackend.TopicJobOutput),
		progress: b.newThrottle("job progress", clabinternaltuibackend.TopicJobs),
	}

	return b
}

func (b *Backend) setDefaults() {
	if b.run == nil {
		b.run = execRunner{}
	}

	if b.clock == nil {
		b.clock = systemClock{}
	}

	if b.lookPath == nil {
		b.lookPath = exec.LookPath
	}

	if b.getenv == nil {
		b.getenv = os.Getenv
	}

	if b.environ == nil {
		b.environ = os.Environ
	}

	if b.loadTopo == nil {
		b.loadTopo = b.loadTopologyFile
	}

	if b.version == nil {
		b.version = b.executableVersion
	}
}

// newThrottle returns a throttle that signals a topic.
func (b *Backend) newThrottle(name string, topic clabinternaltuibackend.Topic) *throttle {
	return newThrottle(jobNotifyInterval, func() {
		// The throttle calls back from a timer goroutine at times.
		b.guard(name+" notification", func() { b.mark(topic) })
	})
}

// currentUser returns the name of the user at the terminal, who is not the user this
// process runs as when it was started through sudo.
func currentUser(getenv func(string) string) string {
	if name := getenv("SUDO_USER"); name != "" {
		return name
	}

	if current, err := user.Current(); err == nil {
		return current.Username
	}

	return getenv("USER")
}

// Start loads the inventory and begins to keep it current. The backend stops watching
// when ctx ends; running jobs are only stopped by Close.
func (b *Backend) Start(ctx context.Context) error {
	if !b.started.CompareAndSwap(false, true) {
		return errors.New("the backend was started already")
	}

	if err := b.reload(ctx); err != nil {
		return fmt.Errorf("failed to list the %s: %w", b.listed(), err)
	}

	context.AfterFunc(ctx, b.cancel)

	b.loop("inventory watcher", b.watch)
	b.loop("topology file scan", b.scanLoop)

	switch {
	case b.lrt != nil:
		// The events of a lab runtime tell about the nodes and their interfaces alike.
		b.loop("lab runtime event stream", b.streamLabRuntime)
		b.loop("lab runtime usage sampler", b.sampleLabRuntime)
	default:
		b.loop("runtime event stream", b.streamEvents)
		b.loop("statistics sampler", b.sampleLoop)

		if b.priv.ok {
			b.loop("interface state stream", b.ifaceStream)
		}
	}

	b.spawn("version lookup", b.lookupVersion)
	b.files.request()

	return nil
}

// Close stops the watchers, asks the running jobs to stop and waits for that to happen.
// The jobs are waited for until they ended, also one that takes a while to clean up after
// itself, ie. a deployment that is rolled back: their output is read through pipes, and a
// command whose output is gone with this process is ended by the next line it prints. How
// long that takes is bounded by the escalation of the signals that stop a job. The other
// goroutines get a moment.
func (b *Backend) Close() {
	b.closeOnce.Do(func() {
		b.jobs.cancelAll()
		b.cancel()

		b.wgMu.Lock()
		b.closing = true
		b.wgMu.Unlock()

		ended := make(chan struct{})

		go b.guard("close", func() {
			b.jobs.awaitAll()
			close(ended)
		})

		select {
		case <-ended:
		case <-time.After(b.jobs.graces.longest()):
		}

		done := make(chan struct{})

		go b.guard("close", func() {
			b.wg.Wait()
			close(done)
		})

		select {
		case <-done:
		case <-time.After(closeWait):
		}
	})
}

// Notify returns the channel that is signaled when state changed.
func (b *Backend) Notify() <-chan struct{} { return b.notify }

// Changed returns the topics that changed since the previous call.
func (b *Backend) Changed() clabinternaltuibackend.Topic {
	b.mu.Lock()
	defer b.mu.Unlock()

	changed := b.changed
	b.changed = 0

	return changed
}

// mark records that topics changed and wakes the UI.
func (b *Backend) mark(topics clabinternaltuibackend.Topic) {
	if topics == 0 {
		return
	}

	b.mu.Lock()
	b.changed |= topics
	b.mu.Unlock()

	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// SetFocus tells what is on screen. The lab in focus is sampled more often while its
// traffic rates are shown.
func (b *Backend) SetFocus(f clabinternaltuibackend.Focus) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.focus = f
}

func (b *Backend) currentFocus() clabinternaltuibackend.Focus {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.focus
}

// Host returns the description of the environment.
func (b *Backend) Host() clabinternaltuibackend.Host {
	b.mu.Lock()
	defer b.mu.Unlock()

	host := b.host
	host.Caps.Reasons = maps.Clone(b.host.Caps.Reasons)

	return host
}

// Messages returns what the backend itself logged.
func (b *Backend) Messages() []clabinternaltuibackend.LogLine {
	b.msgMu.Lock()
	defer b.msgMu.Unlock()

	return b.messages.from(0)
}

// logf adds a message of the backend. Nothing the backend has to say goes to the terminal,
// which belongs to the UI.
func (b *Backend) logf(level, format string, args ...any) {
	line := clabinternaltuibackend.LogLine{
		Time:  b.clock.Now(),
		Level: level,
		Text:  oneLine(fmt.Sprintf(format, args...)),
	}

	b.msgMu.Lock()
	b.messages.add(line)
	b.msgMu.Unlock()

	b.mark(clabinternaltuibackend.TopicMessages)
}

// spawn runs fn in a goroutine of the backend. A panic in it is reported as a message
// instead of ending the process, which would leave the terminal in the state the UI put it
// in.
func (b *Backend) spawn(name string, fn func()) {
	b.wgMu.Lock()

	counted := !b.closing
	if counted {
		b.wg.Add(1)
	}

	b.wgMu.Unlock()

	go func() {
		if counted {
			defer b.wg.Done()
		}

		b.guard(name, fn)
	}()
}

// loop runs fn in a goroutine until the backend is closed, and again should it panic.
func (b *Backend) loop(name string, fn func(ctx context.Context)) {
	b.spawn(name, func() {
		for b.ctx.Err() == nil {
			if b.guard(name, func() { fn(b.ctx) }) {
				return
			}

			if !sleep(b.ctx, panicRestartDelay) {
				return
			}
		}
	})
}

// guard runs fn and reports whether it returned without a panic. A panic is turned into
// messages.
func (b *Backend) guard(name string, fn func()) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			b.reportPanic(name, r, debug.Stack())
		}
	}()

	fn()

	return true
}

func (b *Backend) reportPanic(name string, value any, stack []byte) {
	// Reporting must not panic in turn.
	defer func() { _ = recover() }()

	b.logf(levelError, "panic in %s: %v", name, value)

	for i, line := range strings.Split(strings.TrimSpace(string(stack)), "\n") {
		if i == maxPanicStackLines {
			break
		}

		b.logf(levelDebug, "%s", line)
	}
}

// jobFinished brings the state up to date after a job: whatever the job did, the labs, the
// topology files and the interfaces may look different now.
func (b *Backend) jobFinished(*job) {
	b.requestReload()
	b.files.request()
	b.mark(clabinternaltuibackend.TopicLinks | clabinternaltuibackend.TopicInterfaces)
}

// lookupVersion fills in the version of the containerlab binary that runs the commands.
func (b *Backend) lookupVersion() {
	ctx, cancel := context.WithTimeout(b.ctx, versionTimeout)
	defer cancel()

	version, err := b.version(ctx)
	if err != nil {
		if b.ctx.Err() == nil {
			b.logf(levelWarn, "could not tell the version of %s: %v", b.exe, err)
		}

		return
	}

	b.mu.Lock()
	b.host.Version = version
	b.mu.Unlock()

	b.mark(clabinternaltuibackend.TopicHost)
}

// executableVersion asks the containerlab binary for its version.
func (b *Backend) executableVersion(ctx context.Context) (string, error) {
	const maxVersionLen = 64

	cmd := exec.CommandContext(ctx, b.exe, "version", "--short")
	cmd.Env = childEnv(b.environ())

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := oneLine(string(out))
	if version == "" || len(version) > maxVersionLen {
		return "", errors.New("unexpected output of the version command")
	}

	return version, nil
}
