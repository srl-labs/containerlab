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
)

var (
	labels       []string
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

		resultCollection := exec.NewExecCollection()

		for _, node := range c.Nodes {
			for _, execCommand := range execCommands {
				execCmd, err := exec.NewExecCmdFromString(execCommand)
				if err != nil {
					// do not stop exec for other nodes if some failed
					log.Error(err)
				}

				execResult, err := node.RunExec(ctx, execCmd)
				if err != nil {
					// skip nodes that do not support exec
					if err == exec.ErrRunExecNotSupported {
						continue
					}
					return err
				}
				resultCollection.Add(node.Config().ShortName, execResult)
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
	execCmd.Flags().StringSliceVarP(&labels, "label", "", []string{}, "labels to filter container subset")
	execCmd.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")
}
