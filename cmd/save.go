// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// saveCmd represents the save command.
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.dev/cmd/save/ documentation to see the exact command used per node's kind`,
	RunE: func(_ *cobra.Command, _ []string) error {
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

		err = clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(len(c.Nodes))
		for _, node := range c.Nodes {
			go func(node clabnodes.Node) {
				defer wg.Done()

				err := node.SaveConfig(ctx)
				if err != nil {
					log.Errorf("err: %v", err)
				}
			}(node)
		}
		wg.Wait()

		return nil
	},
}

func init() {
	saveCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	RootCmd.AddCommand(saveCmd)
}
