// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func destroyCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "destroy",
		Short: "destroy a lab",
		Long: "destroy a lab based defined by means of the topology definition file\n" +
			"reference: https://containerlab.dev/cmd/destroy/",
		Aliases: []string{"des"},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return destroyFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(
		&o.Destroy.Cleanup,
		"cleanup",
		"c",
		o.Destroy.Cleanup,
		"delete lab directory. Cannot be used with node-filter",
	)
	c.Flags().BoolVarP(
		&o.Global.GracefulShutdown,
		"graceful",
		"",
		o.Global.GracefulShutdown,
		"attempt to stop containers before removing",
	)
	c.Flags().BoolVarP(
		&o.Destroy.All,
		"all",
		"a",
		o.Destroy.All,
		"destroy all containerlab labs",
	)
	c.Flags().BoolVarP(
		&o.Destroy.AutoApprove,
		"yes",
		"y",
		o.Destroy.AutoApprove,
		"auto-approve deletion when used with --all (skips confirmation prompt)",
	)
	c.Flags().UintVarP(
		&o.Deploy.MaxWorkers,
		"max-workers",
		"",
		o.Deploy.MaxWorkers,
		"limit the maximum number of workers deleting nodes",
	)
	c.Flags().BoolVarP(
		&o.Destroy.KeepManagementNetwork,
		"keep-mgmt-net",
		"",
		o.Destroy.KeepManagementNetwork,
		"do not remove the management network",
	)
	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)

	return c, nil
}

func destroyFn(cobraCmd *cobra.Command, o *Options) error {
	if o.Destroy.Cleanup && len(o.Filter.NodeFilter) != 0 {
		return fmt.Errorf("cleanup cannot be used with node-filter")
	}

	if o.Destroy.All && o.Global.TopologyName != "" {
		return fmt.Errorf("--all and --name should not be used together")
	}

	clabOptions := o.ToClabOptions()

	clabOptions = append(
		clabOptions,
		// during destroy we don't want to check bind paths
		// as it is irrelevant for this command.
		clabcore.WithSkippedBindsPathsCheck(),
	)

	clab, err := clabcore.NewContainerLab(clabOptions...)
	if err != nil {
		return err
	}

	return clab.Destroy(cobraCmd.Context(), o.ToClabDestroyOptions()...)
}
