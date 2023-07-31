// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/containernetworking/plugins/pkg/ns"
	gotc "github.com/florianl/go-tc"
	log "github.com/sirupsen/logrus"
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
	netemSetCmd.Flags().DurationVarP(&netemDelay, "delay", "", 0*time.Second,
		"time to delay outgoing packets (e.g. 100ms, 2s)")
	netemSetCmd.Flags().DurationVarP(&netemJitter, "jitter", "", 0*time.Second,
		"delay variation, aka jitter (e.g. 50ms)")
	netemSetCmd.Flags().Float64VarP(&netemLoss, "loss-percent", "", 0,
		"random packet loss expressed in percentage (e.g. 0.1 means 0.1%)")
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
	PreRunE: validateInput,
	RunE:    netemSetFn,
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
		link, err := net.InterfaceByName(netemInterface)
		if err != nil {
			return err
		}

		qdisc, err := tc.SetImpairments(tcnl, netemNode, link, netemDelay, netemJitter, netemLoss, netemRate)
		if err != nil {
			return err
		}

		printImpairments(netemInterface, qdisc)

		return nil
	})

	return err
}

func validateInput(cmd *cobra.Command, args []string) error {
	if netemLoss < 0 || netemLoss > 100 {
		return fmt.Errorf("packet loss must be in the range between 0 and 100")
	}

	if netemJitter != 0 && netemDelay == 0 {
		return fmt.Errorf("jitter cannot be set without setting delay")
	}

	return nil
}

func printImpairments(ifName string, qdisc *gotc.Object) {
	columns := []table.Column{
		{Title: "Name", Width: 10},
		{Title: "Delay", Width: 6},
		{Title: "Jitter", Width: 7},
		{Title: "Packet Loss", Width: 14},
		{Title: "Rate (kbit)", Width: 14},
	}

	delay := time.Duration(*qdisc.Netem.Latency64) * time.Nanosecond
	jitter := time.Duration(*qdisc.Netem.Jitter64) * time.Nanosecond
	loss := strconv.FormatFloat(float64(qdisc.Netem.Qopt.Loss)/float64(math.MaxUint32)*100, 'f', 2, 64)
	rate := strconv.Itoa(int(qdisc.Netem.Rate.Rate * 8 / 1000))

	rows := []table.Row{
		{ifName, delay.String(), jitter.String(), loss + "%", rate},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(1), // table is always only 1 row in size
	)

	s := table.Styles{
		Header: lipgloss.NewStyle().Bold(true).Padding(0, 1),
		Cell:   lipgloss.NewStyle().Padding(0, 1),
	}
	t.SetStyles(s)

	fmt.Println(t.View())
}
