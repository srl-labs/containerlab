// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// The sessions in this file reach nodes that a lab runtime runs in a cluster. There is no
// command line client to hand the terminal to as there is for the containers of this host:
// the session goes through the runtime, in this process.

const (
	// sshPort is the port the SSH server of a node listens on.
	sshPort = 22
	// loopbackAddress is where the forwarded port of a node is offered on this host.
	loopbackAddress = "127.0.0.1"
	// forwardEndWait is how long the end of a session waits for its forwarder to end.
	forwardEndWait = time.Second
)

// sessionStreams are the standard streams a session is given.
type sessionStreams struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (s *sessionStreams) SetStdin(r io.Reader)  { s.stdin = r }
func (s *sessionStreams) SetStdout(w io.Writer) { s.stdout = w }
func (s *sessionStreams) SetStderr(w io.Writer) { s.stderr = w }

// attachSession is a shell in a node, run through the lab runtime with a terminal.
type attachSession struct {
	sessionStreams

	attacher clablabruntime.NodeAttacher
	req      clablabruntime.AttachRequest
}

var _ clabinternaltuibackend.Session = (*attachSession)(nil)

// Run implements clabinternaltuibackend.Session. The terminal is switched to raw mode for
// the session, as the terminal that matters is the one at the other end, and its size is
// passed on whenever it changes.
func (s *attachSession) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// With a terminal the output of the command arrives as one stream.
	streams := clablabruntime.AttachStreams{Stdout: s.stdout}

	if fd, ok := terminalOf(s.stdin); ok {
		if state, err := term.MakeRaw(fd); err == nil {
			defer func() { _ = term.Restore(fd, state) }()
		}

		streams.Resize = watchTerminalSize(ctx, fd)
	}

	input, err := newSessionInput(s.stdin)
	if err != nil {
		return err
	}

	// The session must have stopped reading the terminal before the UI reads keys again:
	// the next key would be lost to it otherwise.
	defer input.stop()

	streams.Stdin = input

	return s.attacher.Attach(ctx, s.req, streams)
}

// terminalOf returns the file descriptor of a stream that is a terminal.
func terminalOf(stream any) (uintptr, bool) {
	file, ok := stream.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(file.Fd()) {
		return 0, false
	}

	return file.Fd(), true
}

// watchTerminalSize reports the size of a terminal, now and whenever it changes, until ctx
// ends. Only the most recent size is kept for a reader that is behind.
func watchTerminalSize(ctx context.Context, fd uintptr) <-chan clablabruntime.TerminalSize {
	sizes := make(chan clablabruntime.TerminalSize, 1)

	report := func() {
		width, height, err := term.GetSize(fd)
		if err != nil || width <= 0 || height <= 0 {
			return
		}

		select {
		case <-sizes:
		default:
		}

		sizes <- clablabruntime.TerminalSize{
			Width:  uint16(min(width, int(^uint16(0)))),
			Height: uint16(min(height, int(^uint16(0)))),
		}
	}

	report()

	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(resized)

		for {
			select {
			case <-ctx.Done():
				return
			case <-resized:
				report()
			}
		}
	}()

	return sizes
}

// sessionInput reads the input of a session and can be made to stop doing so while a read
// is waiting for input.
type sessionInput struct {
	reader cancelreader.CancelReader

	// mu is held while a read is in progress.
	mu      sync.Mutex
	stopped bool
}

func newSessionInput(r io.Reader) (*sessionInput, error) {
	reader, err := cancelreader.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read the terminal: %w", err)
	}

	return &sessionInput{reader: reader}, nil
}

// Read implements io.Reader. Input ends once the reader was stopped.
func (i *sessionInput) Read(p []byte) (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.stopped {
		return 0, io.EOF
	}

	n, err := i.reader.Read(p)
	if errors.Is(err, cancelreader.ErrCanceled) {
		err = io.EOF
	}

	return n, err
}

// stop ends the reading and returns once no read is in progress any more.
func (i *sessionInput) stop() {
	i.reader.Cancel()

	i.mu.Lock()
	defer i.mu.Unlock()

	i.stopped = true
	_ = i.reader.Close()
}

// forwardSession is an SSH session on a node that is reached through a port the lab
// runtime forwards: the ssh command connects to a port of this host that leads to the SSH
// server of the node.
type forwardSession struct {
	sessionStreams

	// dial opens a connection to the SSH server of the node.
	dial func(ctx context.Context) (net.Conn, error)
	// ssh is the path of the ssh command, user the user to log in as, if known.
	ssh, user string
	// node names the node for the messages of the ssh command.
	node string
}

var _ clabinternaltuibackend.Session = (*forwardSession)(nil)

// Run implements clabinternaltuibackend.Session.
func (s *forwardSession) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(ctx, "tcp", net.JoinHostPort(loopbackAddress, "0"))
	if err != nil {
		return fmt.Errorf("cannot forward a port to the node: %w", err)
	}

	forwarded := make(chan error, 1)

	go func() { forwarded <- s.forward(ctx, listener) }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		listener.Close()

		return errors.New("cannot forward a port to the node")
	}

	cmd := exec.CommandContext(ctx, s.ssh, sshForwardArgs(addr.Port, s.user, s.node)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.stdin, s.stdout, s.stderr

	runErr := cmd.Run()

	// Closing the listener ends a forwarder that still waits for the command to connect.
	listener.Close()
	cancel()

	// A connection that could not be forwarded is why the command failed. A forwarder
	// that is still busy connecting is not waited for.
	select {
	case err := <-forwarded:
		if err != nil && runErr != nil {
			return fmt.Errorf("%w: %w", runErr, err)
		}
	case <-time.After(forwardEndWait):
	}

	return runErr
}

// sshForwardArgs builds the arguments of an ssh command that reaches a node through a
// forwarded port of this host. The host key is not checked, as it is not for the nodes of
// local labs either: nodes are created anew all the time, and the port is another one for
// every session.
func sshForwardArgs(port int, user, node string) []string {
	destination := loopbackAddress
	if user != "" {
		destination = user + "@" + destination
	}

	const option = "-o"

	return []string{
		"-p", strconv.Itoa(port),
		option, "StrictHostKeyChecking=no",
		option, "UserKnownHostsFile=/dev/null",
		option, "HostKeyAlias=" + node,
		destination,
	}
}

// forward connects the one connection the ssh command makes to the node and returns once
// that connection is over. Nothing else is let through the port.
func (s *forwardSession) forward(ctx context.Context, listener net.Listener) error {
	local, err := listener.Accept()

	listener.Close()

	if err != nil {
		// The command ended without ever connecting, which is no failure of the port.
		return nil
	}

	defer local.Close()

	remote, err := s.dial(ctx)
	if err != nil {
		return err
	}

	defer remote.Close()

	done := make(chan struct{}, 2) //nolint:mnd // one per direction

	pipe := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)

		done <- struct{}{}
	}

	go pipe(remote, local)
	go pipe(local, remote)

	// One direction that ended ends the connection, also when the session is over.
	select {
	case <-done:
	case <-ctx.Done():
	}

	return nil
}

// labRuntimeSession prepares an interactive session on a node that the lab runtime runs.
func (b *Backend) labRuntimeSession(
	rec *labRecord,
	node *clabinternaltuibackend.Node,
	method clabinternaltuibackend.ConnectMethod,
) (clabinternaltuibackend.Session, error) {
	name := rec.runtimeNode(node)

	if method.SSH {
		forwarder, ok := b.lrt.(clablabruntime.NodePortForwarder)
		if !ok {
			return nil, fmt.Errorf("%w: the %s runtime cannot forward ports",
				clabinternaltuibackend.ErrUnavailable, b.rtName)
		}

		path, err := b.lookPath(sshBinary)
		if err != nil {
			return nil, fmt.Errorf("ssh executable not found in PATH: %w", err)
		}

		request := clablabruntime.NodeRequest{
			Name:      rec.name,
			Namespace: rec.namespace,
			Nodes:     []string{name},
		}

		return &forwardSession{
			dial: func(ctx context.Context) (net.Conn, error) {
				return forwarder.DialPort(ctx, request, sshPort)
			},
			ssh:  path,
			user: b.kindUser(node.Kind),
			node: node.Container,
		}, nil
	}

	attacher, ok := b.lrt.(clablabruntime.NodeAttacher)
	if !ok {
		return nil, fmt.Errorf("%w: the %s runtime cannot open sessions",
			clabinternaltuibackend.ErrUnavailable, b.rtName)
	}

	shell := method.Shell
	if len(shell) == 0 {
		shell = []string{defaultShell}
	}

	return &attachSession{
		attacher: attacher,
		req: clablabruntime.AttachRequest{
			Name:      rec.name,
			Namespace: rec.namespace,
			NodeName:  name,
			Command:   shell,
			TTY:       true,
		},
	}, nil
}

// kindUser returns the user the nodes of a kind are logged in to by default, empty when
// the kind has none.
func (b *Backend) kindUser(kind string) string {
	user := ""

	b.guard("kind user lookup", func() {
		if b.clab == nil || b.clab.Reg == nil {
			return
		}

		if entry := b.clab.Reg.Kind(kind); entry != nil {
			user = entry.GetCredentials().GetUsername()
		}
	})

	return user
}
