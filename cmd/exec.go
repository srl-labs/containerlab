// Copyright 2020 Nokia
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

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func execCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "exec [containername]",
		Short: "execute a command in one or multiple containers",
		Long: `Execute a command inside one or multiple containers, or open an interactive shell.

Without --interactive, the command specified via --cmd is executed on all containers
matching the lab/label filters and the result is printed.

With --interactive (-i), an interactive shell session is opened inside a single
matched container. The remote shell is auto-detected from the node kind, overridden
by the node's CLAB_EXEC_INTERACTIVE_SHELL env var, or forced via --shell.
When the filter matches more than one container a beautiful TUI lets you pick one
with keyboard navigation (and optionally adjust the shell).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return execFn(cobraCmd, o, args)
		},
	}

	c.Flags().StringArrayVarP(
		&o.Exec.Commands,
		"cmd",
		"",
		o.Exec.Commands,
		"command to execute",
	)
	c.Flags().StringSliceVarP(
		&o.Filter.LabelFilter,
		"label",
		"",
		o.Filter.LabelFilter,
		"labels to filter container subset",
	)
	c.Flags().StringVarP(
		&o.Exec.Format,
		"format",
		"f",
		o.Exec.Format,
		"output format. One of [json, plain]",
	)
	c.Flags().BoolVarP(
		&o.Exec.Interactive,
		"interactive",
		"i",
		o.Exec.Interactive,
		"open an interactive shell in a single matched container",
	)
	c.Flags().StringVarP(
		&o.Exec.Shell,
		"shell",
		"s",
		o.Exec.Shell,
		"shell to use for --interactive (overrides image-based auto-detection)",
	)

	c.MarkFlagsMutuallyExclusive("cmd", "interactive")

	return c, nil
}

func execFn(_ *cobra.Command, o *Options, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if o.Exec.Interactive {
		var nameFilter string
		if len(args) == 1 {
			nameFilter = args[0]
		}
		return execInteractive(ctx, o, nameFilter)
	}

	if len(args) == 1 {
		return fmt.Errorf("positional argument %q is only valid with --interactive", args[0])
	}

	if len(o.Exec.Commands) == 0 {
		return errors.New("provide command to execute")
	}

	outputFormat, err := clabexec.ParseExecOutputFormat(o.Exec.Format)
	if err != nil {
		return err
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	listOptions := []clabcore.ListOption{
		clabcore.WithListFromCliArgs(o.Filter.LabelFilter),
	}

	if o.Global.TopologyFile != "" {
		listOptions = append(
			listOptions,
			clabcore.WithListLabName(c.Config.Name),
		)
	}

	resultCollection, err := c.Exec(ctx, o.Exec.Commands, listOptions...)
	if resultCollection != nil {
		switch outputFormat {
		case clabconstants.FormatPlain:
			resultCollection.Log()
		case clabconstants.FormatJSON:
			out, dumpErr := resultCollection.Dump(outputFormat)
			if dumpErr != nil {
				return fmt.Errorf("failed to print the results collection: %v", dumpErr)
			}

			fmt.Println(out)
		}
	}

	return err
}

// execInteractive drops the user into an interactive shell inside a single matched container.
// nameFilter, if non-empty, is a substring matched against container names after the
// topology/label filters have been applied. The topology file is auto-detected from the
// current directory when neither --topo nor --name was given.
func execInteractive(ctx context.Context, o *Options, nameFilter string) error {
	// Auto-detect topology file the same way deploy does for UX convenience.
	if o.Global.TopologyFile == "" && o.Global.TopologyName == "" {
		if found, err := clabcore.FindTopoFileByPath("."); err == nil {
			o.Global.TopologyFile = found
			log.Debugf("auto-detected topology file %q for interactive exec", found)
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

	// Apply optional substring filter on container name.
	if nameFilter != "" {
		var matched []clabruntime.GenericContainer
		for _, ct := range containers {
			n := strings.TrimPrefix(ct.Names[0], "/")
			if strings.Contains(n, nameFilter) {
				matched = append(matched, ct)
			}
		}
		containers = matched
	}

	switch len(containers) {
	case 0:
		return errors.New("no containers matched the given filters")
	case 1:
		// exactly one match — proceed directly (no TUI needed, fastest path)
	default:
		// Multiple matches: try beautiful TUI first; fall back to plain list if not a TTY.
		shellPreviewMap := buildShellPreviewMap(c, o, containers)
		selIdx, selErr := tryContainerTUI(containers, shellPreviewMap, c)
		if selErr != nil {
			if errors.Is(selErr, ErrSelectionCancelled) {
				return fmt.Errorf("selection cancelled")
			}
			// Not a terminal or TUI failed → print ambiguous list helpfully and return error.
			printAmbiguousList(os.Stderr, containers)
			return fmt.Errorf("interactive exec requires exactly one container, got %d (filtered %q): %w", len(containers), nameFilter, selErr)
		}
		containers = []clabruntime.GenericContainer{containers[selIdx]}
	}

	ct := containers[0]
	name := strings.TrimPrefix(ct.Names[0], "/")

	shell := resolveShell(o, c, ct)

	// If user did not force a shell via flag and we're on a terminal, offer the shell picker
	// as a second-stage TUI (keyboard navigation to select the right shell).
	if o.Exec.Shell == "" && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd()) {
		kind := ct.Labels[clabconstants.NodeKind]
		if kind == "" {
			kind = ct.Labels["kind"]
		}
		if kind == "" {
			// fallback: try to infer from node registry via c.Nodes
			shortName := ct.Labels[clabconstants.NodeName]
			if node, ok := c.Nodes[shortName]; ok && node != nil {
				kind = node.Config().Kind
			}
		}
		if picked, err := runShellSelector(name, kind, shell); err == nil {
			shell = picked
		} else if errors.Is(err, ErrSelectionCancelled) {
			return fmt.Errorf("shell selection cancelled")
		} else {
			// On error keep default shell but log debug
			log.Debugf("shell picker failed, using default shell %q: %v", strings.Join(shell, " "), err)
		}
	}

	prettyShell := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render(strings.Join(shell, " "))
	log.Infof("attaching to %s with shell %s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Render(name),
		prettyShell,
	)

	// Resolve runtime binary: prefer configured runtime, then docker, then podman.
	runtimeBin := resolveRuntimeBinary(o)

	dockerPath, err := exec.LookPath(runtimeBin)
	if err != nil {
		return fmt.Errorf("%s executable not found in PATH: %w", runtimeBin, err)
	}

	argv := append([]string{runtimeBin, "exec", "-it", name}, shell...)

	// syscall.Exec replaces the current process so stdin/stdout/stderr are wired directly.
	// This gives the user a native interactive experience (Ctrl+C, window resize, etc.).
	return syscall.Exec(dockerPath, argv, os.Environ())
}

// buildShellPreviewMap precomputes shell previews for each container to show in the picker.
func buildShellPreviewMap(c *clabcore.CLab, o *Options, containers []clabruntime.GenericContainer) map[string][]string {
	m := make(map[string][]string, len(containers))
	for _, ct := range containers {
		m[strings.TrimPrefix(ct.Names[0], "/")] = resolveShell(o, c, ct)
	}
	return m
}

// resolveShell determines the shell argv for a given container using priority:
//  1. --shell flag
//  2. CLAB_EXEC_INTERACTIVE_SHELL env on node
//  3. Node kind ExecInteractiveShell()
//  4. /bin/sh fallback
func resolveShell(o *Options, c *clabcore.CLab, ct clabruntime.GenericContainer) []string {
	if o.Exec.Shell != "" {
		return strings.Fields(o.Exec.Shell)
	}

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

// tryContainerTUI attempts to run the container picker TUI. Returns index or error.
func tryContainerTUI(containers []clabruntime.GenericContainer, shells map[string][]string, c *clabcore.CLab) (int, error) {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stderr.Fd()) {
		return -1, fmt.Errorf("not a terminal")
	}

	labName := ""
	if c != nil && c.Config != nil {
		labName = c.Config.Name
	}

	return runContainerSelector(containers, shells, labName)
}

func printAmbiguousList(w *os.File, containers []clabruntime.GenericContainer) {
	fmt.Fprintln(w, "ambiguous match; narrow with a more specific name, --label, or --topo:")
	for _, ct := range containers {
		name := strings.TrimPrefix(ct.Names[0], "/")
		img := shortImage(ct.Image)
		fmt.Fprintf(w, "  %s (%s) [%s]\n", name, img, ct.State)
	}
	fmt.Fprintln(w, "\nHint: run with a TTY to get an interactive picker with search.")
}

func resolveRuntimeBinary(o *Options) string {
	if o != nil && o.Global != nil && o.Global.Runtime != "" {
		rt := strings.ToLower(o.Global.Runtime)
		if strings.Contains(rt, "podman") {
			return "podman"
		}
		if strings.Contains(rt, "docker") {
			return "docker"
		}
		// allow explicit binary path like "sudo podman" not supported; fallback
		return rt
	}
	// Probe PATH preference: docker first, then podman
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return "podman"
}
