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
	clabnetem "github.com/srl-labs/containerlab/netem"
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
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return netemSetFn(cobraCmd.Context(), o)
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

func netemSetFn(ctx context.Context, o *Options) error {
	// Ensure that the sch_netem kernel module is loaded (for Fedora/RHEL compatibility)
	if err := exec.CommandContext(ctx, "modprobe", "sch_netem").Run(); err != nil {
		log.Warn(
			"failed to load sch_netem kernel module (expected on OrbStack machines)",
			"err",
			err,
		)
	}

	node, err := clabcore.ResolveNetemNode(ctx, o.Global.Runtime, o.Global.Timeout, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	target, err := node.TargetFor(o.ToolsNetem.Interface)
	if err != nil {
		return err
	}

	nodeNs, err := ns.GetNS(target.NSPath)
	if err != nil {
		return err
	}

	tcnl, err := clabnetem.NewTC(int(nodeNs.Fd()))
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
			clabutils.SanitizeInterfaceName(target.Iface))
		if err != nil {
			return err
		}

		netemIfName := netemIfLink.Attrs().Name

		link, err := net.InterfaceByName(netemIfName)
		if err != nil {
			return err
		}

		qdisc, err := clabnetem.SetImpairments(
			tcnl,
			target.DisplayName,
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

		printImpairments([]tableWriter.Row{qdiscToTableData(qdisc)})

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

	if err := clabutils.CheckAndGetRootPrivs(); err != nil {
		return err
	}

	return nil
}

func printImpairments(rows []tableWriter.Row) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, err := clabcore.ResolveNetemNode(ctx, o.Global.Runtime, o.Global.Timeout, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	nodeNs, err := ns.GetNS(node.NSPath)
	if err != nil {
		return err
	}

	tcnl, err := clabnetem.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v", err)
		}
	}()

	jsonFormat := o.ToolsNetem.Format == clabconstants.FormatJSON

	var impairments []clabtypes.ImpairmentData

	var tableRows []tableWriter.Row

	var ifaceNames []string

	err = nodeNs.Do(func(_ ns.NetNS) error {
		qdiscs, err := clabnetem.Impairments(tcnl)
		if err != nil {
			return err
		}

		for idx := range qdiscs {
			if jsonFormat {
				if qdiscs[idx].Attribute.Kind != "netem" {
					continue // skip clsact or other qdisc types
				}

				impairments = append(impairments, qdiscToJSONData(&qdiscs[idx]))
			} else {
				tableRows = append(tableRows, qdiscToTableData(&qdiscs[idx]))
			}
		}

		// Tools-interface lookups run in the host netns, outside this closure.
		// A tools interface is published under the interface's topology-facing
		// name: the alias when a kind remapped the interface name.
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, l := range links {
			name := l.Attrs().Alias
			if name == "" {
				name = l.Attrs().Name
			}

			ifaceNames = append(ifaceNames, name)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Interfaces whose tools interface lives in the host netns carry their
	// impairments there, not in the container netns.
	hostImpairments, hostRows, err := toolsIfaceImpairments(node, ifaceNames)
	if err != nil {
		return err
	}

	impairments = append(impairments, hostImpairments...)
	tableRows = append(tableRows, hostRows...)

	if jsonFormat {
		outputData := map[string][]clabtypes.ImpairmentData{
			o.ToolsNetem.ContainerName: impairments,
		}

		jsonData, err := json.MarshalIndent(outputData, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %v", err)
		}

		fmt.Println(string(jsonData))
	} else {
		printImpairments(tableRows)
	}

	return nil
}

// toolsIfaceImpairments returns JSON and table impairment rows for the
// interfaces whose tools interface lives in the host netns (see
// utils.StitchAltName).
func toolsIfaceImpairments(
	node *clabcore.NetemNode,
	ifaces []string,
) ([]clabtypes.ImpairmentData, []tableWriter.Row, error) {
	toolsIfaces := map[uint32]string{} // host ifindex -> display name

	for _, iface := range ifaces {
		toolsIface := node.ToolsIfaceFor(iface)
		if toolsIface == "" {
			continue
		}

		l, err := netlink.LinkByName(toolsIface)
		if err != nil {
			continue
		}

		toolsIfaces[uint32(l.Attrs().Index)] = fmt.Sprintf("%s (host)", iface)
	}

	if len(toolsIfaces) == 0 {
		return nil, nil, nil
	}

	hostNs, err := ns.GetCurrentNS()
	if err != nil {
		return nil, nil, err
	}

	tcnl, err := clabnetem.NewTC(int(hostNs.Fd()))
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v", err)
		}
	}()

	qdiscs, err := clabnetem.Impairments(tcnl)
	if err != nil {
		return nil, nil, err
	}

	var impairments []clabtypes.ImpairmentData

	var rows []tableWriter.Row

	for idx := range qdiscs {
		display, ok := toolsIfaces[qdiscs[idx].Ifindex]
		if !ok || qdiscs[idx].Attribute.Kind != "netem" {
			continue
		}

		d := qdiscToJSONData(&qdiscs[idx])
		d.Interface = display
		impairments = append(impairments, d)

		row := qdiscToTableData(&qdiscs[idx])
		row[0] = display
		rows = append(rows, row)
	}

	return impairments, rows, nil
}

func netemResetFn(o *Options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, err := clabcore.ResolveNetemNode(ctx, o.Global.Runtime, o.Global.Timeout, o.ToolsNetem.ContainerName)
	if err != nil {
		return err
	}

	target, err := node.TargetFor(o.ToolsNetem.Interface)
	if err != nil {
		return err
	}

	nodeNs, err := ns.GetNS(target.NSPath)
	if err != nil {
		return err
	}

	tcnl, err := clabnetem.NewTC(int(nodeNs.Fd()))
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
			clabutils.SanitizeInterfaceName(target.Iface))
		if err != nil {
			return err
		}
		// Retrieve the standard net.Interface from the netlink.Link name.
		netemIfIface, err := net.InterfaceByName(netemIfLink.Attrs().Name)
		if err != nil {
			return err
		}

		if err := clabnetem.DeleteImpairments(tcnl, netemIfIface); err != nil {
			return err
		}

		fmt.Printf("Reset impairments on node %q, interface %q\n",
			target.DisplayName, netemIfLink.Attrs().Name)

		return nil
	})

	return err
}
