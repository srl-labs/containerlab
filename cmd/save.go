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
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/core"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
)

// saveCmd represents the save command.
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.dev/cmd/save/ documentation to see the exact command used per node's kind`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if common.Name == "" && common.Topo == "" {
			return fmt.Errorf("provide topology file path  with --topo flag")
		}
		opts := []core.ClabOption{
			core.WithTimeout(common.Timeout),
			core.WithTopoPath(common.Topo, common.VarsFile),
			core.WithNodeFilter(common.NodeFilter),
			core.WithRuntime(common.Runtime,
				&runtime.RuntimeConfig{
					Debug:            common.Debug,
					Timeout:          common.Timeout,
					GracefulShutdown: common.Graceful,
				},
			),
			core.WithDebug(common.Debug),
		}
		c, err := core.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		err = links.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(len(c.Nodes))
		for _, node := range c.Nodes {
			go func(node nodes.Node) {
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
	saveCmd.Flags().StringSliceVarP(&common.NodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	RootCmd.AddCommand(saveCmd)
}
