// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// argv splits a command line written with single spaces into its arguments.
func argv(line string) []string { return strings.Split(line, " ") }

func TestDeployArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.DeployRequest
		want string
	}{
		"plain": {
			req:  clabinternaltuibackend.DeployRequest{TopoFile: "/labs/a.clab.yml"},
			want: "deploy -r docker -t /labs/a.clab.yml --format json",
		},
		"all options": {
			req: clabinternaltuibackend.DeployRequest{
				TopoFile:       "/labs/a.clab.yml",
				Reconfigure:    true,
				MaxWorkers:     4,
				NodeFilter:     []string{"leaf1", "leaf2"},
				SkipPostDeploy: true,
			},
			want: "deploy -r docker -t /labs/a.clab.yml --reconfigure --max-workers 4 " +
				"--node-filter leaf1,leaf2 --skip-post-deploy --format json",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), deployArgs("docker", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDestroyArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.DestroyRequest
		want string
	}{
		"by name": {
			req:  clabinternaltuibackend.DestroyRequest{Lab: "lab1", TopoFile: "/labs/a.clab.yml"},
			want: "destroy -r podman --name lab1",
		},
		"by file when the name is unknown": {
			req:  clabinternaltuibackend.DestroyRequest{TopoFile: "/labs/a.clab.yml"},
			want: "destroy -r podman -t /labs/a.clab.yml",
		},
		"all options": {
			req: clabinternaltuibackend.DestroyRequest{
				Lab: "lab1", Cleanup: true, Graceful: true, KeepMgmtNet: true,
			},
			want: "destroy -r podman --name lab1 --cleanup --graceful --keep-mgmt-net",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), destroyArgs("podman", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRedeployArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.RedeployRequest
		want string
	}{
		"plain": {
			req:  clabinternaltuibackend.RedeployRequest{Lab: "lab1", TopoFile: "/labs/a.clab.yml"},
			want: "redeploy -r docker -t /labs/a.clab.yml --format json",
		},
		"all options": {
			req: clabinternaltuibackend.RedeployRequest{
				TopoFile: "/labs/a.clab.yml", Cleanup: true, Graceful: true,
				KeepMgmtNet: true, MaxWorkers: 2,
			},
			want: "redeploy -r docker -t /labs/a.clab.yml --cleanup --graceful " +
				"--keep-mgmt-net --max-workers 2 --format json",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), redeployArgs("docker", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNodeActionArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.NodeActionRequest
		want string
	}{
		"selected nodes": {
			req: clabinternaltuibackend.NodeActionRequest{
				Lab: "lab1", Action: clabinternaltuibackend.NodeRestart,
				Nodes: []string{"leaf1", "leaf2"},
			},
			want: "restart -r docker --name lab1 --node leaf1,leaf2",
		},
		"all nodes": {
			req: clabinternaltuibackend.NodeActionRequest{
				Lab: "lab1", Action: clabinternaltuibackend.NodeStop,
			},
			want: "stop -r docker --name lab1",
		},
		"start": {
			req: clabinternaltuibackend.NodeActionRequest{
				Lab: "lab1", Action: clabinternaltuibackend.NodeStart, Nodes: []string{"spine1"},
			},
			want: "start -r docker --name lab1 --node spine1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), nodeActionArgs("docker", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSaveArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.SaveRequest
		want string
	}{
		"whole lab": {
			req:  clabinternaltuibackend.SaveRequest{Lab: "lab1"},
			want: "save -r docker --name lab1",
		},
		"selected nodes with a copy": {
			req: clabinternaltuibackend.SaveRequest{
				Lab: "lab1", Nodes: []string{"leaf1", "leaf2"}, CopyTo: "/backup",
			},
			want: "save -r docker --name lab1 --node-filter leaf1,leaf2 --copy /backup",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), saveArgs("docker", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNetemArgs(t *testing.T) {
	node := clabinternaltuibackend.NodeRef{Lab: "lab1", Name: "leaf1", Container: "clab-lab1-leaf1"}

	tests := map[string]struct {
		netem clabinternaltuibackend.Impairment
		want  string
	}{
		"every impairment": {
			netem: clabinternaltuibackend.Impairment{
				Delay:         100 * time.Millisecond,
				Jitter:        10 * time.Millisecond,
				LossPct:       1.5,
				RateKbit:      1000,
				CorruptionPct: 0.1,
			},
			want: "tools netem set -r docker -n clab-lab1-leaf1 -i e1-1 --delay 100ms " +
				"--jitter 10ms --loss 1.5 --rate 1000 --corruption 0.1",
		},
		"the ones that are off are named as well": {
			netem: clabinternaltuibackend.Impairment{Delay: 2 * time.Second},
			want: "tools netem set -r docker -n clab-lab1-leaf1 -i e1-1 --delay 2s " +
				"--jitter 0s --loss 0 --rate 0 --corruption 0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			target := clabinternaltuibackend.InterfaceRef{Node: node, Name: "e1-1"}

			got := netemSetArgs("docker", &target, &tt.netem)
			if diff := cmp.Diff(argv(tt.want), got); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}

	want := argv("tools netem reset -r docker -n clab-lab1-leaf1 -i e1-1")
	if diff := cmp.Diff(want, netemResetArgs("docker", node.Container, "e1-1")); diff != "" {
		t.Errorf("unexpected reset arguments (-want +got):\n%s", diff)
	}
}

func TestShareArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.ShareRequest
		want string
	}{
		"sshx attach": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareSSHX,
				Action: clabinternaltuibackend.ShareAttach,
			},
			want: "tools sshx attach -r docker -l lab1",
		},
		"sshx reattach with readers": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareSSHX,
				Action: clabinternaltuibackend.ShareReattach, ReadOnly: true,
			},
			want: "tools sshx reattach -r docker -l lab1 --enable-readers",
		},
		"sshx detach takes no options": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareSSHX,
				Action: clabinternaltuibackend.ShareDetach, ReadOnly: true,
			},
			want: "tools sshx detach -r docker -l lab1",
		},
		"gotty attach with defaults": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareGoTTY,
				Action: clabinternaltuibackend.ShareAttach,
			},
			want: "tools gotty attach -r docker -l lab1",
		},
		"gotty reattach with every option": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareGoTTY,
				Action: clabinternaltuibackend.ShareReattach, Port: 9090,
				Username: "admin", Password: "s3cret", Shell: "zsh",
				// Readers are an sshx feature.
				ReadOnly: true,
			},
			want: "tools gotty reattach -r docker -l lab1 --port 9090 --username admin " +
				"--password s3cret --shell zsh",
		},
		"gotty detach takes no options": {
			req: clabinternaltuibackend.ShareRequest{
				Lab: "lab1", Tool: clabinternaltuibackend.ShareGoTTY,
				Action: clabinternaltuibackend.ShareDetach, Port: 9090, Password: "s3cret",
			},
			want: "tools gotty detach -r docker -l lab1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(argv(tt.want), shareArgs("docker", &tt.req)); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCaptureArgs(t *testing.T) {
	tests := map[string]struct {
		req  clabinternaltuibackend.CaptureRequest
		sink string
		want string
	}{
		"to the standard output": {
			req: clabinternaltuibackend.CaptureRequest{
				TopoFile: "/labs/a.clab.yml",
				Targets: []clabinternaltuibackend.CaptureTarget{
					{Node: "leaf1", Interfaces: []string{"e1-1"}},
				},
			},
			sink: "-",
			want: "capture -r docker -t /labs/a.clab.yml --target leaf1:e1-1 -w -",
		},
		"several targets into a file with limits": {
			req: clabinternaltuibackend.CaptureRequest{
				TopoFile: "/labs/a.clab.yml",
				Targets: []clabinternaltuibackend.CaptureTarget{
					{Node: "leaf1", Interfaces: []string{"e1-1", "e1-2"}},
					{Node: "spine1", Interfaces: []string{"ethernet-1/1"}},
				},
				Duration:    30 * time.Second,
				PacketLimit: 500,
			},
			sink: "/tmp/lab.pcapng",
			want: "capture -r docker -t /labs/a.clab.yml --target leaf1:e1-1,e1-2 " +
				"--target spine1:ethernet-1/1 --duration 30s --packet-limit 500 " +
				"-w /tmp/lab.pcapng",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := captureArgs("docker", &tt.req, tt.sink)
			if diff := cmp.Diff(argv(tt.want), got); diff != "" {
				t.Errorf("unexpected arguments (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStreamAndViewerArgs(t *testing.T) {
	want := argv("events -r podman --format json --initial-state")
	if diff := cmp.Diff(want, eventsArgs("podman")); diff != "" {
		t.Errorf("unexpected events arguments (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(argv("-k -i -"), wiresharkArgs()); diff != "" {
		t.Errorf("unexpected wireshark arguments (-want +got):\n%s", diff)
	}
}

func TestCommandLine(t *testing.T) {
	tests := map[string]struct {
		args []string
		want string
	}{
		"plain arguments": {
			args: argv("deploy -r docker -t /labs/a.clab.yml"),
			want: "containerlab deploy -r docker -t /labs/a.clab.yml",
		},
		"the password is masked": {
			args: argv("tools gotty attach -l lab1 --username admin --password s3cret --shell sh"),
			want: "containerlab tools gotty attach -l lab1 --username admin " +
				"--password ****** --shell sh",
		},
		"arguments a shell would split are quoted": {
			args: []string{"deploy", "-t", "/my labs/it's.clab.yml", ""},
			want: `containerlab deploy -t '/my labs/it'\''s.clab.yml' ''`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := commandLine(programName, tt.args); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestChildEnv(t *testing.T) {
	got := childEnv([]string{
		"PATH=/usr/bin",
		"CLAB_TOPO=/elsewhere/b.clab.yml",
		"CLAB_NAME=other",
		"CLAB_TOPOLOGY=kept",
		"CLAB_VERSION_CHECK=enable",
		"HOME=/home/user",
	})
	want := []string{
		"PATH=/usr/bin",
		"CLAB_TOPOLOGY=kept",
		"HOME=/home/user",
		"CLAB_VERSION_CHECK=disable",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected environment (-want +got):\n%s", diff)
	}
}

func TestWithEnv(t *testing.T) {
	got := withEnv([]string{"HOME=/root", "PATH=/usr/bin", "USER=root"},
		[]string{"HOME=/home/user", "USER=user", "XDG_RUNTIME_DIR=/run/user/1000"})
	want := []string{
		"PATH=/usr/bin", "HOME=/home/user", "USER=user", "XDG_RUNTIME_DIR=/run/user/1000",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected environment (-want +got):\n%s", diff)
	}
}
