// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

var (
	NetemNode      string
	NetemInterface string
	NetemDelay     string
	NetemJitter    string
	NetemLoss      float64
	NetemRate      uint64
)

func init() {
	toolsCmd.AddCommand(netemCmd)

	netemCmd.AddCommand(netemSetCmd)
	netemSetCmd.Flags().StringVarP(&NetemNode, "node", "", "", "node to apply impairment to")
	netemSetCmd.Flags().StringVarP(&NetemInterface, "interface", "", "", "interface to apply impairment to")
	netemSetCmd.Flags().StringVarP(&NetemDelay, "delay", "", "0ms", "link receive delay")
	netemSetCmd.Flags().StringVarP(&NetemJitter, "jitter", "", "0ms", "link receive jitter")
	netemSetCmd.Flags().Float64VarP(&NetemLoss, "loss", "", 0, "link receive loss (0 >= rate => 100)")
	netemSetCmd.Flags().Uint64VarP(&NetemRate, "rate", "", 0, "link receive rate in kbit")

	netemSetCmd.MarkFlagRequired("node")
	netemSetCmd.MarkFlagRequired("interface")
}

var netemCmd = &cobra.Command{
	Use:   "netem",
	Short: "link impairment operations",
}

var netemSetCmd = &cobra.Command{
	Use:   "set",
	Short: "set operation",
	RunE:  netemSetFn,
}

func netemSetFn(cmd *cobra.Command, args []string) error {
	// Parse Delay and Jitter to become Durations
	delayDur, err := time.ParseDuration(NetemDelay)
	if err != nil {
		return err
	}
	jitterDur, err := time.ParseDuration(NetemJitter)
	if err != nil {
		return err
	}

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
	nodeNsPath, err := rt.GetNSPath(ctx, NetemNode)
	if err != nil {
		return err
	}

	// get namespace handle
	nshandle, err := netns.GetFromPath(nodeNsPath)
	if err != nil {
		return err
	}
	// get netlink handle for the namespace
	nlHandle, err := netlink.NewHandleAt(nshandle)
	if err != nil {
		return err
	}
	// get the link by name from the namespace
	nlLink, err := nlHandle.LinkByName(NetemInterface)
	if err != nil {
		return err
	}

	// finally set the netem parameters
	nsFd := int(nshandle)
	err = utils.SetDelayJitterLoss(NetemNode, nsFd, nlLink, delayDur, jitterDur, NetemLoss, NetemRate)
	if err != nil {
		return err
	}

	log.Info("Successful")

	return nil
}
