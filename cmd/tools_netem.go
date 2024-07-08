// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	gotc "github.com/florianl/go-tc"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/internal/tc"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/vishvananda/netlink"
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
	netemSetCmd.Flags().DurationVarP(&netemDelay, "delay", "", 0*time.Second,
		"time to delay outgoing packets (e.g. 100ms, 2s)")
	netemSetCmd.Flags().DurationVarP(&netemJitter, "jitter", "", 0*time.Second,
		"delay variation, aka jitter (e.g. 50ms)")
	netemSetCmd.Flags().Float64VarP(&netemLoss, "loss", "", 0,
		"random packet loss expressed in percentage (e.g. 0.1 means 0.1%)")
	netemSetCmd.Flags().Uint64VarP(&netemRate, "rate", "", 0, "link rate limit in kbit")

	netemSetCmd.MarkFlagRequired("node")
	netemSetCmd.MarkFlagRequired("interface")

	netemCmd.AddCommand(netemShowCmd)
	netemShowCmd.Flags().StringVarP(&netemNode, "node", "n", "", "node to apply impairment to")
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
	PreRunE: validateInput,
	RunE:    netemSetFn,
}

var netemShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show link impairments for a node",
	RunE:  netemShowFn,
}

func netemSetFn(_ *cobra.Command, _ []string) error {
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

	tcnl, err := tc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		netemIfLink, err := netlink.LinkByName(links.SanitiseInterfaceName(netemInterface))
		if err != nil {
			return err
		}

		netemIfName := netemIfLink.Attrs().Name
		link, err := net.InterfaceByName(netemIfName)
		if err != nil {
			return err
		}

		qdisc, err := tc.SetImpairments(tcnl, netemNode, link, netemDelay, netemJitter, netemLoss, netemRate)
		if err != nil {
			return err
		}

		printImpairments([]gotc.Object{*qdisc})

		return nil
	})

	return err
}

func validateInput(_ *cobra.Command, _ []string) error {
	if netemLoss < 0 || netemLoss > 100 {
		return fmt.Errorf("packet loss must be in the range between 0 and 100")
	}

	if netemJitter != 0 && netemDelay == 0 {
		return fmt.Errorf("jitter cannot be set without setting delay")
	}

	return nil
}

func printImpairments(qdiscs []gotc.Object) {
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{
		"Interface",
		"Delay",
		"Jitter",
		"Packet Loss",
		"Rate (kbit)",
	}

	table.SetHeader(header)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)

	var rows [][]string

	for _, qdisc := range qdiscs {
		rows = append(rows, qdiscToTableData(qdisc))
	}

	table.AppendBulk(rows)
	table.Render()
}

func qdiscToTableData(qdisc gotc.Object) []string {
	link, err := netlink.LinkByIndex(int(qdisc.Ifindex))
	if err != nil {
		log.Errorf("could not get netlink interface by index: %v", err)
	}

	var delay, jitter, loss, rate string

	ifDisplayName := link.Attrs().Name
	if link.Attrs().Alias != "" {
		ifDisplayName += fmt.Sprintf(" (%s)", link.Attrs().Alias)
	}

	// return N/A values when netem is not set
	// which is the case when qdisc is not set for an interface
	if qdisc.Netem == nil {
		return []string{
			ifDisplayName,
			"N/A", // delay
			"N/A", // jitter
			"N/A", // loss
			"N/A", // rate
		}
	}

	if qdisc.Netem.Latency64 != nil {
		delay = (time.Duration(*qdisc.Netem.Latency64) * time.Nanosecond).String()
	}

	if qdisc.Netem.Jitter64 != nil {
		jitter = (time.Duration(*qdisc.Netem.Jitter64) * time.Nanosecond).String()
	}

	loss = strconv.FormatFloat(float64(qdisc.Netem.Qopt.Loss)/float64(math.MaxUint32)*100, 'f', 2, 64) + "%"
	rate = strconv.Itoa(int(qdisc.Netem.Rate.Rate * 8 / 1000))

	return []string{
		ifDisplayName,
		delay,
		jitter,
		loss,
		rate,
	}
}

func netemShowFn(_ *cobra.Command, _ []string) error {
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

	tcnl, err := tc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		qdiscs, err := tc.Impairments(tcnl)
		if err != nil {
			return err
		}

		printImpairments(qdiscs)

		return nil
	})

	return err
}
