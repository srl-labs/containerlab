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
		Short: "Save running configuration for all nodes in a lab. It saves the config locally for each OS with an option to copy it to a user-specified directory",
		Long: `Save saves a running configuration as startup config for all nodes in a lab. It saves the config locally for each OS with an option to copy it to a user-specified directory. The exact command that is used to save the config
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

			return c.Save(cobraCmd.Context(), o.ToClabSaveOptions()...)
		},
	}

	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)
	c.Flags().StringVar(
		&o.Save.CopyOut,
		"copy-out",
		"",
		"copy the saved running config to this directory (created if missing; relative to topo dir; when unset, config stays in clab-<labname>/<node>/)",
	)

	return c, nil
}
