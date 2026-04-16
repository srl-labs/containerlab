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
		Args:  cobra.MaximumNArgs(1),
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

func execFn(cobraCmd *cobra.Command, o *Options, args []string) error {
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
	if err != nil {
		return err
	}

	switch outputFormat {
	case clabconstants.FormatPlain:
		resultCollection.Log()
	case clabconstants.FormatJSON:
		out, err := resultCollection.Dump(outputFormat)
		if err != nil {
			return fmt.Errorf("failed to print the results collection: %v", err)
		}

		fmt.Println(out)
	}

	return err
}

// execInteractive drops the user into an interactive shell inside a single matched container.
// nameFilter, if non-empty, is a substring matched against container names after the
// topology/label filters have been applied. The topology file is auto-detected from the
// current directory when neither --topo nor --name was given.
func execInteractive(ctx context.Context, o *Options, nameFilter string) error {
	// Auto-detect topology file the same way tools dc does.
	if o.Global.TopologyFile == "" && o.Global.TopologyName == "" {
		if found, err := clabcore.FindTopoFileByPath("."); err == nil {
			o.Global.TopologyFile = found
		}
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
			if strings.Contains(strings.TrimPrefix(ct.Names[0], "/"), nameFilter) {
				matched = append(matched, ct)
			}
		}
		containers = matched
	}

	switch len(containers) {
	case 0:
		return errors.New("no containers matched the given filters")
	case 1:
		// exactly one match — proceed
	default:
		fmt.Fprintln(os.Stderr, "ambiguous match; narrow with a more specific name, --label, or --topo:")

		for _, ct := range containers {
			name := strings.TrimPrefix(ct.Names[0], "/")
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", name, ct.Image)
		}

		return fmt.Errorf("interactive exec requires exactly one container, got %d", len(containers))
	}

	ct := containers[0]
	name := strings.TrimPrefix(ct.Names[0], "/")

	var shell []string

	shortName := ct.Labels[clabconstants.NodeName]
	node, nodeKnown := c.Nodes[shortName]

	switch {
	case o.Exec.Shell != "":
		shell = strings.Fields(o.Exec.Shell)
	case nodeKnown && node.Config().Env["CLAB_EXEC_INTERACTIVE_SHELL"] != "":
		shell = strings.Fields(node.Config().Env["CLAB_EXEC_INTERACTIVE_SHELL"])
	case nodeKnown:
		shell = node.ExecInteractiveShell()
	default:
		shell = []string{"/bin/sh"}
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker executable not found in PATH: %w", err)
	}

	argv := append([]string{"docker", "exec", "-it", name}, shell...)

	return syscall.Exec(dockerPath, argv, os.Environ())
}
