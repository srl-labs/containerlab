// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func disableTxOffloadCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "disable-tx-offload",
		Short: "disables tx checksum offload on eth0 interface of a container",

		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			ctx := cobraCmd.Context()

			opts := []clabcore.ClabOption{
				clabcore.WithTimeout(o.Global.Timeout),
				clabcore.WithRuntime(
					o.Global.Runtime,
					&clabruntime.RuntimeConfig{
						Debug:            o.Global.DebugCount > 0,
						Timeout:          o.Global.Timeout,
						GracefulShutdown: o.Destroy.GracefulShutdown,
					},
				),
				clabcore.WithDebug(o.Global.DebugCount > 0),
			}
			c, err := clabcore.NewContainerLab(opts...)
			if err != nil {
				return err
			}

			node, err := c.GetNode(o.ToolsTxOffload.ContainerName)
			if err != nil {
				return err
			}

			err = node.ExecFunction(ctx, clabutils.NSEthtoolTXOff(
				o.ToolsTxOffload.ContainerName, "eth0"))
			if err != nil {
				return err
			}

			log.Infof("Tx checksum offload disabled for eth0 interface of %s container", o.ToolsTxOffload.ContainerName)
			return nil
		},
	}

	c.Flags().StringVarP(&o.ToolsTxOffload.ContainerName, "container", "c",
		o.ToolsTxOffload.ContainerName, "container name to disable offload in")

	err := c.MarkFlagRequired("container")
	if err != nil {
		return nil, err
	}

	return c, nil
}
