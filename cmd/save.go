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

func saveCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "save",
		Short: "save containers configuration",
		Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.dev/cmd/save/ documentation to see the exact command used per node's kind`,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
				return fmt.Errorf("provide topology file path  with --topo flag")
			}
			opts := []clabcore.ClabOption{
				clabcore.WithTimeout(o.Global.Timeout),
				clabcore.WithTopoPath(o.Global.TopologyFile, o.Global.VarsFile),
				clabcore.WithNodeFilter(nodeFilter),
				clabcore.WithRuntime(
					o.Global.Runtime,
					&clabruntime.RuntimeConfig{
						Debug:            o.Global.DebugCount > 0,
						Timeout:          o.Global.Timeout,
						GracefulShutdown: gracefulShutdown,
					},
				),
				clabcore.WithDebug(o.Global.DebugCount > 0),
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

	return c, nil
}
