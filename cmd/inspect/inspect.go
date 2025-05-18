// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	inspectFormat string
	details       bool
	all           bool
	wide          bool
)

// InspectCmd represents the inspect command.
var InspectCmd = &cobra.Command{
	Use:     "inspect",
	Short:   "inspect lab details",
	Long:    "show details about a particular lab or all running labs\nreference: https://containerlab.dev/cmd/inspect/",
	Aliases: []string{"ins", "i"},
	RunE:    inspectFn,
}

func init() {
	InspectCmd.Flags().BoolVarP(&details, "details", "", false,
		"print all details of lab containers (JSON format, grouped by lab)")
	InspectCmd.Flags().StringVarP(&inspectFormat, "format", "f", "table", "output format. One of [table, json]")
	InspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
	InspectCmd.Flags().BoolVarP(&wide, "wide", "w", false,
		"also more details about a lab and its nodes")
}

func inspectFn(_ *cobra.Command, _ []string) error {
	if common.Name == "" && common.Topo == "" && !all {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo) or the --all flag")
	}

	// Format validation (only relevant if --details is NOT used)
	if !details && inspectFormat != "table" && inspectFormat != "json" {
		return fmt.Errorf("output format %q is not supported when --details is not used, use 'table' or 'json'", inspectFormat)
	}
	// If --details is used, the format is implicitly JSON.
	if details {
		inspectFormat = "json" // Force JSON format if details are requested
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := []clab.ClabOption{
		clab.WithTimeout(common.Timeout),
		clab.WithRuntime(common.Runtime,
			&runtime.RuntimeConfig{
				Debug:            common.Debug,
				Timeout:          common.Timeout,
				GracefulShutdown: common.Graceful,
			},
		),
		clab.WithDebug(common.Debug),
	}

	if common.Topo != "" {
		opts = append(opts,
			clab.WithTopoPath(common.Topo, common.VarsFile),
			clab.WithNodeFilter(common.NodeFilter),
		)
	}

	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	containers, err := listContainers(ctx, c)
	if err != nil {
		return err
	}

	// Handle empty results
	if len(containers) == 0 {
		if inspectFormat == "json" {
			fmt.Println("{}")
		} else { // Table format
			log.Info("no containers found")
		}
		return err
	}

	// Handle --details (always produces grouped JSON output)
	if details {
		return printContainerDetailsJSON(containers)
	}

	// Handle non-details cases (table or grouped JSON summary)
	err = PrintContainerInspect(containers, inspectFormat)
	return err
}

// listContainers handles listing containers based on different criteria (topology or labels).
func listContainers(ctx context.Context, c *clab.CLab) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer
	var err error
	var gLabels []*types.GenericFilter

	if common.Topo != "" {
		// List containers defined in the topology file
		containers, err = c.ListNodesContainers(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers based on topology: %s", err)
		}
	} else {
		// List containers based on labels (--name or --all)
		if common.Name != "" {
			// Filter by specific lab name
			gLabels = []*types.GenericFilter{{
				FilterType: "label", Match: common.Name,
				Field: labels.Containerlab, Operator: "=",
			}}
		} else { // --all case
			// Filter for any containerlab container
			gLabels = []*types.GenericFilter{{
				FilterType: "label",
				Field:      labels.Containerlab, Operator: "exists",
			}}
		}

		containers, err = c.ListContainers(ctx, gLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers based on labels: %s", err)
		}
	}

	return containers, nil
}

func toTableData(contDetails []types.ContainerDetails) []tableWriter.Row {
	tabData := make([]tableWriter.Row, 0, len(contDetails))

	for i := range contDetails {
		d := &contDetails[i]
		tabRow := tableWriter.Row{}

		if all {
			tabRow = append(tabRow, d.LabPath, d.LabName)
		}

		if wide {
			tabRow = append(tabRow, d.Owner)
		}

		// we do not want to print status other than health in the table view
		if !strings.Contains(d.Status, "health") {
			d.Status = ""
		} else {
			d.Status = fmt.Sprintf("(%s)", d.Status)
		}

		// Common fields
		tabRow = append(tabRow,
			d.Name,
			fmt.Sprintf("%s\n%s", d.Kind, d.Image),
			fmt.Sprintf("%s\n%s", d.State, d.Status),
			fmt.Sprintf("%s\n%s",
				ipWithoutPrefix(d.IPv4Address),
				ipWithoutPrefix(d.IPv6Address)))

		tabData = append(tabData, tabRow)
	}
	return tabData
}

// getShortestTopologyPath calculates the relative path to the provided topology file from the current working directory and returns it if it is shorted than the absolute path p.
func getShortestTopologyPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}

	// get topo file path relative of the cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(cwd, p)
	if err != nil {
		return "", err
	}

	if len(relPath) < len(p) {
		return relPath, nil
	}

	return p, nil
}

// printContainerDetailsJSON handles the detailed JSON output of containers grouped by lab name.
func printContainerDetailsJSON(containers []runtime.GenericContainer) error {
	groupedDetails := make(map[string][]runtime.GenericContainer)
	// Sort containers first by lab name, then by container name for consistent output
	sort.Slice(containers, func(i, j int) bool {
		labNameI := containers[i].Labels[labels.Containerlab]
		labNameJ := containers[j].Labels[labels.Containerlab]
		if labNameI == labNameJ {
			// Use the first name if available
			nameI := ""
			if len(containers[i].Names) > 0 {
				nameI = containers[i].Names[0]
			}
			nameJ := ""
			if len(containers[j].Names) > 0 {
				nameJ = containers[j].Names[0]
			}
			return nameI < nameJ
		}
		return labNameI < labNameJ
	})

	// Group the sorted containers
	for _, cont := range containers {
		labName := cont.Labels[labels.Containerlab]
		// Ensure labName exists, default to a placeholder if missing (shouldn't happen with filters)
		if labName == "" {
			labName = "_unknown_lab_"
		}
		groupedDetails[labName] = append(groupedDetails[labName], cont)
	}

	b, err := json.MarshalIndent(groupedDetails, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal grouped container details: %v", err)
	}

	fmt.Println(string(b))

	return nil
}

// PrintContainerInspect handles non-details output (table or grouped JSON summary).
func PrintContainerInspect(containers []runtime.GenericContainer, format string) error {
	contDetails := make([]types.ContainerDetails, 0, len(containers))

	// Gather summary details of each container
	for _, cont := range containers {
		absPath := cont.Labels[labels.TopoFile]
		shortPath, err := getShortestTopologyPath(absPath)
		if err != nil {
			log.Warnf("failed to get relative topology path for container %s: %v, using raw path %q", cont.Names[0], err, absPath)
			shortPath = absPath // Use raw path as fallback for display
		}

		status := parseStatus(cont.Status)

		cdet := types.ContainerDetails{
			LabName:     cont.Labels[labels.Containerlab],
			LabPath:     shortPath, // Relative or shortest path for table view
			AbsLabPath:  absPath,   // Absolute path for JSON view
			Image:       cont.Image,
			State:       cont.State,
			Status:      status,
			IPv4Address: cont.GetContainerIPv4(),
			IPv6Address: cont.GetContainerIPv6(),
			ContainerID: cont.ShortID,
		}

		if len(cont.Names) > 0 {
			cdet.Name = cont.Names[0]
		}
		if group, ok := cont.Labels[labels.NodeGroup]; ok {
			cdet.Group = group
		}
		if kind, ok := cont.Labels[labels.NodeKind]; ok {
			cdet.Kind = kind
		}
		if owner, ok := cont.Labels[labels.Owner]; ok {
			cdet.Owner = owner
		}

		contDetails = append(contDetails, cdet)
	}

	// Sort summary details by lab name, then container name
	sort.Slice(contDetails, func(i, j int) bool {
		if contDetails[i].LabName == contDetails[j].LabName {
			return contDetails[i].Name < contDetails[j].Name
		}
		return contDetails[i].LabName < contDetails[j].LabName
	})

	// --- Output based on format ---
	switch format {
	case "json":
		// Group summary results by LabName
		// Use a map where keys are lab names and values are slices of container details
		groupedLabs := make(map[string][]types.ContainerDetails)
		for _, cd := range contDetails {
			labName := cd.LabName
			if labName == "" {
				labName = "_unknown_lab_" // Should not happen if filters work correctly
			}
			// Assign the *entire* ContainerDetails struct (including AbsLabPath)
			groupedLabs[labName] = append(groupedLabs[labName], cd)
		}

		b, err := json.MarshalIndent(groupedLabs, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal grouped container summary: %v", err)
		}

		fmt.Println(string(b))

		return err

	case "table":
		// Generate and render table using the summary data (which uses relative LabPath)
		tabData := toTableData(contDetails)
		table := tableWriter.NewWriter()
		table.SetOutputMirror(os.Stdout)
		table.SetStyle(tableWriter.StyleRounded)
		table.Style().Format.Header = text.FormatTitle
		table.Style().Format.HeaderAlign = text.AlignCenter
		table.Style().Options.SeparateRows = true
		table.Style().Color = tableWriter.ColorOptions{
			Header: text.Colors{text.Bold},
		}

		// Define base header structure (same as in the empty table case)
		headerBase := tableWriter.Row{"Name", "Kind/Image", "State", "IPv4/6 Address"}
		if wide {
			headerBase = slices.Insert(headerBase, 0, "Owner")
		}

		var header tableWriter.Row
		colConfigs := []tableWriter.ColumnConfig{}

		if all {
			header = append(tableWriter.Row{"Topology", "Lab Name"}, headerBase...)
			colConfigs = append(colConfigs, tableWriter.ColumnConfig{
				Number:    1,
				AutoMerge: true, VAlign: text.VAlignMiddle,
			})
			colConfigs = append(colConfigs, tableWriter.ColumnConfig{
				Number:    2,
				AutoMerge: true, VAlign: text.VAlignMiddle,
			})
			if wide {
				colConfigs = append(colConfigs, tableWriter.ColumnConfig{
					Number:    3,
					AutoMerge: true, VAlign: text.VAlignMiddle,
				})
			}
		} else {
			header = headerBase
			if wide {
				colConfigs = append(colConfigs, tableWriter.ColumnConfig{
					Number:    1,
					AutoMerge: true, VAlign: text.VAlignMiddle,
				})
			}
		}

		table.AppendHeader(header)
		if len(colConfigs) > 0 {
			table.SetColumnConfigs(colConfigs)
		}

		table.AppendRows(tabData)
		table.Render()
		return nil
	}
	// Should not be reached if format validation is correct
	return fmt.Errorf("internal error: unhandled format %q", format)
}

// parseStatus extracts a simpler status string, focusing on health states.
func parseStatus(status string) string {
	if strings.Contains(status, "unhealthy") {
		return "unhealthy"
	} else if strings.Contains(status, "health: starting") {
		return "health: starting"
	} else if strings.Contains(status, "healthy") {
		return "healthy"
	}
	// Return original status if no specific health info found
	return status
}

// ipWithoutPrefix removes the CIDR prefix length from an IP address string.
// Returns "N/A" if input contains "N/A".
// Returns original string if it doesn't contain exactly one "/".
func ipWithoutPrefix(ip string) string {
	if strings.Contains(ip, "N/A") {
		return ip
	}

	ipParts := strings.Split(ip, "/")
	if len(ipParts) != 2 {
		return ip // Return original if not in expected format "address/prefix"
	}

	return ipParts[0]
}
