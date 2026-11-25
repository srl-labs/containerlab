// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package backend

import (
	"io"
	"os/exec"
)

// Session is an interactive session on a node. It is given the terminal with the setters
// and has it for as long as Run takes.
type Session interface {
	SetStdin(r io.Reader)
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	// Run blocks until the session ended.
	Run() error
}

// CommandSession is a session that is a command of this host, ie. the command line client
// of a container runtime.
type CommandSession struct {
	Cmd *exec.Cmd
}

var _ Session = (*CommandSession)(nil)

// NewCommandSession returns the session a command provides.
func NewCommandSession(cmd *exec.Cmd) *CommandSession { return &CommandSession{Cmd: cmd} }

// SetStdin implements Session. Streams that were set on the command already are kept.
func (s *CommandSession) SetStdin(r io.Reader) {
	if s.Cmd.Stdin == nil {
		s.Cmd.Stdin = r
	}
}

// SetStdout implements Session.
func (s *CommandSession) SetStdout(w io.Writer) {
	if s.Cmd.Stdout == nil {
		s.Cmd.Stdout = w
	}
}

// SetStderr implements Session.
func (s *CommandSession) SetStderr(w io.Writer) {
	if s.Cmd.Stderr == nil {
		s.Cmd.Stderr = w
	}
}

// Run implements Session.
func (s *CommandSession) Run() error { return s.Cmd.Run() }
