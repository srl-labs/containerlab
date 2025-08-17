// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func destroyCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:     "destroy",
		Short:   "destroy a lab",
		Long:    "destroy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/destroy/",
		Aliases: []string{"des"},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return destroyFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(&o.Destroy.Cleanup, "cleanup", "c", o.Destroy.Cleanup,
		"delete lab directory. Cannot be used with node-filter")
	c.Flags().BoolVarP(&o.Destroy.GracefulShutdown, "graceful", "", o.Destroy.GracefulShutdown,
		"attempt to stop containers before removing")
	c.Flags().BoolVarP(&o.Destroy.All, "all", "a", o.Destroy.All, "destroy all containerlab labs")
	c.Flags().BoolVarP(&o.Destroy.AutoApprove, "yes", "y", o.Destroy.AutoApprove,
		"auto-approve deletion when used with --all (skips confirmation prompt)")
	c.Flags().UintVarP(&o.Deploy.MaxWorkers, "max-workers", "", o.Deploy.MaxWorkers,
		"limit the maximum number of workers deleting nodes")
	c.Flags().BoolVarP(&o.Destroy.KeepManagementNetwork, "keep-mgmt-net", "",
		o.Destroy.KeepManagementNetwork, "do not remove the management network")
	c.Flags().StringSliceVarP(&o.Filter.NodeFilter, "node-filter", "", o.Filter.NodeFilter,
		"comma separated list of nodes to include")

	return c, nil
}

func destroyFn(cobraCmd *cobra.Command, o *Options) error {
	if o.Destroy.Cleanup && len(o.Filter.NodeFilter) != 0 {
		return fmt.Errorf("cleanup cannot be used with node-filter")
	}

	if o.Destroy.All && o.Global.TopologyName != "" {
		return fmt.Errorf("--all and --name should not be used together")
	}

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(o.Global.Timeout),
		clabcore.WithLabName(o.Global.TopologyName),
		clabcore.WithRuntime(
			o.Global.Runtime,
			&clabruntime.RuntimeConfig{
				Debug:            o.Global.DebugCount > 0,
				Timeout:          o.Global.Timeout,
				GracefulShutdown: o.Destroy.GracefulShutdown,
			},
		),
		clabcore.WithDebug(o.Global.DebugCount > 0),
		// during destroy we don't want to check bind paths
		// as it is irrelevant for this command.
		clabcore.WithSkippedBindsPathsCheck(),
	}

	if o.Global.TopologyFile != "" {
		opts = append(opts, clabcore.WithTopoPath(o.Global.TopologyFile, o.Global.VarsFile))
	}

	if o.Destroy.KeepManagementNetwork {
		opts = append(opts, clabcore.WithKeepMgmtNet())
	}

	clab, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	destroyOptions := []clabcore.DestroyOption{
		clabcore.WithDestroyMaxWorkers(o.Deploy.MaxWorkers),
		clabcore.WithDestroyNodeFilter(o.Filter.NodeFilter),
	}

	if o.Destroy.KeepManagementNetwork {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyKeepMgmtNet(),
		)
	}

	if o.Destroy.Cleanup {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyCleanup(),
		)
	}

	if o.Destroy.GracefulShutdown {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyGraceful(),
		)
	}

	if o.Destroy.All {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyAll(),
		)

		if !o.Destroy.AutoApprove {
			destroyOptions = append(
				destroyOptions,
				clabcore.WithDestroyTerminalPrompt(),
			)
		}
	}

	return clab.Destroy(cobraCmd.Context(), destroyOptions...)
}
