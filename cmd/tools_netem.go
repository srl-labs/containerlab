// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"net"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/internal/tc"
	"github.com/srl-labs/containerlab/runtime"
)

var (
	netemNode      string
	netemInterface string
	netemDelay     time.Duration
	netemJitter    time.Duration
	netemLoss      float64
	netemRate      uint64
)

func init() {
	toolsCmd.AddCommand(netemCmd)

	netemCmd.AddCommand(netemSetCmd)
	netemSetCmd.Flags().StringVarP(&netemNode, "node", "n", "", "node to apply impairment to")
	netemSetCmd.Flags().StringVarP(&netemInterface, "interface", "i", "", "interface to apply impairment to")
	netemSetCmd.Flags().DurationVarP(&netemDelay, "delay", "", 0*time.Second, "time to delay outgoing packets (e.g. 100ms, 2s)")
	netemSetCmd.Flags().DurationVarP(&netemJitter, "jitter", "", 0*time.Second, "delay variation, aka jitter (e.g. 50ms)")
	netemSetCmd.Flags().Float64VarP(&netemLoss, "loss-percent", "", 0, "random packet loss expressed in percentage (e.g. 0.1 means 0.1%)")
	netemSetCmd.Flags().Uint64VarP(&netemRate, "rate", "", 0, "link rate limit in kbit")

	netemSetCmd.MarkFlagRequired("node")
	netemSetCmd.MarkFlagRequired("interface")
}

var netemCmd = &cobra.Command{
	Use:   "netem",
	Short: "link impairment operations",
}

var netemSetCmd = &cobra.Command{
	Use:   "set",
	Short: "set link impairments",
	Long: `The netem queue discipline provides Network Emulation
functionality for testing protocols by emulating the properties
of real-world networks.`,
	RunE: netemSetFn,
}

func netemSetFn(cmd *cobra.Command, args []string) error {
	// Get the runtime initializer.
	_, rinit, err := clab.RuntimeInitializer(rt)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	// init runtime with timeout
	err = rt.Init(
		runtime.WithConfig(
			&runtime.RuntimeConfig{
				Timeout: timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the containers NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, netemNode)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS

	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	err = nodeNs.Do(func(_ ns.NetNS) error {
		link, err := net.InterfaceByName(netemInterface)
		if err != nil {
			return err
		}

		err = tc.SetImpairments(netemNode, int(nodeNs.Fd()), link, netemDelay, netemJitter, netemLoss, netemRate)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}
