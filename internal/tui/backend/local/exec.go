// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
)

const (
	// execWorkers is the number of nodes a command runs in at the same time.
	execWorkers = 8
	// maxExecOutput bounds the output kept of a command, per stream.
	maxExecOutput = 1 << 20

	truncationNote = "\n[output truncated]"

	defaultShell = "/bin/sh"
	defaultKind  = "linux"
	dockerBinary = "docker"
	podmanBinary = "podman"
	sshBinary    = "ssh"
)

// Exec runs a command in the containers of the given nodes, a few at a time, and sends the
// result of each node as it completes.
func (b *Backend) Exec(
	ctx context.Context,
	nodes []clabinternaltuibackend.NodeRef,
	command string,
) (<-chan clabinternaltuibackend.ExecResult, error) {
	parsed, err := clabexec.NewExecCmdFromString(command)
	if err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	if len(parsed.GetCmd()) == 0 {
		return nil, errors.New("no command given")
	}

	// The channel holds every result, so that no worker waits for a reader that left.
	results := make(chan clabinternaltuibackend.ExecResult, len(nodes))
	slots := make(chan struct{}, execWorkers)

	var wg sync.WaitGroup

	for _, ref := range nodes {
		wg.Add(1)

		b.spawn("node exec", func() {
			defer wg.Done()

			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				results <- clabinternaltuibackend.ExecResult{Node: ref, Err: ctx.Err()}

				return
			}

			results <- b.execInNode(ctx, ref, parsed.GetCmd())
		})
	}

	b.spawn("node exec collector", func() {
		wg.Wait()
		close(results)
	})

	return results, nil
}

// execInNode runs a command in the container of one node.
func (b *Backend) execInNode(
	ctx context.Context,
	ref clabinternaltuibackend.NodeRef,
	command []string,
) clabinternaltuibackend.ExecResult {
	result := clabinternaltuibackend.ExecResult{Node: ref}

	rec, node, ctr := b.inv.find(ref)
	if node == nil {
		result.Err = fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)

		return result
	}

	result.Node = node.NodeRef
	started := b.clock.Now()

	var (
		out *clabexec.ExecResult
		err error
	)

	// Every node gets a command of its own, the runtime may keep hold of it.
	if b.lrt != nil {
		out, err = b.execInLabRuntime(ctx, rec, node, slices.Clone(command))
	} else {
		out, err = ctr.RunExec(ctx, clabexec.NewExecCmdFromSlice(slices.Clone(command)))
	}

	result.Duration = b.clock.Now().Sub(started)

	if err != nil {
		result.Err = err

		return result
	}

	result.Stdout = cleanOutput(out.GetStdOutString())
	result.Stderr = cleanOutput(out.GetStdErrString())
	result.ExitCode = out.GetReturnCode()

	return result
}

// cleanOutput bounds the output of a command and removes what must not reach a terminal.
func cleanOutput(s string) string {
	truncated := len(s) > maxExecOutput
	if truncated {
		// Do not cut in the middle of a character.
		cut := maxExecOutput
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}

		s = s[:cut]
	}

	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = clabinternaltuitermtext.Sanitize(lines[i])
	}

	s = strings.Join(lines, "\n")
	if truncated {
		s += truncationNote
	}

	return s
}

// ConnectMethods lists the ways to open an interactive session on a node, with the shell
// the node's kind prefers first.
func (b *Backend) ConnectMethods(
	node *clabinternaltuibackend.Node,
) []clabinternaltuibackend.ConnectMethod {
	kind := defaultKind
	if node != nil && node.Kind != "" {
		kind = node.Kind
	}

	methods := clabinternaltuibackend.DefaultConnectMethods(b.kindShell(kind), kind)

	// The SSH configuration of a lab is only written for the nodes of this host.
	for i := range methods {
		if methods[i].SSH && b.lrt != nil {
			methods[i].Description = "ssh through a port forwarded to the node"
		}
	}

	return methods
}

// kindShell returns the interactive shell of a node kind. Kinds are asked once: the answer
// is the same for every node of a kind and the UI asks on every redraw.
func (b *Backend) kindShell(kind string) []string {
	b.shellMu.Lock()
	defer b.shellMu.Unlock()

	if shell, ok := b.shells[kind]; ok {
		return slices.Clone(shell)
	}

	shell := []string{defaultShell}

	b.guard("kind shell lookup", func() {
		if b.clab.Reg == nil {
			return
		}

		// A node that was never initialized knows the shell of its kind all the same.
		if n, err := b.clab.Reg.NewNodeOfKind(kind); err == nil && n != nil {
			if kindShell := n.ExecInteractiveShell(); len(kindShell) > 0 {
				shell = kindShell
			}
		}
	})

	b.shells[kind] = shell

	return slices.Clone(shell)
}

// Interactive prepares an interactive session on a node: a shell started through the
// runtime's command line client, or SSH to the node, which the SSH configuration written
// at deployment makes reachable under its container name. The standard streams are left
// unset, the caller connects them to the terminal. A node that a lab runtime runs is
// reached through that runtime instead.
func (b *Backend) Interactive(
	ref clabinternaltuibackend.NodeRef,
	method clabinternaltuibackend.ConnectMethod,
) (clabinternaltuibackend.Session, error) {
	rec, node, _ := b.inv.find(ref)

	if b.lrt != nil {
		if node == nil {
			return nil, fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)
		}

		return b.labRuntimeSession(rec, node, method)
	}

	container := ref.Container
	if node != nil {
		container = node.Container
	}

	if container == "" {
		return nil, fmt.Errorf("node %q of lab %q not found", ref.Name, ref.Lab)
	}

	if method.SSH {
		path, err := b.lookPath(sshBinary)
		if err != nil {
			return nil, fmt.Errorf("ssh executable not found in PATH: %w", err)
		}

		//nolint:noctx // ends with the session
		return clabinternaltuibackend.NewCommandSession(exec.Command(path, container)), nil
	}

	client := runtimeClient(b.rtName)

	path, err := b.lookPath(client)
	if err != nil {
		return nil, fmt.Errorf("%s executable not found in PATH: %w", client, err)
	}

	shell := method.Shell
	if len(shell) == 0 {
		shell = []string{defaultShell}
	}

	args := append([]string{"exec", "-it", container}, shell...)

	//nolint:noctx // ends with the session
	return clabinternaltuibackend.NewCommandSession(exec.Command(path, args...)), nil
}

// runtimeClient returns the command line client of a runtime.
func runtimeClient(rt string) string {
	switch {
	case strings.Contains(rt, podmanBinary):
		return podmanBinary
	case strings.Contains(rt, dockerBinary):
		return dockerBinary
	default:
		return rt
	}
}
