// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/core"
	"github.com/srl-labs/containerlab/exec"
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
	Use:   "exec",
	Short: "execute a command on one or multiple containers",
	RunE:  execFn,
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

	opts := make([]core.ClabOption, 0, 5)

	// exec can work with or without a topology file
	// when topology file is provided we need to parse it
	// when topo file is not provided, we rely on labels to perform the filtering
	if common.Topo != "" {
		opts = append(opts, core.WithTopoPath(common.Topo, common.VarsFile))
	}

	opts = append(opts,
		core.WithTimeout(common.Timeout),
		core.WithRuntime(common.Runtime,
			&runtime.RuntimeConfig{
				Debug:            common.Debug,
				Timeout:          common.Timeout,
				GracefulShutdown: common.Graceful,
			},
		),
		core.WithDebug(common.Debug),
	)

	if common.Name != "" {
		opts = append(opts, core.WithLabName(common.Name))
	}

	c, err := core.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	var filters []*types.GenericFilter

	if len(labelsFilter) != 0 {
		filters = types.FilterFromLabelStrings(labelsFilter)
	}

	if common.Topo != "" {
		labFilter := []string{fmt.Sprintf("%s=%s", labels.Containerlab, c.Config.Name)}
		filters = append(filters, types.FilterFromLabelStrings(labFilter)...)
	}

	resultCollection, err := c.Exec(ctx, execCommands, core.NewExecOptions(filters))
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
	RootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVarP(&execCommands, "cmd", "", []string{}, "command to execute")
	execCmd.Flags().StringSliceVarP(&labelsFilter, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
