package transport

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/scrapli/scrapligo/util"
	"golang.org/x/crypto/ssh"
)

const (
	// SystemTransport is the default "system" (/bin/ssh wrapper) transport for scrapligo.
	SystemTransport = "system"

	defaultOpenBin = "ssh"
)

// NewSystemTransport returns an instance of System transport.
func NewSystemTransport(a *SSHArgs) (*System, error) {
	t := &System{
		SSHArgs:  a,
		OpenBin:  defaultOpenBin,
		OpenArgs: make([]string, 0),
		fd:       nil,
	}

	return t, nil
}

// System is the default (/bin/ssh wrapper) transport object.
type System struct {
	SSHArgs   *SSHArgs
	ExtraArgs []string
	OpenBin   string
	OpenArgs  []string
	fd        *os.File
	c         *exec.Cmd
}

func (t *System) buildOpenArgs(a *Args) {
	if len(t.OpenArgs) > 0 {
		t.OpenArgs = []string{}
	}

	t.OpenArgs = []string{
		a.Host,
		"-p",
		fmt.Sprintf("%d", a.Port),
		"-o",
		fmt.Sprintf("ConnectTimeout=%d", int(a.TimeoutSocket.Seconds())),
		"-o",
		fmt.Sprintf("ServerAliveInterval=%d", int(a.TimeoutSocket.Seconds())),
	}

	if a.User != "" {
		t.OpenArgs = append(
			t.OpenArgs,
			"-l",
			a.User,
		)
	}

	if t.SSHArgs.StrictKey {
		t.OpenArgs = append(
			t.OpenArgs,
			"-o",
			"StrictHostKeyChecking=yes",
		)

		if t.SSHArgs.KnownHostsFile != "" {
			t.OpenArgs = append(
				t.OpenArgs,
				"-o",
				fmt.Sprintf("UserKnownHostsFile=%s", t.SSHArgs.KnownHostsFile),
			)
		}
	} else {
		t.OpenArgs = append(
			t.OpenArgs,
			"-o",
			"StrictHostKeyChecking=no",
			"-o",
			"UserKnownHostsFile=/dev/null",
		)
	}

	if t.SSHArgs.ConfigFile != "" {
		t.OpenArgs = append(
			t.OpenArgs,
			"-F",
			t.SSHArgs.ConfigFile,
		)
	} else {
		t.OpenArgs = append(
			t.OpenArgs,
			"-F",
			"/dev/null",
		)
	}

	if t.SSHArgs.PrivateKeyPath != "" {
		t.OpenArgs = append(
			t.OpenArgs,
			"-i",
			t.SSHArgs.PrivateKeyPath,
		)
	}

	if len(t.ExtraArgs) > 0 {
		t.OpenArgs = append(
			t.OpenArgs,
			t.ExtraArgs...,
		)
	}
}

func (t *System) open(a *Args) error {
	if len(t.OpenArgs) == 0 {
		t.buildOpenArgs(a)
	}

	a.l.Debugf("opening system transport with bin '%s' and args '%s'", t.OpenBin, t.OpenArgs)

	t.c = exec.Command(t.OpenBin, t.OpenArgs...) //nolint:gosec

	var err error

	t.fd, err = pty.StartWithSize(
		t.c,
		&pty.Winsize{
			// ignoring linters we know these will fit in uint16
			Rows: uint16(a.TermHeight), //nolint:gosec
			Cols: uint16(a.TermWidth),  //nolint:gosec
		},
	)
	if err != nil {
		a.l.Criticalf("encountered error spawning pty, error: %s", err)

		return err
	}

	return nil
}

func (t *System) openNetconf(a *Args) error {
	if len(t.OpenArgs) == 0 {
		t.buildOpenArgs(a)
	}

	t.OpenArgs = append(t.OpenArgs, "-s", "netconf")

	a.l.Debugf("opening system transport with bin '%s' and args '%s'", t.OpenBin, t.OpenArgs)

	t.c = exec.Command(t.OpenBin, t.OpenArgs...) //nolint:gosec

	var err error

	t.fd, err = pty.Start(t.c)
	if err != nil {
		a.l.Criticalf("encountered error spawning pty, error: %s", err)

		return err
	}

	return nil
}

// Open opens the System transport.
func (t *System) Open(a *Args) error {
	// check that the  private key exists, is readable and is a ssh private key
	if t.SSHArgs.PrivateKeyPath != "" {
		if t.SSHArgs.PrivateKeyPassPhrase != "" {
			a.l.Critical("password protected key with system transport is not supported")

			return util.ErrBadOption
		}

		k, err := os.ReadFile(t.SSHArgs.PrivateKeyPath)
		if err != nil {
			a.l.Criticalf("error reading ssh key: %s", err)

			return err
		}

		_, err = ssh.ParsePrivateKey(k)
		if err != nil {
			a.l.Criticalf("error parsing ssh key: %s", err)

			return err
		}
	}

	if t.SSHArgs.NetconfConnection {
		return t.openNetconf(a)
	}

	return t.open(a)
}

// Close closes the System transport.
func (t *System) Close() error {
	err := t.fd.Close()

	t.fd = nil

	// t.c.ProcessState is always nil in our case
	if t.c != nil && t.c.Process != nil {
		err = t.c.Process.Kill()
		if err != nil {
			return err
		}
	}

	return err
}

// IsAlive returns true if the System transport file descriptor is not nil.
func (t *System) IsAlive() bool {
	return t.fd != nil
}

// Read reads n bytes from the transport.
func (t *System) Read(n int) ([]byte, error) {
	b := make([]byte, n)
	// we tried to make this call non blocking by calling syscall.SetNonBlock and SetReadDeadline
	// but it doesn't seem possible with pty implementation
	// see https://github.com/creack/pty/pull/167
	// and https://github.com/creack/pty/issues/174
	n, err := t.fd.Read(b)
	if err != nil {
		return nil, err
	}

	return b[0:n], nil
}

// Write writes bytes b to the transport.
func (t *System) Write(b []byte) error {
	_, err := t.fd.Write(b)

	return err
}

// GetInChannelAuthType returns the in channel auth flavor for the system transport.
func (t *System) GetInChannelAuthType() InChannelAuthType {
	return InChannelAuthSSH
}

// GetSSHArgs returns the ssh args for the system transport.
func (t *System) GetSSHArgs() *SSHArgs {
	return t.SSHArgs
}
