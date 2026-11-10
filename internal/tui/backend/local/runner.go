// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

// outputDrainDelay is how long the output of a process is still read after it exited. A
// descendant that inherited the pipes can keep them open for longer than that.
const outputDrainDelay = 2 * time.Second

// strippedEnv lists the variables that do not reach a child as they are: the topology
// selection, because a child must only act on what its arguments name, and the version
// check setting, which is overridden.
var strippedEnv = []string{"CLAB_TOPO", "CLAB_NAME", "CLAB_VERSION_CHECK"}

// procSpec describes a process to start.
type procSpec struct {
	path string
	args []string
	dir  string
	env  []string
	// stdout and stderr receive the output. They are written to from separate goroutines
	// and must not fail. Output without a writer is discarded.
	stdout, stderr io.Writer
	// stdin asks for a pipe to the standard input of the process, which is the null
	// device otherwise.
	stdin bool
	// session starts the process in a session of its own, detached from the terminal,
	// instead of a process group of its own.
	session bool
	// cred makes the process run as another user.
	cred *syscall.Credential
}

// process is a started process.
type process interface {
	// stdin returns the pipe to the standard input, nil unless it was asked for.
	stdin() io.WriteCloser
	// signal sends a signal to the process group. It does nothing once the process was
	// waited for.
	signal(sig syscall.Signal) error
	// wait blocks until the process exited and its output was read. It returns the exit
	// code, which is negative when the process was ended by a signal.
	wait() (int, error)
}

// runner starts processes. Tests replace it to run without real children.
type runner interface {
	start(spec *procSpec) (process, error)
}

// execRunner starts real processes.
type execRunner struct{}

func (execRunner) start(spec *procSpec) (process, error) {
	cmd := exec.Command(spec.path, spec.args...) //nolint:noctx // ended by signals, see signal
	cmd.Dir = spec.dir
	cmd.Env = spec.env

	// With an explicit environment the working directory variable is not kept in step.
	if spec.dir != "" && spec.env != nil {
		cmd.Env = withEnv(spec.env, []string{"PWD=" + spec.dir})
	}

	cmd.WaitDelay = outputDrainDelay
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    !spec.session,
		Setsid:     spec.session,
		Credential: spec.cred,
	}

	// A nil writer must stay a nil interface for the output to go to the null device.
	if spec.stdout != nil {
		cmd.Stdout = spec.stdout
	}

	if spec.stderr != nil {
		cmd.Stderr = spec.stderr
	}

	p := &execProcess{cmd: cmd}

	if spec.stdin {
		in, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}

		p.in = in
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return p, nil
}

type execProcess struct {
	cmd *exec.Cmd
	in  io.WriteCloser

	mu     sync.Mutex
	reaped bool
}

func (p *execProcess) stdin() io.WriteCloser { return p.in }

func (p *execProcess) signal(sig syscall.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// The process ID may belong to another process once this one was reaped.
	if p.reaped {
		return nil
	}

	// The process leads its group, whose ID therefore equals the process ID.
	err := syscall.Kill(-p.cmd.Process.Pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}

	return err
}

func (p *execProcess) wait() (int, error) {
	err := p.cmd.Wait()

	p.mu.Lock()
	p.reaped = true
	p.mu.Unlock()

	var exitErr *exec.ExitError

	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), nil
	case errors.Is(err, exec.ErrWaitDelay):
		// The process exited, something else kept its output open.
		return p.cmd.ProcessState.ExitCode(), nil
	default:
		return -1, err
	}
}

// childEnv returns the environment of a containerlab child process, derived from the given
// one: the topology selection is removed and the version check, which would add its notice
// to every output, is turned off.
func childEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+1)

	for _, entry := range environ {
		if !isStrippedEnv(envName(entry)) {
			env = append(env, entry)
		}
	}

	return append(env, "CLAB_VERSION_CHECK=disable")
}

// envName returns the name of the variable an environment entry sets.
func envName(entry string) string {
	name, _, _ := strings.Cut(entry, "=")

	return name
}

func isStrippedEnv(name string) bool { return slices.Contains(strippedEnv, name) }

// commandLine renders arguments as a command line for display. The value of a password
// flag is masked.
func commandLine(program string, args []string) string {
	var b strings.Builder

	b.WriteString(program)

	mask := false

	for _, arg := range args {
		b.WriteByte(' ')

		switch {
		case mask:
			b.WriteString("******")
		default:
			b.WriteString(shellQuote(arg))
		}

		mask = arg == "--password"
	}

	return b.String()
}

// shellQuote quotes an argument the way a shell needs it, when it needs it.
func shellQuote(arg string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" +
		"@%+=:,./-_"

	if arg != "" && strings.Trim(arg, safe) == "" {
		return arg
	}

	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}
