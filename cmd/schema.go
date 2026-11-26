// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/spf13/cobra"
	clabschemas "github.com/srl-labs/containerlab/schemas"
)

func schemaCmd(_ *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "schema",
		Short: "print the JSON schema of the topology file",
		Long: "print the JSON schema of the topology file of this containerlab version" +
			"\nreference: https://containerlab.dev/cmd/schema/",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			_, err := cobraCmd.OutOrStdout().Write(clabschemas.Topology())
			return err
		},
	}

	return c, nil
}
