// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	labelsFilter []string
	execFormat   string
	execCommands []string
)

// execCmd represents the exec command.
var execCmd = &cobra.Command{
	Use:     "exec",
	Short:   "execute a command on one or multiple containers",
	PreRunE: sudoCheck,
	RunE:    execFn,
}

func execFn(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(execCommands) == 0 {
		return errors.New("provide command to execute")
	}

	outputFormat, err := exec.ParseExecOutputFormat(execFormat)
	if err != nil {
		return err
	}

	opts := make([]clab.ClabOption, 0, 5)

	// exec can work with or without a topology file
	// when topology file is provided we need to parse it
	// when topo file is not provided, we rely on labels to perform the filtering
	if topo != "" {
		opts = append(opts, clab.WithTopoPath(topo, varsFile))
	}

	opts = append(opts,
		clab.WithTimeout(timeout),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
		clab.WithDebug(debug),
	)

	if name != "" {
		opts = append(opts, clab.WithLabName(name))
	}

	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	var filters []*types.GenericFilter

	if len(labelsFilter) != 0 {
		filters = types.FilterFromLabelStrings(labelsFilter)
	}

	if topo != "" {
		labFilter := []string{fmt.Sprintf("%s=%s", labels.Containerlab, c.Config.Name)}
		filters = append(filters, types.FilterFromLabelStrings(labFilter)...)
	}

	resultCollection, err := c.Exec(ctx, execCommands, clab.NewExecOptions(filters))
	if err != nil {
		return err
	}

	switch outputFormat {
	case exec.ExecFormatPlain:
		resultCollection.Log()
	case exec.ExecFormatJSON:
		out, err := resultCollection.Dump(outputFormat)
		if err != nil {
			return fmt.Errorf("failed to print the results collection: %v", err)
		}

		fmt.Println(out)
	}

	return err
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVarP(&execCommands, "cmd", "", []string{}, "command to execute")
	execCmd.Flags().StringSliceVarP(&labelsFilter, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
