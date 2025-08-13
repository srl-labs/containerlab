// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var cntName string

// upgradeCmd represents the version command.
var disableTxOffloadCmd = &cobra.Command{
	Use:   "disable-tx-offload",
	Short: "disables tx checksum offload on eth0 interface of a container",

	PreRunE: clabutils.CheckAndGetRootPrivs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		opts := []clabcore.ClabOption{
			clabcore.WithTimeout(timeout),
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

		node, err := c.GetNode(cntName)
		if err != nil {
			return err
		}

		err = node.ExecFunction(ctx, clabutils.NSEthtoolTXOff(cntName, "eth0"))
		if err != nil {
			return err
		}

		log.Infof("Tx checksum offload disabled for eth0 interface of %s container", cntName)
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(disableTxOffloadCmd)
	disableTxOffloadCmd.Flags().StringVarP(&cntName, "container", "c", "", "container name to disable offload in")
	_ = disableTxOffloadCmd.MarkFlagRequired("container")
}
