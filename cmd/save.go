// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
)

func saveCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "save",
		Short: "save containers configuration",
		Long: `save performs a configuration save. The exact command that is used to save the config
depends on the node kind. Refer to the https://containerlab.dev/cmd/save/ documentation to see
the exact command used per node's kind`,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
				return fmt.Errorf("provide topology file path  with --topo flag")
			}

			c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
			if err != nil {
				return err
			}

			return c.Save(cobraCmd.Context())
		},
	}

	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)

	return c, nil
}
