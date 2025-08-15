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
		PreRunE: clabutils.CheckAndGetRootPrivs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return destroyFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(&cleanup, "cleanup", "c", false,
		"delete lab directory. Cannot be used with node-filter")
	c.Flags().BoolVarP(&gracefulShutdown, "graceful", "", false,
		"attempt to stop containers before removing")
	c.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	c.Flags().BoolVarP(&yes, "yes", "y", false,
		"auto-approve deletion when used with --all (skips confirmation prompt)")
	c.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers deleting nodes")
	c.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")
	c.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")

	return c, nil
}

func destroyFn(cobraCmd *cobra.Command, o *Options) error {
	if cleanup && len(nodeFilter) != 0 {
		return fmt.Errorf("cleanup cannot be used with node-filter")
	}

	if all && o.Global.TopologyName != "" {
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
				GracefulShutdown: gracefulShutdown,
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

	if keepMgmtNet {
		opts = append(opts, clabcore.WithKeepMgmtNet())
	}

	clab, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	destroyOptions := []clabcore.DestroyOption{
		clabcore.WithDestroyMaxWorkers(maxWorkers),
		clabcore.WithDestroyNodeFilter(nodeFilter),
	}

	if keepMgmtNet {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyKeepMgmtNet(),
		)
	}

	if cleanup {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyCleanup(),
		)
	}

	if gracefulShutdown {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyGraceful(),
		)
	}

	if all {
		destroyOptions = append(
			destroyOptions,
			clabcore.WithDestroyAll(),
		)

		if !yes {
			destroyOptions = append(
				destroyOptions,
				clabcore.WithDestroyTerminalPrompt(),
			)
		}
	}

	return clab.Destroy(cobraCmd.Context(), destroyOptions...)
}
