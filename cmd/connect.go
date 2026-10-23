// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func connectCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "connect [node]",
		Short: "open an interactive shell or SSH session on a lab node",
		Long: `Open an interactive shell (or SSH) on a single lab node.

Node selection happens with fuzzy matching on node names. If more than one node matches,
a beautiful TUI picker lets you fuzzy-search and navigate with the keyboard. After picking a
node, a second picker lets you choose how to connect: the node's default shell, a custom shell,
or plain SSH via the per-lab SSH config.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			var nodeFilter string
			if len(args) == 1 {
				nodeFilter = args[0]
			}
			return connectFn(cobraCmd.Context(), o, nodeFilter)
		},
	}

	c.Flags().StringVarP(
		&o.Connect.Shell,
		"shell",
		"s",
		o.Connect.Shell,
		"shell command to run instead of the node's default (skips the method picker)",
	)
	c.Flags().BoolVarP(
		&o.Connect.SSH,
		"ssh",
		"",
		o.Connect.SSH,
		"connect via SSH instead of a container shell (skips the method picker)",
	)

	return c, nil
}

// connectFn resolves the matching lab node(s) and either runs the requested
// connection method directly or drops the user into the picker TUI.
func connectFn(ctx context.Context, o *Options, nodeFilter string) error {
	// Auto-detect the topology file when nothing was provided. This makes
	// `clab connect` from a lab directory "just work".
	if o.Global.TopologyFile == "" && o.Global.TopologyName == "" {
		if found, err := clabcore.FindTopoFileByPath("."); err == nil {
			o.Global.TopologyFile = found
			log.Debugf("auto-detected topology file %q for connect", found)
		}
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	if err := c.CheckConnectivity(ctx); err != nil {
		return err
	}

	listOptions := []clabcore.ListOption{
		clabcore.WithListFromCliArgs(o.Filter.LabelFilter),
	}

	if o.Global.TopologyFile != "" {
		listOptions = append(listOptions, clabcore.WithListLabName(c.Config.Name))
	}

	containers, err := c.ListContainers(ctx, listOptions...)
	if err != nil {
		return err
	}

	// Optional substring pre-filter on the node name (the positional arg).
	if nodeFilter != "" {
		var matched []clabruntime.GenericContainer
		for _, ct := range containers {
			if strings.Contains(strings.TrimPrefix(ct.Names[0], "/"), nodeFilter) {
				matched = append(matched, ct)
			}
		}
		containers = matched
	}

	switch len(containers) {
	case 0:
		return errors.New("no nodes matched the given filters")
	case 1:
		// Exactly one match: go straight to the method picker (unless a
		// method was forced via --shell/--ssh).
		return connectSingle(ctx, o, c, containers[0])
	default:
		// Multiple matches: let the user pick the node first.
		idx, err := runNodePicker(containers, c)
		if err != nil {
			return err
		}
		return connectSingle(ctx, o, c, containers[idx])
	}
}

// connectSingle opens the connection for exactly one container. When a method
// was forced via --shell or --ssh the TUI is skipped entirely.
func connectSingle(_ context.Context, o *Options, c *clabcore.CLab, ct clabruntime.GenericContainer) error {
	name := strings.TrimPrefix(ct.Names[0], "/")

	forcedMethod := ""
	switch {
	case o.Connect.Shell != "":
		forcedMethod = "shell"
	case o.Connect.SSH:
		forcedMethod = "ssh"
	}

	var method connectMethod
	switch forcedMethod {
	case "shell":
		method = connectMethod{
			Label:       "forced shell",
			Description: "explicitly requested via --shell",
			Shell:       strings.Fields(o.Connect.Shell),
		}
	case "ssh":
		method = connectMethod{
			Label:       "ssh",
			Description: "explicitly requested via --ssh",
			SSH:         true,
		}
	default:
		m, err := runMethodPicker(ct, c)
		if err != nil {
			return err
		}
		method = m
	}

	if method.SSH {
		return connectSSH(name)
	}
	return connectShell(o, c, ct, method.Shell)
}

// connectShell replaces the current process with a `docker/podman exec -it`
// invocation of the given shell argv.
func connectShell(o *Options, c *clabcore.CLab, ct clabruntime.GenericContainer, shell []string) error {
	if len(shell) == 0 {
		shell = []string{"/bin/sh"}
	}

	runtimeBin := resolveRuntimeBinary(o)
	binPath, err := exec.LookPath(runtimeBin)
	if err != nil {
		return fmt.Errorf("%s executable not found in PATH: %w", runtimeBin, err)
	}

	name := strings.TrimPrefix(ct.Names[0], "/")
	argv := append([]string{runtimeBin, "exec", "-it", name}, shell...)

	log.Debug("connecting via runtime exec", "binary", binPath, "argv", argv)

	return syscall.Exec(binPath, argv, os.Environ())
}

// connectSSH replaces the current process with `ssh <longname>`, which relies
// on the per-lab ssh config written by `clab deploy` into
// /etc/ssh/ssh_config.d/clab-<lab>.conf.
func connectSSH(longName string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh executable not found in PATH: %w", err)
	}

	log.Debug("connecting via ssh", "host", longName)

	return syscall.Exec(sshPath, []string{"ssh", longName}, os.Environ())
}

// resolveConnectShell computes the default shell for a container using the
// same priority rules as exec interactive:
//  1. CLAB_EXEC_INTERACTIVE_SHELL env var from the node
//  2. node kind default via ExecInteractiveShell()
//  3. /bin/sh fallback
func resolveConnectShell(c *clabcore.CLab, ct clabruntime.GenericContainer) []string {
	shortName := ct.Labels[clabconstants.NodeName]
	if c != nil && c.Nodes != nil {
		if node, ok := c.Nodes[shortName]; ok && node != nil {
			if envShell := node.Config().Env[clabconstants.ClabExecInteractiveShellEnv]; envShell != "" {
				return strings.Fields(envShell)
			}
			return node.ExecInteractiveShell()
		}
	}
	return []string{"/bin/sh"}
}

// resolveRuntimeBinary returns the container runtime binary name to use.
func resolveRuntimeBinary(o *Options) string {
	if o != nil && o.Global != nil && o.Global.Runtime != "" {
		rt := strings.ToLower(o.Global.Runtime)
		switch {
		case strings.Contains(rt, "podman"):
			return "podman"
		case strings.Contains(rt, "docker"):
			return "docker"
		default:
			return rt
		}
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return "podman"
}
