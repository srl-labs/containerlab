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

func disableTxOffloadCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "disable-tx-offload",
		Short: "disables tx checksum offload on eth0 interface of a container",

		PreRunE: clabutils.CheckAndGetRootPrivs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			opts := []clabcore.ClabOption{
				clabcore.WithTimeout(o.Global.Timeout),
				clabcore.WithRuntime(
					o.Global.Runtime,
					&clabruntime.RuntimeConfig{
						Debug:            o.Global.DebugCount > 0,
						Timeout:          o.Global.Timeout,
						GracefulShutdown: gracefulShutdown,
					},
				),
				clabcore.WithDebug(o.Global.DebugCount > 0),
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

	c.Flags().StringVarP(&cntName, "container", "c", "", "container name to disable offload in")

	err := c.MarkFlagRequired("container")
	if err != nil {
		return nil, err
	}

	return c, nil
}
