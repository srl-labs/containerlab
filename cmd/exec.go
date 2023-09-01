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
	"github.com/srl-labs/containerlab/links"
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
	if len(execCommands) == 0 {
		return errors.New("provide command to execute")
	}

	outputFormat, err := exec.ParseExecOutputFormat(execFormat)
	if err != nil {
		return err
	}

	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
		clab.WithTopoPath(topo, varsFile),
		clab.WithNodeFilter(nodeFilter),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
		clab.WithDebug(debug),
	}
	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	err = links.SetMgmtNetUnderlayingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if name == "" {
		name = c.Config.Name
	}

	var filters []*types.GenericFilter
	switch {
	case len(labelsFilter) != 0:
		filters = types.FilterFromLabelStrings(labelsFilter)
	default:
		// when user-defined labels are not provided we should filter the nodes of the lab
		labFilter := []string{fmt.Sprintf("%s=%s", labels.Containerlab, c.Config.Name)}
		filters = types.FilterFromLabelStrings(labFilter)
	}

	// list all containers using global runtime using provided filters
	cnts, err := c.GlobalRuntime().ListContainers(ctx, filters)
	if err != nil {
		return err
	}

	// prepare the exec collection and the exec command
	resultCollection := exec.NewExecCollection()

	// build execs from the string input
	var execCmds []*exec.ExecCmd
	for _, execCmdStr := range execCommands {
		execCmd, err := exec.NewExecCmdFromString(execCmdStr)
		if err != nil {
			return err
		}
		execCmds = append(execCmds, execCmd)
	}

	// run the exec commands on all the containers matching the filter
	for _, cnt := range cnts {
		// iterate over the commands
		for _, execCmd := range execCmds {
			// execute the commands
			execResult, err := cnt.RunExec(ctx, execCmd)
			if err != nil {
				// skip nodes that do not support exec
				if err == exec.ErrRunExecNotSupported {
					continue
				}
			}
			resultCollection.Add(cnt.Names[0], execResult)
		}
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
