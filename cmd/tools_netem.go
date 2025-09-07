// Copyright 2023 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	gotc "github.com/florianl/go-tc"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltc "github.com/srl-labs/containerlab/internal/tc"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

const (
	msPerSec = 1_000
)

func netemCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   "netem",
		Short: "link impairment operations",
	}

	netemSetCmd := &cobra.Command{
		Use:   "set",
		Short: "set link impairments",
		Long: `The netem queue discipline provides Network Emulation
functionality for testing protocols by emulating the properties
of real-world networks.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateInputAndRoot(o)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return netemSetFn(o)
		},
	}

	c.AddCommand(netemSetCmd)
	netemSetCmd.Flags().StringVarP(
		&o.ToolsNetem.ContainerName,
		"node",
		"n",
		o.ToolsNetem.ContainerName,
		"node to apply impairment to",
	)
	netemSetCmd.Flags().StringVarP(
		&o.ToolsNetem.Interface,
		"interface",
		"i",
		o.ToolsNetem.Interface,
		"interface to apply impairment to",
	)
	netemSetCmd.Flags().DurationVarP(
		&o.ToolsNetem.Delay,
		"delay",
		"",
		o.ToolsNetem.Delay,
		"time to delay outgoing packets (e.g. 100ms, 2s)",
	)
	netemSetCmd.Flags().DurationVarP(
		&o.ToolsNetem.Jitter,
		"jitter",
		"",
		o.ToolsNetem.Jitter,
		"delay variation, aka jitter (e.g. 50ms)",
	)
	netemSetCmd.Flags().Float64VarP(
		&o.ToolsNetem.Loss,
		"loss",
		"",
		o.ToolsNetem.Loss,
		"random packet loss expressed in percentage (e.g. 0.1 means 0.1%)",
	)
	netemSetCmd.Flags().Uint64VarP(
		&o.ToolsNetem.Rate,
		"rate",
		"",
		o.ToolsNetem.Rate, "link rate limit in kbit")
	netemSetCmd.Flags().Float64VarP(
		&o.ToolsNetem.Corruption,
		"corruption",
		"",
		0,
		"random packet corruption probability expressed in percentage (e.g. 0.1 means 0.1%)",
	)
	netemSetCmd.MarkFlagRequired("node")
	netemSetCmd.MarkFlagRequired("interface")

	netemShowCmd := &cobra.Command{
		Use:   "show",
		Short: "show link impairments for a node",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateInputAndRoot(o)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return netemShowFn(o)
		},
	}
	c.AddCommand(netemShowCmd)
	netemShowCmd.Flags().StringVarP(
		&o.ToolsNetem.ContainerName,
		"node",
		"n",
		o.ToolsNetem.ContainerName,
		"node to apply impairment to",
	)
	netemShowCmd.Flags().StringVarP(
		&o.ToolsNetem.Format,
		"format",
		"f",
		o.ToolsNetem.Format,
		"output format (table, json)",
	)

	netemResetCmd := &cobra.Command{
		Use:   "reset",
		Short: "reset link impairments",
		Long:  `Reset network impairments by deleting the netem qdisc from the specified interface.`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateInputAndRoot(o)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return netemResetFn(o)
		},
	}
	c.AddCommand(netemResetCmd)
	netemResetCmd.Flags().StringVarP(&o.ToolsNetem.ContainerName, "node", "n",
		o.ToolsNetem.ContainerName, "node to reset impairment on")
	netemResetCmd.Flags().StringVarP(&o.ToolsNetem.Interface, "interface", "i",
		o.ToolsNetem.Interface, "interface to reset impairment on")
	netemResetCmd.MarkFlagRequired("node")
	netemResetCmd.MarkFlagRequired("interface")

	return c, nil
}

func netemSetFn(o *Options) error {
	// Ensure that the sch_netem kernel module is loaded (for Fedora/RHEL compatibility)
	if err := exec.Command("modprobe", "sch_netem").Run(); err != nil {
		log.Warn("failed to load sch_netem kernel module (expected on OrbStack machines)", "err", err)
	}

	// Get the runtime initializer.
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	// init runtime with timeout
	err = rt.Init(
		clabruntime.WithConfig(
			&clabruntime.RuntimeConfig{
				Timeout: o.Global.Timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the containers NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS

	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	tcnl, err := clabinternaltc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		netemIfLink, err := netlink.LinkByName(
			clablinks.SanitizeInterfaceName(o.ToolsNetem.Interface))
		if err != nil {
			return err
		}

		netemIfName := netemIfLink.Attrs().Name

		link, err := net.InterfaceByName(netemIfName)
		if err != nil {
			return err
		}

		qdisc, err := clabinternaltc.SetImpairments(
			tcnl,
			o.ToolsNetem.ContainerName,
			link,
			o.ToolsNetem.Delay,
			o.ToolsNetem.Jitter,
			o.ToolsNetem.Loss,
			o.ToolsNetem.Rate,
			o.ToolsNetem.Corruption,
		)
		if err != nil {
			return err
		}

		printImpairments([]gotc.Object{*qdisc})

		return nil
	})

	return err
}

func validateInputAndRoot(o *Options) error {
	if o.ToolsNetem.Loss < 0 || o.ToolsNetem.Loss > 100 {
		return fmt.Errorf("packet loss must be in the range between 0 and 100")
	}

	if o.ToolsNetem.Jitter != 0 && o.ToolsNetem.Delay == 0 {
		return fmt.Errorf("jitter cannot be set without setting delay")
	}

	clabutils.CheckAndGetRootPrivs()

	return nil
}

func printImpairments(qdiscs []gotc.Object) {
	table := tableWriter.NewWriter()
	table.SetOutputMirror(os.Stdout)
	table.SetStyle(tableWriter.StyleRounded)
	table.Style().Format.Header = text.FormatTitle
	table.Style().Format.HeaderAlign = text.AlignCenter
	table.Style().Color = tableWriter.ColorOptions{
		Header: text.Colors{text.Bold},
	}

	header := tableWriter.Row{
		"Interface",
		"Delay",
		"Jitter",
		"Packet Loss",
		"Rate (kbit)",
		"Corruption",
	}

	table.AppendHeader(header)

	rows := make([]tableWriter.Row, len(qdiscs))

	for idx := range qdiscs {
		rows[idx] = qdiscToTableData(&qdiscs[idx])
	}

	table.AppendRows(rows)
	table.Render()
}

func qdiscToTableData(qdisc *gotc.Object) tableWriter.Row {
	link, err := netlink.LinkByIndex(int(qdisc.Ifindex))
	if err != nil {
		log.Errorf("could not get netlink interface by index: %v", err)
	}

	var delay, jitter, loss, rate, corruption string

	ifDisplayName := link.Attrs().Name
	if link.Attrs().Alias != "" {
		ifDisplayName += fmt.Sprintf(" (%s)", link.Attrs().Alias)
	}

	// return N/A values when netem is not set
	// which is the case when qdisc is not set for an interface
	if qdisc.Netem == nil {
		return tableWriter.Row{
			ifDisplayName,
			clabconstants.NotApplicable, // delay
			clabconstants.NotApplicable, // jitter
			clabconstants.NotApplicable, // loss
			clabconstants.NotApplicable, // rate
			clabconstants.NotApplicable, // corruption
		}
	}

	if qdisc.Netem.Latency64 != nil {
		delay = (time.Duration(*qdisc.Netem.Latency64) * time.Nanosecond).String()
	}

	if qdisc.Netem.Jitter64 != nil {
		jitter = (time.Duration(*qdisc.Netem.Jitter64) * time.Nanosecond).String()
	}

	loss = strconv.FormatFloat(
		float64(qdisc.Netem.Qopt.Loss)/float64(math.MaxUint32)*100, 'f', 2, 64,
	) + "%"
	rate = strconv.Itoa(int(qdisc.Netem.Rate.Rate * 8 / msPerSec))
	corruption = strconv.FormatFloat(float64(qdisc.Netem.Corrupt.Probability)/
		float64(math.MaxUint32)*100, 'f', 2, 64) + "%"

	return tableWriter.Row{
		ifDisplayName,
		delay,
		jitter,
		loss,
		rate,
		corruption,
	}
}

// qdiscToJSONData converts the full qdisc object to a simplified view.
func qdiscToJSONData(qdisc *gotc.Object) clabtypes.ImpairmentData {
	link, err := netlink.LinkByIndex(int(qdisc.Ifindex))
	if err != nil {
		log.Errorf("could not get netlink interface by index: %v", err)
	}

	var delay, jitter string

	var loss, corruption float64

	var rate int

	ifDisplayName := link.Attrs().Name
	if link.Attrs().Alias != "" {
		ifDisplayName += fmt.Sprintf(" (%s)", link.Attrs().Alias)
	}

	// Return "N/A" values when netem is not set.
	if qdisc.Netem == nil {
		return clabtypes.ImpairmentData{
			Interface: ifDisplayName,
		}
	}

	if qdisc.Netem.Latency64 != nil && *qdisc.Netem.Latency64 != 0 {
		delay = (time.Duration(*qdisc.Netem.Latency64) * time.Nanosecond).String()
	}

	if qdisc.Netem.Jitter64 != nil && *qdisc.Netem.Jitter64 != 0 {
		jitter = (time.Duration(*qdisc.Netem.Jitter64) * time.Nanosecond).String()
	}

	if qdisc.Netem.Rate != nil && int(qdisc.Netem.Rate.Rate) != 0 {
		rate = int(qdisc.Netem.Rate.Rate * 8 / msPerSec)
	}

	if qdisc.Netem.Corrupt != nil && qdisc.Netem.Corrupt.Probability != 0 {
		// round to 2 decimal places
		corruption = math.Round((float64(qdisc.Netem.Corrupt.Probability)/
			float64(math.MaxUint32)*100)*100) / 100 //nolint: mnd
	}

	if qdisc.Netem.Qopt.Loss != 0 {
		// round to 2 decimal places
		loss = math.Round(
			(float64(qdisc.Netem.Qopt.Loss)/float64(math.MaxUint32)*100)*100) / 100 //nolint: mnd
	}

	return clabtypes.ImpairmentData{
		Interface:  ifDisplayName,
		Delay:      delay,
		Jitter:     jitter,
		PacketLoss: loss,
		Rate:       rate,
		Corruption: corruption,
	}
}

func netemShowFn(o *Options) error {
	// Get the runtime initializer.
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	err = rt.Init(
		clabruntime.WithConfig(
			&clabruntime.RuntimeConfig{
				Timeout: o.Global.Timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the container's NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS
	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	tcnl, err := clabinternaltc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		qdiscs, err := clabinternaltc.Impairments(tcnl)
		if err != nil {
			return err
		}

		if o.ToolsNetem.Format == clabconstants.FormatJSON {
			var impairments []clabtypes.ImpairmentData

			for idx := range qdiscs {
				if qdiscs[idx].Attribute.Kind != "netem" {
					continue // skip clsact or other qdisc types
				}

				impairments = append(impairments, qdiscToJSONData(&qdiscs[idx]))
			}

			// Structure output as a map keyed by the node name.
			outputData := map[string][]clabtypes.ImpairmentData{
				o.ToolsNetem.ContainerName: impairments,
			}

			jsonData, err := json.MarshalIndent(outputData, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling JSON: %v", err)
			}

			fmt.Println(string(jsonData))
		} else {
			printImpairments(qdiscs)
		}

		return nil
	})

	return err
}

func netemResetFn(o *Options) error {
	// Get the runtime initializer.
	_, rinit, err := clabcore.RuntimeInitializer(o.Global.Runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	err = rt.Init(
		clabruntime.WithConfig(
			&clabruntime.RuntimeConfig{
				Timeout: o.Global.Timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the container's NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS
	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	tcnl, err := clabinternaltc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		netemIfLink, err := netlink.LinkByName(
			clablinks.SanitizeInterfaceName(o.ToolsNetem.Interface))
		if err != nil {
			return err
		}
		// Retrieve the standard net.Interface from the netlink.Link name.
		netemIfIface, err := net.InterfaceByName(netemIfLink.Attrs().Name)
		if err != nil {
			return err
		}

		if err := clabinternaltc.DeleteImpairments(tcnl, netemIfIface); err != nil {
			return err
		}

		fmt.Printf("Reset impairments on node %q, interface %q\n",
			o.ToolsNetem.ContainerName, netemIfLink.Attrs().Name)

		return nil
	})

	return err
}
