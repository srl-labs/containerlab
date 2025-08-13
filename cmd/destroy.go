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

var (
	all         bool
	cleanup     bool
	keepMgmtNet bool
	yes         bool
)

// destroyCmd represents the destroy command.
var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Short:   "destroy a lab",
	Long:    "destroy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/destroy/",
	Aliases: []string{"des"},
	PreRunE: clabutils.CheckAndGetRootPrivs,
	RunE:    destroyFn,
}

func init() {
	RootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", false,
		"delete lab directory. Cannot be used with node-filter")
	destroyCmd.Flags().BoolVarP(&gracefulShutdown, "graceful", "", false,
		"attempt to stop containers before removing")
	destroyCmd.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	destroyCmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"auto-approve deletion when used with --all (skips confirmation prompt)")
	destroyCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers deleting nodes")
	destroyCmd.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")
	destroyCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
}

func destroyFn(cobraCmd *cobra.Command, _ []string) error {
	if cleanup && len(nodeFilter) != 0 {
		return fmt.Errorf("cleanup cannot be used with node-filter")
	}

	if all && labName != "" {
		return fmt.Errorf("--all and --name should not be used together")
	}

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(timeout),
		clabcore.WithLabName(labName),
		clabcore.WithRuntime(
			runtime,
			&clabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(debug),
		// during destroy we don't want to check bind paths
		// as it is irrelevant for this command.
		clabcore.WithSkippedBindsPathsCheck(),
	}

	if topoFile != "" {
		opts = append(opts, clabcore.WithTopoPath(topoFile, varsFile))
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
