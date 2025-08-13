// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

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
	clabcore "github.com/srl-labs/containerlab/core"
	clablabels "github.com/srl-labs/containerlab/labels"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

var (
	inspectFormat string
	details       bool
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
	InspectCmd.Flags().StringVarP(&inspectFormat, "format", "f", "table",
		"output format. One of [table, json, csv]")
	InspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
	InspectCmd.Flags().BoolVarP(&wide, "wide", "w", false,
		"also more details about a lab and its nodes")
}

func inspectFn(cobraCmd *cobra.Command, _ []string) error {
	if labName == "" && topoFile == "" && !all {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo) or the --all flag")
	}

	// Format validation (only relevant if --details is NOT used)
	if !details && inspectFormat != "table" && inspectFormat != "json" && inspectFormat != "csv" {
		return fmt.Errorf("output format %q is not supported when --details is not used, use 'table', 'json' or 'csv'", inspectFormat)
	}
	// If --details is used, the format is implicitly JSON.
	if details {
		inspectFormat = "json" // Force JSON format if details are requested
	}

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(timeout),
		clabcore.WithRuntime(
			runtime,
			&clabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(debug),
	}

	if topoFile != "" {
		opts = append(opts,
			clabcore.WithTopoPath(topoFile, varsFile),
			clabcore.WithNodeFilter(nodeFilter),
		)
	}

	c, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	containers, err := listContainers(cobraCmd.Context(), c)
	if err != nil {
		return err
	}

	// Handle empty results
	if len(containers) == 0 {
		switch inspectFormat {
		case "json":
			fmt.Println("{}")
		case "csv":
			fmt.Println("lab_name,labPath,absLabPath,name,container_id,image,kind,state,status,ipv4_address,ipv6_address,owner")
		default: // Table format
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
func listContainers(ctx context.Context, c *clabcore.CLab) ([]clabruntime.GenericContainer, error) {
	var containers []clabruntime.GenericContainer
	var err error

	if topoFile != "" {
		// List containers defined in the topology file
		containers, err = c.ListNodesContainers(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers based on topology: %s", err)
		}
	} else {
		var listOptions []clabcore.ListOption

		// List containers based on labels (--name or --all)
		if labName != "" {
			listOptions = append(listOptions, clabcore.WithListLabName(labName))
		} else {
			listOptions = append(listOptions, clabcore.WithListclabLabelExists())
		}

		containers, err = c.ListContainers(ctx, listOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers based on labels: %s", err)
		}
	}

	return containers, nil
}

func toTableData(contDetails []clabtypes.ContainerDetails) []tableWriter.Row {
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
		if wide {
			// Print all fields on one line, no newlines
			tabRow = append(tabRow,
				d.Name,
				fmt.Sprintf("%s %s", d.Kind, d.Image),
				fmt.Sprintf("%s %s", d.State, d.Status),
				fmt.Sprintf("%s %s",
					ipWithoutPrefix(d.IPv4Address),
					ipWithoutPrefix(d.IPv6Address)))
		} else {
			tabRow = append(tabRow,
				d.Name,
				fmt.Sprintf("%s\n%s", d.Kind, d.Image),
				fmt.Sprintf("%s\n%s", d.State, d.Status),
				fmt.Sprintf("%s\n%s",
					ipWithoutPrefix(d.IPv4Address),
					ipWithoutPrefix(d.IPv6Address)))
		}

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
func printContainerDetailsJSON(containers []clabruntime.GenericContainer) error {
	groupedDetails := make(map[string][]clabruntime.GenericContainer)
	// Sort containers first by lab name, then by container name for consistent output
	sort.Slice(containers, func(i, j int) bool {
		labNameI := containers[i].Labels[clablabels.Containerlab]
		labNameJ := containers[j].Labels[clablabels.Containerlab]
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
	for idx := range containers {
		labName := containers[idx].Labels[clablabels.Containerlab]
		// Ensure labName exists, default to a placeholder if missing (shouldn't happen with filters)
		if labName == "" {
			labName = "_unknown_lab_"
		}
		groupedDetails[labName] = append(groupedDetails[labName], containers[idx])
	}

	b, err := json.MarshalIndent(groupedDetails, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal grouped container details: %v", err)
	}

	fmt.Println(string(b))

	return nil
}

func printContainerInspectJSON(contDetails []clabtypes.ContainerDetails) error {
	// Group summary results by LabName
	// Use a map where keys are lab names and values are slices of container details
	groupedLabs := make(map[string][]clabtypes.ContainerDetails)
	for idx := range contDetails {
		labName := contDetails[idx].LabName
		if labName == "" {
			labName = "_unknown_lab_" // Should not happen if filters work correctly
		}
		// Assign the *entire* ContainerDetails struct (including AbsLabPath)
		groupedLabs[labName] = append(groupedLabs[labName], contDetails[idx])
	}

	b, err := json.MarshalIndent(groupedLabs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal grouped container summary: %v", err)
	}

	fmt.Println(string(b))

	return nil
}

func printContainerInspectTable(contDetails []clabtypes.ContainerDetails) {
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

	// For --wide, avoid AutoMerge and multi-line cells
	headerBase := tableWriter.Row{"Name", "Kind/Image", "State", "IPv4/6 Address"}
	if wide {
		headerBase = slices.Insert(headerBase, 0, "Owner")
	}

	var header tableWriter.Row
	colConfigs := []tableWriter.ColumnConfig{}

	if all {
		header = append(tableWriter.Row{"Topology", "Lab Name"}, headerBase...)
		if !wide {
			colConfigs = append(
				colConfigs,
				tableWriter.ColumnConfig{
					Number:    1,
					AutoMerge: true, VAlign: text.VAlignMiddle,
				},
				tableWriter.ColumnConfig{
					Number:    2,
					AutoMerge: true, VAlign: text.VAlignMiddle,
				},
			)
		}
		// If wide, do not set AutoMerge for any columns
	} else {
		header = headerBase
		if !wide {
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
}

func printContainerInspectCSV(contDetails []clabtypes.ContainerDetails) {
	csv := "lab_name,labPath,absLabPath,name,container_id,image,kind,state,status,ipv4_address,ipv6_address,owner\n"
	for idx := range contDetails {
		csv += fmt.Sprintf("%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
			contDetails[idx].LabName,
			contDetails[idx].LabPath,
			contDetails[idx].AbsLabPath,
			contDetails[idx].Name,
			contDetails[idx].ContainerID,
			contDetails[idx].Image,
			contDetails[idx].Kind,
			contDetails[idx].State,
			contDetails[idx].Status,
			contDetails[idx].IPv4Address,
			contDetails[idx].IPv6Address,
			contDetails[idx].Owner)
	}
	fmt.Print(csv)
}

// PrintContainerInspect handles non-details output (table or grouped JSON summary).
func PrintContainerInspect(containers []clabruntime.GenericContainer, format string) error {
	contDetails := make([]clabtypes.ContainerDetails, 0, len(containers))

	// Gather summary details of each container
	for idx := range containers {
		absPath := containers[idx].Labels[clablabels.TopoFile]
		shortPath, err := getShortestTopologyPath(absPath)
		if err != nil {
			log.Warnf("failed to get relative topology path for container %s: %v, using raw path %q", containers[idx].Names[0], err, absPath)
			shortPath = absPath // Use raw path as fallback for display
		}

		status := parseStatus(containers[idx].Status)

		cdet := clabtypes.ContainerDetails{
			LabName:     containers[idx].Labels[clablabels.Containerlab],
			LabPath:     shortPath, // Relative or shortest path for table view
			AbsLabPath:  absPath,   // Absolute path for JSON view
			Image:       containers[idx].Image,
			State:       containers[idx].State,
			Status:      status,
			IPv4Address: containers[idx].GetContainerIPv4(),
			IPv6Address: containers[idx].GetContainerIPv6(),
			ContainerID: containers[idx].ShortID,
		}

		if len(containers[idx].Names) > 0 {
			cdet.Name = containers[idx].Names[0]
		}
		if group, ok := containers[idx].Labels[clablabels.NodeGroup]; ok {
			cdet.Group = group
		}
		if kind, ok := containers[idx].Labels[clablabels.NodeKind]; ok {
			cdet.Kind = kind
		}
		if owner, ok := containers[idx].Labels[clablabels.Owner]; ok {
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

	switch format {
	case "json":
		err := printContainerInspectJSON(contDetails)
		if err != nil {
			return err
		}
	case "table":
		printContainerInspectTable(contDetails)
	case "csv":
		printContainerInspectCSV(contDetails)
	default:
		return fmt.Errorf("internal error: unhandled format %q", format)
	}

	return nil
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
