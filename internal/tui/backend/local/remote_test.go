// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func TestSSHDestination(t *testing.T) {
	tests := map[string]struct {
		user, connection, want string
	}{
		"default port": {"alice", "198.51.100.7 51234 192.0.2.10 22", "alice@192.0.2.10"},
		"other port":   {"alice", "198.51.100.7 51234 192.0.2.10 2222", "-p 2222 alice@192.0.2.10"},
		"ipv6":         {"alice", "2001:db8::7 51234 2001:db8::10 22", "alice@2001:db8::10"},
		"link local":   {"alice", "fe80::7%eth0 51234 fe80::10%eth0 22", "alice@fe80::10%eth0"},
		"no session":   {"alice", "", ""},
		"no user":      {"", "198.51.100.7 51234 192.0.2.10 22", ""},
		"cut short":    {"alice", "198.51.100.7 51234 192.0.2.10", ""},
		"no address":   {"alice", "a b $(reboot) 22", ""},
		"no port":      {"alice", "198.51.100.7 51234 192.0.2.10 ssh", ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := sshDestination(tc.user, tc.connection); got != tc.want {
				t.Errorf(
					"sshDestination(%q, %q) = %q, want %q",
					tc.user,
					tc.connection,
					got,
					tc.want,
				)
			}
		})
	}
}

// writeProc adds a process to a made-up /proc.
func writeProc(t *testing.T, root string, pid, parent int, name, environ string) {
	t.Helper()

	dir := filepath.Join(root, "proc", strconv.Itoa(pid))
	writeFile(t, filepath.Join(dir, "stat"),
		strconv.Itoa(pid)+" ("+name+") S "+strconv.Itoa(parent)+" 1 1 0 -1 4194304")

	if environ != "" {
		writeFile(t, filepath.Join(dir, "environ"), environ)
	}
}

func TestAncestorEnv(t *testing.T) {
	root := t.TempDir()

	// sudo, whose environment is the one of root and cannot be read, was run from a shell
	// that a session of the SSH server started.
	writeProc(t, root, 400, 300, "sudo", "")
	writeProc(
		t,
		root,
		300,
		200,
		"bash) R (x",
		"HOME=/home/alice\x00SSH_CONNECTION=1.1.1.1 5 2.2.2.2 22\x00",
	)
	writeProc(t, root, 200, 1, "sshd-session", "SSH_CONNECTION=wrong\x00")

	if got := ancestorEnv(root, 400, "SSH_CONNECTION"); got != "1.1.1.1 5 2.2.2.2 22" {
		t.Errorf("found %q", got)
	}

	if got := ancestorEnv(root, 400, "CONNECTION"); got != "" {
		t.Errorf("found %q for a variable that is not set", got)
	}

	if got := ancestorEnv(root, 999, "SSH_CONNECTION"); got != "" {
		t.Errorf("found %q for a process that does not exist", got)
	}

	// A process tree that leads in a circle ends the search.
	writeProc(t, root, 500, 501, "a", "")
	writeProc(t, root, 501, 500, "b", "")

	if got := ancestorEnv(root, 500, "SSH_CONNECTION"); got != "" {
		t.Errorf("found %q in a circle", got)
	}
}

func TestParentPID(t *testing.T) {
	tests := map[string]int{
		"12 (bash) S 7 12 12": 7,
		"12 (a) b) c) R 9 1":  9,
		"12 (bash":            0,
		"12 (bash) S":         0,
		"12 (bash) S x":       0,
		"":                    0,
	}

	for stat, want := range tests {
		if got := parentPID([]byte(stat)); got != want {
			t.Errorf("parentPID(%q) = %d, want %d", stat, got, want)
		}
	}
}

// sshEnv is the environment of a session through SSH in which sudo was used.
func sshEnv(key string) string {
	switch key {
	case "SUDO_USER":
		return "alice"
	case sshConnectionVar:
		return "198.51.100.7 51234 192.0.2.10 2222"
	default:
		return ""
	}
}

func TestRemoteCapture(t *testing.T) {
	topo := absTestdata(t, "fabric.clab.yml")
	req := clabinternaltuibackend.CaptureRequest{
		Lab: "fabric",
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1", "e1-2"}},
			{Node: "spine1", Interfaces: []string{"e1-1"}},
		},
		Duration: time.Minute,
	}

	capture := "capture -r docker -t " + topo +
		" --target leaf1:e1-1,e1-2 --target spine1:e1-1 --duration 1m0s -w -"

	tests := map[string]struct {
		adjust func(o *Options, d *deps)
		want   string
	}{
		"through sudo": {
			func(_ *Options, d *deps) { d.getenv = sshEnv },
			"ssh -p 2222 alice@192.0.2.10 'sudo containerlab " + capture + "' | wireshark -k -i -",
		},
		"setuid binary": {
			func(_ *Options, d *deps) {
				d.getenv = sshEnv
				d.priv.setuid = true
			},
			"ssh -p 2222 alice@192.0.2.10 'containerlab " + capture + "' | wireshark -k -i -",
		},
		"binary that is not on the path": {
			func(_ *Options, d *deps) {
				d.getenv = sshEnv
				d.priv.setuid = true
				d.exe = "/opt/my lab/containerlab"
			},
			`ssh -p 2222 alice@192.0.2.10 ''\''/opt/my lab/containerlab'\'' ` + capture +
				`' | wireshark -k -i -`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t, tc.adjust)
			b.setLabs(fabricContainers(t)...)

			if host := b.Host(); host.SSH != "-p 2222 alice@192.0.2.10" {
				t.Fatalf("the host is reached through %q", host.SSH)
			}

			got, err := b.RemoteCapture(req)
			if err != nil {
				t.Fatal(err)
			}

			if got != tc.want {
				t.Errorf("unexpected command:\n got %s\nwant %s", got, tc.want)
			}

			if b.run.count() != 0 {
				t.Error("asking for the command started a process")
			}
		})
	}
}

func TestRemoteCaptureThatCannotRun(t *testing.T) {
	target := []clabinternaltuibackend.CaptureTarget{{Node: "leaf1", Interfaces: []string{"e1-1"}}}

	tests := map[string]struct {
		adjust func(o *Options, d *deps)
		req    clabinternaltuibackend.CaptureRequest
		want   error
	}{
		"not through ssh": {
			func(*Options, *deps) {},
			clabinternaltuibackend.CaptureRequest{Lab: "fabric", Targets: target},
			clabinternaltuibackend.ErrUnavailable,
		},
		"read-only": {
			func(o *Options, d *deps) {
				o.ReadOnly = true
				d.getenv = sshEnv
			},
			clabinternaltuibackend.CaptureRequest{Lab: "fabric", Targets: target},
			clabinternaltuibackend.ErrUnavailable,
		},
		"no interfaces": {
			func(_ *Options, d *deps) { d.getenv = sshEnv },
			clabinternaltuibackend.CaptureRequest{Lab: "fabric"},
			nil,
		},
		"unknown lab": {
			func(_ *Options, d *deps) { d.getenv = sshEnv },
			clabinternaltuibackend.CaptureRequest{Lab: "nolab", Targets: target},
			nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newTestBackend(t, tc.adjust)
			b.setLabs(fabricContainers(t)...)

			command, err := b.RemoteCapture(tc.req)
			if err == nil || command != "" {
				t.Fatalf("got %q, %v", command, err)
			}

			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
