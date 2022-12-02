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
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	labels      []string
	execFormat  string
	execCommand string
)

// execCmd represents the exec command.
var execCmd = &cobra.Command{
	Use:     "exec",
	Short:   "execute a command on one or multiple containers",
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		if execCommand == "" {
			return errors.New("provide command to execute")
		}

		switch execFormat {
		case "json", "plain":
			// expected values, go on
		default:
			return errors.New("format is expected to be either json or plain")
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

		resultCollection := types.NewExecCollection()

		for _, node := range c.Nodes {
			exec, err := types.NewExec(execCommand)
			if err != nil {
				return err
			}
			execResult, err := node.RunExecType(ctx, exec)
			if err != nil {
				return err
			}
			resultCollection.Add(node.Config().ShortName, execResult)
		}

		if execFormat == string(types.ExecFormatJSON) {
			fmt.Println(resultCollection.GetInFormat(types.ExecFormatJSON))
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVarP(&execCommand, "cmd", "", "", "command to execute")
	execCmd.Flags().StringSliceVarP(&labels, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
