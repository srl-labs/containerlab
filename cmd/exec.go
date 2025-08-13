// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

var (
	labelsFilter []string
	execFormat   string
	execCommands []string
)

// execCmd represents the exec command.
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "execute a command in one or multiple containers",
	RunE:  execFn,
}

func execFn(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(execCommands) == 0 {
		return errors.New("provide command to execute")
	}

	outputFormat, err := clabexec.ParseExecOutputFormat(execFormat)
	if err != nil {
		return err
	}

	opts := make([]clabcore.ClabOption, 0, 5)

	// exec can work with or without a topology file
	// when topology file is provided we need to parse it
	// when topo file is not provided, we rely on labels to perform the filtering
	if topoFile != "" {
		opts = append(opts, clabcore.WithTopoPath(topoFile, varsFile))
	}

	opts = append(opts,
		clabcore.WithTimeout(timeout),
		clabcore.WithRuntime(
			runtime,
			&clabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(debug),
	)

	if labName != "" {
		opts = append(opts, clabcore.WithLabName(labName))
	}

	c, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	listOptions := []clabcore.ListOption{
		clabcore.WithListFromCliArgs(labelsFilter),
	}

	if topoFile != "" {
		listOptions = append(
			listOptions,
			clabcore.WithListLabName(c.Config.Name),
		)
	}

	resultCollection, err := c.Exec(ctx, execCommands, listOptions...)
	if err != nil {
		return err
	}

	switch outputFormat {
	case clabexec.ExecFormatPlain:
		resultCollection.Log()
	case clabexec.ExecFormatJSON:
		out, err := resultCollection.Dump(outputFormat)
		if err != nil {
			return fmt.Errorf("failed to print the results collection: %v", err)
		}

		fmt.Println(out)
	}

	return err
}

func init() {
	RootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVarP(&execCommands, "cmd", "", []string{}, "command to execute")
	execCmd.Flags().StringSliceVarP(&labelsFilter, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
