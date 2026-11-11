// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"strconv"
	"strings"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// The functions in this file build the arguments of the containerlab commands the backend
// runs. They take the runtime name and requests whose paths are absolute already.

const (
	cmdTools      = "tools"
	flagRuntime   = "-r"
	flagTopo      = "-t"
	flagName      = "--name"
	flagInterface = "-i"
)

func deployArgs(rt string, req *clabinternaltuibackend.DeployRequest) []string {
	args := []string{"deploy", flagRuntime, rt, flagTopo, req.TopoFile}

	if req.Reconfigure {
		args = append(args, "--reconfigure")
	}

	if req.MaxWorkers > 0 {
		args = append(args, "--max-workers", strconv.Itoa(req.MaxWorkers))
	}

	if len(req.NodeFilter) > 0 {
		args = append(args, "--node-filter", strings.Join(req.NodeFilter, ","))
	}

	if req.SkipPostDeploy {
		args = append(args, "--skip-post-deploy")
	}

	return append(args, "--format", "json")
}

// destroyArgs selects the lab by its name, which makes containerlab find the topology file
// through the labels of the lab's containers and still works when that file is gone. The
// file is only named for a lab without a name.
func destroyArgs(rt string, req *clabinternaltuibackend.DestroyRequest) []string {
	args := []string{"destroy", flagRuntime, rt}

	if req.Lab != "" {
		args = append(args, flagName, req.Lab)
	} else {
		args = append(args, flagTopo, req.TopoFile)
	}

	return appendDestroyFlags(args, req.Cleanup, req.Graceful, req.KeepMgmtNet)
}

func redeployArgs(rt string, req *clabinternaltuibackend.RedeployRequest) []string {
	args := []string{"redeploy", flagRuntime, rt, flagTopo, req.TopoFile}
	args = appendDestroyFlags(args, req.Cleanup, req.Graceful, req.KeepMgmtNet)

	if req.MaxWorkers > 0 {
		args = append(args, "--max-workers", strconv.Itoa(req.MaxWorkers))
	}

	return append(args, "--format", "json")
}

func appendDestroyFlags(args []string, cleanup, graceful, keepMgmtNet bool) []string {
	if cleanup {
		args = append(args, "--cleanup")
	}

	if graceful {
		args = append(args, "--graceful")
	}

	if keepMgmtNet {
		args = append(args, "--keep-mgmt-net")
	}

	return args
}

// nodeActionArgs builds a start, stop or restart command. Without nodes the command acts on
// every node of the lab.
func nodeActionArgs(rt string, req *clabinternaltuibackend.NodeActionRequest) []string {
	args := []string{string(req.Action), flagRuntime, rt, flagName, req.Lab}

	if len(req.Nodes) > 0 {
		args = append(args, "--node", strings.Join(req.Nodes, ","))
	}

	return args
}

func saveArgs(rt string, req *clabinternaltuibackend.SaveRequest) []string {
	args := []string{"save", flagRuntime, rt, flagName, req.Lab}

	if len(req.Nodes) > 0 {
		args = append(args, "--node-filter", strings.Join(req.Nodes, ","))
	}

	if req.CopyTo != "" {
		args = append(args, "--copy", req.CopyTo)
	}

	return args
}

// netemSetArgs always names every impairment, also the ones that are off: the command
// replaces what is set on the interface, and naming them all keeps a default taken from the
// environment from slipping in.
func netemSetArgs(
	rt string,
	target *clabinternaltuibackend.InterfaceRef,
	netem *clabinternaltuibackend.Impairment,
) []string {
	return []string{
		cmdTools, "netem", "set", flagRuntime, rt,
		"-n", target.Node.Container,
		flagInterface, target.Name,
		"--delay", netem.Delay.String(),
		"--jitter", netem.Jitter.String(),
		"--loss", formatPercent(netem.LossPct),
		"--rate", strconv.FormatUint(netem.RateKbit, 10),
		"--corruption", formatPercent(netem.CorruptionPct),
	}
}

func netemResetArgs(rt, container, iface string) []string {
	return []string{
		cmdTools,
		"netem",
		"reset",
		flagRuntime,
		rt,
		"-n",
		container,
		flagInterface,
		iface,
	}
}

func formatPercent(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func shareArgs(rt string, req *clabinternaltuibackend.ShareRequest) []string {
	args := []string{cmdTools, string(req.Tool), string(req.Action), flagRuntime, rt, "-l", req.Lab}

	if req.Action == clabinternaltuibackend.ShareDetach {
		return args
	}

	if req.Tool == clabinternaltuibackend.ShareSSHX {
		if req.ReadOnly {
			args = append(args, "--enable-readers")
		}

		return args
	}

	if req.Port > 0 {
		args = append(args, "--port", strconv.Itoa(req.Port))
	}

	if req.Username != "" {
		args = append(args, "--username", req.Username)
	}

	if req.Password != "" {
		args = append(args, "--password", req.Password)
	}

	if req.Shell != "" {
		args = append(args, "--shell", req.Shell)
	}

	return args
}

// captureArgs builds a capture command writing to sink, a file or "-" for the standard
// output. The lab is selected by its topology file, or by its name when no file is given.
func captureArgs(rt string, req *clabinternaltuibackend.CaptureRequest, sink string) []string {
	args := []string{"capture", flagRuntime, rt}

	if req.TopoFile != "" {
		args = append(args, flagTopo, req.TopoFile)
	} else {
		args = append(args, flagName, req.Lab)
	}

	for _, target := range req.Targets {
		args = append(args, "--target", target.Node+":"+strings.Join(target.Interfaces, ","))
	}

	if req.Duration > 0 {
		args = append(args, "--duration", req.Duration.String())
	}

	if req.PacketLimit > 0 {
		args = append(args, "--packet-limit", strconv.Itoa(req.PacketLimit))
	}

	return append(args, "-w", sink)
}

// eventsArgs builds the command that streams the state of the interfaces of every lab.
func eventsArgs(rt string) []string {
	return []string{"events", flagRuntime, rt, "--format", "json", "--initial-state"}
}

// wiresharkArgs makes Wireshark start capturing from its standard input right away.
func wiresharkArgs() []string { return []string{"-k", flagInterface, "-"} }
