// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
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

	return c, nil
}

func execFn(_ *cobra.Command, o *Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
