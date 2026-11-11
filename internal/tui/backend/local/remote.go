// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// sshConnectionVar is the variable an SSH server sets for a session: the addresses and
	// ports of the client and of the server, in that order.
	sshConnectionVar = "SSH_CONNECTION"
	sshDefaultPort   = "22"
	// maxAncestors bounds the walk up the process tree.
	maxAncestors = 32
	rootUser     = "root"
)

// sshConnection returns the description of the SSH session the UI is used through, empty
// when there is none.
func (b *Backend) sshConnection() string {
	if conn := b.getenv(sshConnectionVar); conn != "" {
		return conn
	}

	// sudo does not pass the variable on. The shell it was run from has it.
	return ancestorEnv(b.fsRoot, os.Getppid(), sshConnectionVar)
}

// ancestorEnv looks a variable up in the environments of a process and its ancestors, the
// closest one first. Environments that cannot be read are skipped.
func ancestorEnv(root string, pid int, name string) string {
	prefix := []byte(name + "=")

	for range maxAncestors {
		if pid <= 1 {
			break
		}

		dir := filepath.Join(root, "proc", strconv.Itoa(pid))

		if environ, err := os.ReadFile(filepath.Join(dir, "environ")); err == nil {
			for entry := range bytes.SplitSeq(environ, []byte{0}) {
				if value, ok := bytes.CutPrefix(entry, prefix); ok && len(value) > 0 {
					return string(value)
				}
			}
		}

		stat, err := os.ReadFile(filepath.Join(dir, "stat"))
		if err != nil {
			break
		}

		pid = parentPID(stat)
	}

	return ""
}

// parentPID returns the parent of the process a /proc/<pid>/stat file describes, zero
// when it cannot be told. The name of the process, which comes before, can hold anything.
func parentPID(stat []byte) int {
	end := bytes.LastIndexByte(stat, ')')
	if end < 0 {
		return 0
	}

	// What follows the name is the state and then the parent.
	fields := bytes.Fields(stat[end+1:])
	if len(fields) < 2 { //nolint:mnd // see above
		return 0
	}

	pid, err := strconv.Atoi(string(fields[1]))
	if err != nil {
		return 0
	}

	return pid
}

// sshDestination turns the description of an SSH session into the arguments that make
// the ssh command reach this host as the given user. The address is the one the session
// came in on, which is not the one the user typed when there is address translation or a
// jump host on the way.
func sshDestination(user, connection string) string {
	const serverFields = 4

	fields := strings.Fields(connection)
	if len(fields) != serverFields || user == "" {
		return ""
	}

	addr, err := netip.ParseAddr(fields[2])
	if err != nil {
		return ""
	}

	port, err := strconv.ParseUint(fields[3], 10, 16)
	if err != nil {
		return ""
	}

	destination := shellQuote(user + "@" + addr.String())
	if fields[3] != sshDefaultPort {
		destination = fmt.Sprintf("-p %d %s", port, destination)
	}

	return destination
}

// RemoteCapture returns the command line that runs a capture from the computer of the
// user and shows it in the Wireshark there: the capture command runs here, through SSH,
// and its output is piped into Wireshark at the other end.
func (b *Backend) RemoteCapture(req clabinternaltuibackend.CaptureRequest) (string, error) {
	if err := b.allow(true); err != nil {
		return "", err
	}

	host := b.Host()
	if host.SSH == "" {
		return "", fmt.Errorf("%w: the UI is not used through SSH",
			clabinternaltuibackend.ErrUnavailable)
	}

	if _, err := b.prepareCapture(&req); err != nil {
		return "", err
	}

	remote := commandLine(b.remoteProgram(host.User), captureArgs(b.rtName, &req, "-"))

	return "ssh " + host.SSH + " " + shellQuote(remote) + " | " +
		commandLine(wiresharkBinary, wiresharkArgs()), nil
}

// remoteProgram returns how the containerlab binary is run by a user who logs in through
// SSH: by its name when that finds this binary, and through sudo unless the binary or the
// user has root privileges anyway.
func (b *Backend) remoteProgram(user string) string {
	program := shellQuote(b.exe)

	if found, err := b.lookPath(programName); err == nil && sameFile(found, b.exe) {
		program = programName
	}

	if !b.priv.setuid && user != rootUser {
		program = "sudo " + program
	}

	return program
}

// sameFile reports whether two paths name the same file.
func sameFile(a, b string) bool {
	if a == b {
		return true
	}

	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}

	infoB, err := os.Stat(b)
	if err != nil {
		return false
	}

	return os.SameFile(infoA, infoB)
}
