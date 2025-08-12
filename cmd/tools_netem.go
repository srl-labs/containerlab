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
	containerlabcore "github.com/srl-labs/containerlab/core"
	containerlabinternaltc "github.com/srl-labs/containerlab/internal/tc"
	containerlablinks "github.com/srl-labs/containerlab/links"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

var (
	netemNode       string
	netemInterface  string
	netemDelay      time.Duration
	netemJitter     time.Duration
	netemLoss       float64
	netemRate       uint64
	netemCorruption float64
	netemFormat     string
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
	netemSetCmd.Flags().Float64VarP(&netemCorruption, "corruption", "", 0,
		"random packet corruption probability expressed in percentage (e.g. 0.1 means 0.1%)")

	netemSetCmd.MarkFlagRequired("node")
	netemSetCmd.MarkFlagRequired("interface")

	netemCmd.AddCommand(netemShowCmd)
	netemShowCmd.Flags().StringVarP(&netemNode, "node", "n", "", "node to apply impairment to")
	netemShowCmd.Flags().StringVarP(&netemFormat, "format", "f", "table", "output format (table, json)")

	// Add reset command
	netemCmd.AddCommand(netemResetCmd)
	netemResetCmd.Flags().StringVarP(&netemNode, "node", "n", "", "node to reset impairment on")
	netemResetCmd.Flags().StringVarP(&netemInterface, "interface", "i", "", "interface to reset impairment on")
	netemResetCmd.MarkFlagRequired("node")
	netemResetCmd.MarkFlagRequired("interface")
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
	PreRunE: validateInputAndRoot,
	RunE:    netemSetFn,
}

var netemShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "show link impairments for a node",
	PreRunE: validateInputAndRoot,
	RunE:    netemShowFn,
}

var netemResetCmd = &cobra.Command{
	Use:     "reset",
	Short:   "reset link impairments",
	Long:    `Reset network impairments by deleting the netem qdisc from the specified interface.`,
	PreRunE: validateInputAndRoot,
	RunE:    netemResetFn,
}

func netemSetFn(_ *cobra.Command, _ []string) error {
	// Ensure that the sch_netem kernel module is loaded (for Fedora/RHEL compatibility)
	if err := exec.Command("modprobe", "sch_netem").Run(); err != nil {
		log.Warn("failed to load sch_netem kernel module (expected on OrbStack machines)", "err", err)
	}

	// Get the runtime initializer.
	_, rinit, err := containerlabcore.RuntimeInitializer(runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	// init runtime with timeout
	err = rt.Init(
		containerlabruntime.WithConfig(
			&containerlabruntime.RuntimeConfig{
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

	tcnl, err := containerlabinternaltc.NewTC(int(nodeNs.Fd()))
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
			containerlablinks.SanitizeInterfaceName(netemInterface))
		if err != nil {
			return err
		}

		netemIfName := netemIfLink.Attrs().Name
		link, err := net.InterfaceByName(netemIfName)
		if err != nil {
			return err
		}

		qdisc, err := containerlabinternaltc.SetImpairments(tcnl, netemNode, link,
			netemDelay, netemJitter, netemLoss, netemRate, netemCorruption)
		if err != nil {
			return err
		}

		printImpairments([]gotc.Object{*qdisc})

		return nil
	})

	return err
}

func validateInputAndRoot(c *cobra.Command, args []string) error {
	if netemLoss < 0 || netemLoss > 100 {
		return fmt.Errorf("packet loss must be in the range between 0 and 100")
	}

	if netemJitter != 0 && netemDelay == 0 {
		return fmt.Errorf("jitter cannot be set without setting delay")
	}

	containerlabutils.CheckAndGetRootPrivs(c, args)

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

	var rows []tableWriter.Row

	for idx := range qdiscs {
		rows = append(rows, qdiscToTableData(&qdiscs[idx]))
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
			"N/A", // delay
			"N/A", // jitter
			"N/A", // loss
			"N/A", // rate
			"N/A", // corruption
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
func qdiscToJSONData(qdisc *gotc.Object) containerlabtypes.ImpairmentData {
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
		return containerlabtypes.ImpairmentData{
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
		rate = int(qdisc.Netem.Rate.Rate * 8 / 1000)
	}
	if qdisc.Netem.Corrupt != nil && qdisc.Netem.Corrupt.Probability != 0 {
		// round to 2 decimal places
		corruption = math.Round((float64(qdisc.Netem.Corrupt.Probability)/
			float64(math.MaxUint32)*100)*100) / 100
	}
	if qdisc.Netem.Qopt.Loss != 0 {
		// round to 2 decimal places
		loss = math.Round((float64(qdisc.Netem.Qopt.Loss)/float64(math.MaxUint32)*100)*100) / 100
	}

	return containerlabtypes.ImpairmentData{
		Interface:  ifDisplayName,
		Delay:      delay,
		Jitter:     jitter,
		PacketLoss: loss,
		Rate:       rate,
		Corruption: corruption,
	}
}

func netemShowFn(_ *cobra.Command, _ []string) error {
	// Get the runtime initializer.
	_, rinit, err := containerlabcore.RuntimeInitializer(runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()
	err = rt.Init(
		containerlabruntime.WithConfig(
			&containerlabruntime.RuntimeConfig{
				Timeout: timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the container's NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, netemNode)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS
	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	tcnl, err := containerlabinternaltc.NewTC(int(nodeNs.Fd()))
	if err != nil {
		return err
	}
	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v", err)
		}
	}()

	err = nodeNs.Do(func(_ ns.NetNS) error {
		qdiscs, err := containerlabinternaltc.Impairments(tcnl)
		if err != nil {
			return err
		}

		if netemFormat == "json" {
			var impairments []containerlabtypes.ImpairmentData
			for idx := range qdiscs {
				if qdiscs[idx].Attribute.Kind != "netem" {
					continue // skip clsact or other qdisc types
				}
				impairments = append(impairments, qdiscToJSONData(&qdiscs[idx]))
			}
			// Structure output as a map keyed by the node name.
			outputData := map[string][]containerlabtypes.ImpairmentData{
				netemNode: impairments,
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

func netemResetFn(_ *cobra.Command, _ []string) error {
	// Get the runtime initializer.
	_, rinit, err := containerlabcore.RuntimeInitializer(runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()
	err = rt.Init(
		containerlabruntime.WithConfig(
			&containerlabruntime.RuntimeConfig{
				Timeout: timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// retrieve the container's NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, netemNode)
	if err != nil {
		return err
	}

	var nodeNs ns.NetNS
	if nodeNs, err = ns.GetNS(nodeNsPath); err != nil {
		return err
	}

	tcnl, err := containerlabinternaltc.NewTC(int(nodeNs.Fd()))
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
			containerlablinks.SanitizeInterfaceName(netemInterface))
		if err != nil {
			return err
		}
		// Retrieve the standard net.Interface from the netlink.Link name.
		netemIfIface, err := net.InterfaceByName(netemIfLink.Attrs().Name)
		if err != nil {
			return err
		}
		if err := containerlabinternaltc.DeleteImpairments(tcnl, netemIfIface); err != nil {
			return err
		}
		fmt.Printf("Reset impairments on node %q, interface %q\n", netemNode, netemIfLink.Attrs().Name)
		return nil
	})

	return err
}
