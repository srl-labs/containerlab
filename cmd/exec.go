// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"golang.org/x/term"
)

func execCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "exec",
		Short: "execute a command in one or multiple containers",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return execFn(cobraCmd, o)
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
		&o.Exec.Stdin,
		"stdin",
		"i",
		o.Exec.Stdin,
		"pass stdin to the command; the filter must match exactly one node",
	)
	c.Flags().BoolVar(
		&o.Exec.TTY,
		"tty",
		o.Exec.TTY,
		"run the command in a terminal; the filter must match exactly one node",
	)

	c.Example = `# Show the addresses of all nodes of the lab
containerlab exec -t srl02.clab.yml --cmd 'ip -4 addr'

# Open an interactive shell on node srl1
containerlab exec -t srl02.clab.yml --label clab-node-name=srl1 -i --tty --cmd bash

# Copy a file into node srl1
containerlab exec -t srl02.clab.yml --label clab-node-name=srl1 -i \
  --cmd "sh -c 'cat > /tmp/config.sh'" < config.sh`

	return c, nil
}

func execFn(cobraCmd *cobra.Command, o *Options) error {
	ctx, cancel := context.WithCancel(cobraCmd.Context())
	defer cancel()

	if len(o.Exec.Commands) == 0 {
		return errors.New("provide command to execute")
	}

	if o.Exec.Stdin || o.Exec.TTY {
		if len(o.Exec.Commands) != 1 {
			return errors.New("the --stdin and --tty flags run exactly one command")
		}
		if cobraCmd.Flags().Changed("format") {
			return errors.New("the --format flag is not supported with --stdin and --tty")
		}
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

	if o.Exec.Stdin || o.Exec.TTY {
		return execAttachedFn(ctx, cobraCmd, o, c, listOptions)
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

// execAttachedFn runs the command with stdin and a terminal attached, as the flags ask for.
func execAttachedFn(
	ctx context.Context,
	cobraCmd *cobra.Command,
	o *Options,
	c *clabcore.CLab,
	listOptions []clabcore.ListOption,
) error {
	streams := clablabruntime.AttachStreams{
		Stdout: cobraCmd.OutOrStdout(),
		Stderr: cobraCmd.ErrOrStderr(),
	}

	if o.Exec.Stdin {
		streams.Stdin = cobraCmd.InOrStdin()
	}

	restoreTerminal := func() {}

	if o.Exec.TTY {
		stdinFd := int(os.Stdin.Fd())

		if o.Exec.Stdin {
			if !term.IsTerminal(stdinFd) {
				return errors.New("the input device is not a TTY; use --stdin without --tty")
			}

			oldState, err := term.MakeRaw(stdinFd)
			if err != nil {
				return fmt.Errorf("failed to set the terminal to raw mode: %w", err)
			}
			restoreTerminal = func() { _ = term.Restore(stdinFd, oldState) }
			defer restoreTerminal()
		}

		streams.Resize = terminalSizes(ctx)
	}

	err := c.ExecAttached(ctx, o.Exec.Commands[0], o.Exec.TTY, streams, listOptions...)

	var exitErr *clablabruntime.CommandExitError
	if errors.As(err, &exitErr) {
		// exit with the command's exit code, like docker exec, without an error message or the
		// terminal queries of printing one, whose answers the stdin copy would swallow
		restoreTerminal()
		os.Exit(exitErr.Code)
	}

	return err
}

// terminalSizes sends the size of the terminal on stdout now and whenever it changes, until ctx
// ends. A size that wasn't received yet is replaced by the newer one.
func terminalSizes(ctx context.Context) <-chan clablabruntime.TerminalSize {
	sizes := make(chan clablabruntime.TerminalSize, 1)

	stdoutFd := int(os.Stdout.Fd())
	if !term.IsTerminal(stdoutFd) {
		return sizes
	}

	send := func() {
		width, height, err := term.GetSize(stdoutFd)
		if err != nil {
			return
		}

		select {
		case <-sizes:
		default:
		}
		sizes <- clablabruntime.TerminalSize{Width: uint16(width), Height: uint16(height)}
	}

	send()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(winch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-winch:
				send()
			}
		}
	}()

	return sizes
}
