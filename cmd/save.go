// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func saveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "save",
		Short: "save containers configuration",
		Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.dev/cmd/save/ documentation to see the exact command used per node's kind`,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			if labName == "" && topoFile == "" {
				return fmt.Errorf("provide topology file path  with --topo flag")
			}
			opts := []clabcore.ClabOption{
				clabcore.WithTimeout(timeout),
				clabcore.WithTopoPath(topoFile, varsFile),
				clabcore.WithNodeFilter(nodeFilter),
				clabcore.WithRuntime(
					runtime,
					&clabruntime.RuntimeConfig{
						Debug:            debug,
						Timeout:          timeout,
						GracefulShutdown: gracefulShutdown,
					},
				),
				clabcore.WithDebug(debug),
			}
			c, err := clabcore.NewContainerLab(opts...)
			if err != nil {
				return err
			}

			return c.Save(cobraCmd.Context())
		},
	}

	c.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")

	return c
}
