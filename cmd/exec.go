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

func execCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "exec",
		Short: "execute a command in one or multiple containers",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return execFn(cobraCmd, o)
		},
	}

	c.Flags().StringArrayVarP(&execCommands, "cmd", "", []string{}, "command to execute")
	c.Flags().StringSliceVarP(&labelsFilter, "label", "", []string{}, "labels to filter container subset")
	c.Flags().StringVarP(&execFormat, "format", "f", "plain", "output format. One of [json, plain]")

	return c, nil
}

func execFn(_ *cobra.Command, o *Options) error {
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
	if o.Global.TopologyFile != "" {
		opts = append(opts, clabcore.WithTopoPath(o.Global.TopologyFile, o.Global.VarsFile))
	}

	opts = append(opts,
		clabcore.WithTimeout(o.Global.Timeout),
		clabcore.WithRuntime(
			o.Global.Runtime,
			&clabruntime.RuntimeConfig{
				Debug:            o.Global.DebugCount > 0,
				Timeout:          o.Global.Timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(o.Global.DebugCount > 0),
	)

	if o.Global.TopologyName != "" {
		opts = append(opts, clabcore.WithLabName(o.Global.TopologyName))
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

	if o.Global.TopologyFile != "" {
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
