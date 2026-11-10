// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local_test

import (
	"strings"
	"testing"
	"time"

	clabcmd "github.com/srl-labs/containerlab/cmd"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuibackendlocal "github.com/srl-labs/containerlab/internal/tui/backend/local"
)

// TestCommandLinesMatchTheCLI checks every command line the backend can build against the
// commands containerlab has: the command must exist and accept each flag. The backend runs
// these commands as child processes, so a flag that is renamed or removed would otherwise
// only show when the operation is used.
func TestCommandLinesMatchTheCLI(t *testing.T) {
	const rt = "docker"

	node := clabinternaltuibackend.NodeRef{Lab: "lab1", Name: "srl1", Container: "clab-lab1-srl1"}
	share := func(
		tool clabinternaltuibackend.ShareTool, action clabinternaltuibackend.ShareAction,
	) *clabinternaltuibackend.ShareRequest {
		return &clabinternaltuibackend.ShareRequest{
			Lab: "lab1", Tool: tool, Action: action, ReadOnly: true,
			Port: 8080, Username: "user", Password: "secret", Shell: "bash",
		}
	}

	cases := map[string][]string{
		"deploy": clabinternaltuibackendlocal.DeployArgs(rt, &clabinternaltuibackend.DeployRequest{
			Lab: "lab1", TopoFile: "/labs/lab1.clab.yml", Reconfigure: true, MaxWorkers: 4,
			NodeFilter: []string{"srl1", "srl2"}, SkipPostDeploy: true,
		}),
		"destroy by name": clabinternaltuibackendlocal.DestroyArgs(
			rt, &clabinternaltuibackend.DestroyRequest{
				Lab: "lab1", Cleanup: true, Graceful: true, KeepMgmtNet: true,
			}),
		"destroy by file": clabinternaltuibackendlocal.DestroyArgs(
			rt, &clabinternaltuibackend.DestroyRequest{TopoFile: "/labs/lab1.clab.yml"}),
		"redeploy": clabinternaltuibackendlocal.RedeployArgs(
			rt, &clabinternaltuibackend.RedeployRequest{
				Lab: "lab1", TopoFile: "/labs/lab1.clab.yml", Cleanup: true, Graceful: true,
				KeepMgmtNet: true, MaxWorkers: 4,
			}),
		"save": clabinternaltuibackendlocal.SaveArgs(rt, &clabinternaltuibackend.SaveRequest{
			Lab: "lab1", Nodes: []string{"srl1"}, CopyTo: "/tmp/configs",
		}),
		"netem set": clabinternaltuibackendlocal.NetemSetArgs(
			rt, &clabinternaltuibackend.NetemRequest{
				Node: node, Interface: "e1-1",
				Impairment: clabinternaltuibackend.Impairment{
					Delay: 50 * time.Millisecond, Jitter: 5 * time.Millisecond,
					LossPct: 1.5, RateKbit: 1000, CorruptionPct: 0.1,
				},
			}),
		"netem reset": clabinternaltuibackendlocal.NetemResetArgs(rt, node.Container, "e1-1"),
		"capture": clabinternaltuibackendlocal.CaptureArgs(
			rt, &clabinternaltuibackend.CaptureRequest{
				Lab: "lab1", TopoFile: "/labs/lab1.clab.yml",
				Targets: []clabinternaltuibackend.CaptureTarget{
					{Node: "srl1", Interfaces: []string{"e1-1", "e1-2"}},
					{Node: "srl2", Interfaces: []string{"e1-1"}},
				},
				Duration: time.Minute, PacketLimit: 100,
			}, "-"),
		"events": clabinternaltuibackendlocal.EventsArgs(rt),
	}

	for _, action := range []clabinternaltuibackend.NodeAction{
		clabinternaltuibackend.NodeStart, clabinternaltuibackend.NodeStop,
		clabinternaltuibackend.NodeRestart,
	} {
		cases[string(action)] = clabinternaltuibackendlocal.NodeActionArgs(
			rt, &clabinternaltuibackend.NodeActionRequest{
				Lab: "lab1", Action: action, Nodes: []string{"srl1", "srl2"},
			})
	}

	for _, tool := range []clabinternaltuibackend.ShareTool{
		clabinternaltuibackend.ShareSSHX, clabinternaltuibackend.ShareGoTTY,
	} {
		for _, action := range []clabinternaltuibackend.ShareAction{
			clabinternaltuibackend.ShareAttach, clabinternaltuibackend.ShareDetach,
			clabinternaltuibackend.ShareReattach,
		} {
			name := string(tool) + " " + string(action)
			cases[name] = clabinternaltuibackendlocal.ShareArgs(rt, share(tool, action))
		}
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			root, err := clabcmd.Entrypoint()
			if err != nil {
				t.Fatal(err)
			}

			command, rest, err := root.Find(args)
			if err != nil {
				t.Fatalf("containerlab has no command for %q: %v", strings.Join(args, " "), err)
			}

			if command == root {
				t.Fatalf("%q does not name a command", strings.Join(args, " "))
			}

			if err := command.ParseFlags(rest); err != nil {
				t.Errorf("%q is not accepted by `containerlab %s`: %v",
					strings.Join(args, " "), command.CommandPath(), err)
			}

			if left := command.Flags().Args(); len(left) != 0 {
				t.Errorf("%q leaves arguments the command does not take: %q",
					strings.Join(args, " "), left)
			}
		})
	}
}
