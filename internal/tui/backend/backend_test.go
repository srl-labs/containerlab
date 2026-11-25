// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package backend

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestDefaultConnectMethods(t *testing.T) {
	methods := DefaultConnectMethods([]string{"/opt/srlinux/bin/sr_cli"}, "nokia_srlinux")

	if len(methods) < 3 {
		t.Fatalf("expected at least 3 methods, got %d", len(methods))
	}

	if methods[0].Label != "/opt/srlinux/bin/sr_cli" {
		t.Errorf("expected the default shell first, got %q", methods[0].Label)
	}

	if !strings.Contains(methods[0].Description, "default shell") {
		t.Errorf("the default shell is not described as such: %q", methods[0].Description)
	}

	if !methods[1].SSH {
		t.Errorf("expected ssh second, got %+v", methods[1])
	}
}

// TestDefaultConnectMethodsListsEachShellOnce uses a default that is also one of the
// fallbacks: it must not be offered twice.
func TestDefaultConnectMethodsListsEachShellOnce(t *testing.T) {
	methods := DefaultConnectMethods([]string{"/bin/sh"}, "linux")

	if methods[0].Label != "/bin/sh" {
		t.Fatalf("expected the default shell first, got %q", methods[0].Label)
	}

	for _, m := range methods[1:] {
		if m.Label == "/bin/sh" {
			t.Errorf("the default shell is listed twice: %+v", methods)
		}
	}

	if got := DefaultConnectMethods(nil, "linux"); got[0].Label != "/bin/sh" {
		t.Errorf("without a default the fallback shell should come first, got %q", got[0].Label)
	}
}

func TestReadiness(t *testing.T) {
	tests := []struct {
		state, status string
		want          Readiness
	}{
		{"running", "Up 3 hours", Ready},
		{"running", "Up 3 hours (healthy)", Ready},
		{"running", "Up 12 seconds (health: starting)", Starting},
		{"running", "Up 3 hours (unhealthy)", Unhealthy},
		{"created", "Created", Starting},
		{"restarting", "Restarting (1) 2 seconds ago", Starting},
		{"paused", "Up 3 hours (Paused)", Paused},
		{"removing", "", Stopping},
		{"exited", "Exited (0) 2 days ago", Stopped},
		{"exited", "Exited (137) 4 minutes ago", Failed},
		{"dead", "Dead", Failed},
		{"", "", Unknown},
	}

	for _, tt := range tests {
		n := Node{State: tt.state, Status: tt.status}
		if got := n.Readiness(); got != tt.want {
			t.Errorf("state %q status %q: got %v, want %v", tt.state, tt.status, got, tt.want)
		}
	}
}

func TestUptime(t *testing.T) {
	tests := map[string]string{
		"Up 3 hours":                       "3h",
		"Up 41 minutes (healthy)":          "41m",
		"Up About an hour":                 "1h",
		"Up About a minute":                "1m",
		"Up Less than a second":            "1s",
		"Up 12 seconds (health: starting)": "12s",
		"Up 2 days":                        "2d",
		"Up 3 weeks":                       "3w",
		"Up 2 months":                      "2mo",
		"Exited (0) 2 days ago":            "",
		"":                                 "",
	}

	for status, want := range tests {
		n := Node{Status: status}
		if got := n.Uptime(); got != want {
			t.Errorf("Uptime of %q = %q, want %q", status, got, want)
		}
	}
}

func TestLabState(t *testing.T) {
	up := Node{State: "running", Status: "Up 1 hour"}
	starting := Node{State: "running", Status: "Up 2 seconds (health: starting)"}
	stopped := Node{State: "exited", Status: "Exited (0) 1 hour ago"}
	crashed := Node{State: "exited", Status: "Exited (1) 1 hour ago"}

	tests := []struct {
		name string
		lab  Lab
		want LabState
	}{
		{"not deployed", Lab{}, LabUndeployed},
		{"all ready", Lab{Deployed: true, Nodes: []Node{up, up}}, LabRunning},
		{"one starting", Lab{Deployed: true, Nodes: []Node{up, starting}}, LabPartial},
		{"one stopped", Lab{Deployed: true, Nodes: []Node{up, stopped}}, LabPartial},
		{"all stopped", Lab{Deployed: true, Nodes: []Node{stopped, stopped}}, LabStopped},
		{
			"crashed and none running",
			Lab{Deployed: true, Nodes: []Node{crashed, stopped}},
			LabFailed,
		},
		{"deployed without nodes", Lab{Deployed: true}, LabStopped},
	}

	for _, tt := range tests {
		if got := tt.lab.State(); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}

	lab := Lab{Deployed: true, Nodes: []Node{up, starting, stopped}}
	if got := lab.ReadyNodes(); got != 1 {
		t.Errorf("ReadyNodes() = %d, want 1", got)
	}
}

// TestCommandSession runs a command as a session: it gets the streams the session is
// given, except for the ones that were set on the command already.
func TestCommandSession(t *testing.T) {
	var own, given bytes.Buffer

	cmd := exec.CommandContext(t.Context(), "sh", "-c", "cat; echo problem >&2")
	cmd.Stderr = &own

	var session Session = NewCommandSession(cmd)

	session.SetStdin(strings.NewReader("typed"))
	session.SetStdout(&given)
	session.SetStderr(&given)

	if err := session.Run(); err != nil {
		t.Fatal(err)
	}

	if given.String() != "typed" || own.String() != "problem\n" {
		t.Errorf("the session wrote %q, and %q to the stream the command had",
			given.String(), own.String())
	}
}
