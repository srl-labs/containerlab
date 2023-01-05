// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(execCommands) == 0 {
			return errors.New("provide command to execute")
		}

		outputFormat, err := exec.ParseExecOutputFormat(execFormat)
		if err != nil {
			return err
		}

		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo, varsFile),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if name == "" {
			name = c.Config.Name
		}

		filters := []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
		filters = append(filters, types.FilterFromLabelStrings(labelsFilter)...)

		// list all containers matching the filters
		gcl, err := c.ListContainers(ctx, filters)
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
				// do not stop exec for other nodes if some failed
				log.Error(err)
			}
			execCmds = append(execCmds, execCmd)
		}

		// run the exec commands on all the cotnainers matching the filter
		for _, gc := range gcl {
			// iterate over the commands
			for _, execCmd := range execCmds {
				// execute the commands
				execResult, err := gc.RunExec(ctx, execCmd)
				if err != nil {
					// skip nodes that do not support exec
					if err == exec.ErrRunExecNotSupported {
						continue
					}
				}
				resultCollection.Add(gc.Names[0], execResult)
			}
		}

		output, err := resultCollection.Dump(outputFormat)
		if err != nil {
			return err
		}
		fmt.Println(output)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVarP(&execCommands, "cmd", "", []string{}, "command to execute")
	execCmd.Flags().StringSliceVarP(&labelsFilter, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
