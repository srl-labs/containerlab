// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/core"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/utils"
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
	PreRunE: utils.CheckAndGetRootPrivs,
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

	opts := []core.ClabOption{
		core.WithTimeout(timeout),
		core.WithLabName(labName),
		core.WithRuntime(
			runtime,
			&containerlabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		core.WithDebug(debug),
		// during destroy we don't want to check bind paths
		// as it is irrelevant for this command.
		core.WithSkippedBindsPathsCheck(),
	}

	if topoFile != "" {
		opts = append(opts, core.WithTopoPath(topoFile, varsFile))
	}

	if keepMgmtNet {
		opts = append(opts, core.WithKeepMgmtNet())
	}

	clab, err := core.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	destroyOptions := []core.DestroyOption{
		core.WithDestroyMaxWorkers(maxWorkers),
		core.WithDestroyNodeFilter(nodeFilter),
	}

	if keepMgmtNet {
		destroyOptions = append(
			destroyOptions,
			core.WithDestroyKeepMgmtNet(),
		)
	}

	if cleanup {
		destroyOptions = append(
			destroyOptions,
			core.WithDestroyCleanup(),
		)
	}

	if gracefulShutdown {
		destroyOptions = append(
			destroyOptions,
			core.WithDestroyGraceful(),
		)
	}

	if all {
		destroyOptions = append(
			destroyOptions,
			core.WithDestroyAll(),
		)

		if !yes {
			destroyOptions = append(
				destroyOptions,
				core.WithDestroyTerminalPrompt(),
			)
		}
	}

	return clab.DestroyNew(cobraCmd.Context(), destroyOptions...)
}
