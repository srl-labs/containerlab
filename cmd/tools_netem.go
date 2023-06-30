// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"time"

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
	NetemDelay     int
	NetemJitter    int
	NetemLoss      uint
)

func init() {
	toolsCmd.AddCommand(netemCmd)
	netemCmd.AddCommand(netemSetCmd)
	netemSetCmd.Flags().StringVarP(&NetemNode, "node", "", "", "node to apply qdisc to")
	netemSetCmd.Flags().StringVarP(&NetemInterface, "interface", "", "", "interface to apply qdsic to")
	netemSetCmd.Flags().IntVarP(&NetemDelay, "delay", "", 0, "link receive delay")
	netemSetCmd.Flags().IntVarP(&NetemJitter, "jitter", "", 0, "link receive jitter")
	netemSetCmd.Flags().UintVarP(&NetemLoss, "loss", "", 0, "link receive loss")
}

var netemCmd = &cobra.Command{
	Use:   "netem",
	Short: "netem operations",
}

var netemSetCmd = &cobra.Command{
	Use:   "set",
	Short: "set operation",
	RunE: func(cmd *cobra.Command, args []string) error {

		if NetemNode == "" {
			return fmt.Errorf("define a node to work on via --node")
		}

		var err error
		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
			clab.WithDebug(debug),
			clab.WithTopoFile(topo, varsFile),
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cnts, err := c.Nodes[NetemNode].GetContainers(ctx)
		if err != nil {
			return err
		}
		if len(cnts) > 1 {
			return fmt.Errorf("retrieved found more then one container for %q unable to apply netem qdisc", NetemNode)
		}
		cnt := cnts[0]

		delay := time.Duration(NetemDelay) * time.Millisecond
		jitter := time.Duration(NetemJitter) * time.Millisecond

		// get namespace handle
		nshandle, err := netns.GetFromPid(cnt.Pid)
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
		err = utils.SetDelayJitterLoss(cnt.Pid, nlLink, &delay, &jitter, &NetemLoss)
		if err != nil {
			return err
		}

		return nil
	},
}
