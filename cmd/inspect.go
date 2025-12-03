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
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func inspectCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "inspect",
		Short: "inspect lab details",
		Long: "show details about a particular lab or all running labs\n" +
			"reference: https://containerlab.dev/cmd/inspect/",
		Aliases: []string{"ins", "i"},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return inspectFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(
		&o.Inspect.Details,
		"details",
		"", o.Inspect.Details,
		"print all details of lab containers (JSON format, grouped by lab)",
	)
	c.Flags().StringVarP(
		&o.Inspect.Format,
		"format",
		"f",
		o.Inspect.Format,
		"output format. One of [table, json, csv]",
	)
	c.Flags().BoolVarP(
		&o.Destroy.All,
		"all",
		"a",
		o.Destroy.All,
		"show all deployed containerlab labs",
	)
	c.Flags().BoolVarP(
		&o.Inspect.Wide,
		"wide",
		"w",
		o.Inspect.Wide,
		"also more details about a lab and its nodes",
	)

	interfacesC := &cobra.Command{
		Use:   "interfaces",
		Short: "inspect interfaces of one or multiple nodes in a lab",
		Long: "show interfaces and their attributes in a specific deployed lab\n" +
			"reference: https://containerlab.dev/cmd/inspect/interfaces/",
		Aliases: []string{"int", "intf"},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return inspectInterfacesFn(cobraCmd, o)
		},
	}

	c.AddCommand(interfacesC)

	interfacesC.Flags().StringVarP(
		&o.Inspect.InterfacesFormat,
		"format",
		"f",
		o.Inspect.InterfacesFormat, "output format. One of [table, json]",
	)
	interfacesC.Flags().StringVarP(
		&o.Inspect.InterfacesNode,
		"node",
		"n",
		o.Inspect.InterfacesNode,
		"node to inspect",
	)

	return c, nil
}

func inspectFn(cobraCmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" && !o.Destroy.All {
		return fmt.Errorf(
			"provide either a lab name (--name) or a topology file path (--topo) or the --all flag",
		)
	}

	// Format validation (only relevant if --details is NOT used)
	if !o.Inspect.Details &&
		o.Inspect.Format != clabconstants.FormatTable &&
		o.Inspect.Format != clabconstants.FormatJSON &&
		o.Inspect.Format != clabconstants.FormatCSV {
		return fmt.Errorf(
			"output format %q is not supported when --details is not used, use "+
				"'table', 'json' or 'csv'",
			o.Inspect.Format,
		)
	}
	// If --details is used, the format is implicitly JSON.
	if o.Inspect.Details {
		o.Inspect.Format = clabconstants.FormatJSON // Force JSON format if details are requested
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	containers, err := listContainers(cobraCmd.Context(), c, o)
	if err != nil {
		return err
	}

	// Handle empty results
	if len(containers) == 0 {
		switch o.Inspect.Format {
		case clabconstants.FormatJSON:
			fmt.Println("{}")
		case clabconstants.FormatCSV:
			fmt.Println(
				"lab_name,labPath,absLabPath,name,container_id,image,kind,state,status," +
					"ipv4_address,ipv6_address,owner",
			)
		default: // Table format
			log.Info("no containers found")
		}

		return err
	}

	// Handle --details (always produces grouped JSON output)
	if o.Inspect.Details {
		return printContainerDetailsJSON(containers)
	}

	// Handle non-details cases (table or grouped JSON summary)
	err = PrintContainerInspect(containers, o)

	return err
}

// listContainers handles listing containers based on different criteria (topology or labels).
func listContainers(
	ctx context.Context,
	c *clabcore.CLab,
	o *Options,
) ([]clabruntime.GenericContainer, error) {
	var containers []clabruntime.GenericContainer

	var err error

	if o.Global.TopologyFile != "" {
		// List containers defined in the topology file
		containers, err = c.ListNodesContainers(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers based on topology: %s", err)
		}
	} else {
		var listOptions []clabcore.ListOption

		// List containers based on labels (--name or --all)
		if o.Global.TopologyName != "" {
			listOptions = append(listOptions, clabcore.WithListLabName(o.Global.TopologyName))
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

func toTableData(contDetails []clabtypes.ContainerDetails, o *Options) []tableWriter.Row {
	tabData := make([]tableWriter.Row, 0, len(contDetails))

	for i := range contDetails {
		d := &contDetails[i]
		tabRow := tableWriter.Row{}

		if o.Destroy.All {
			tabRow = append(tabRow, d.LabPath, d.LabName)
		}

		if o.Inspect.Wide {
			tabRow = append(tabRow, d.Owner)
		}

		// we do not want to print status other than health in the table view
		if !strings.Contains(d.Status, "health") {
			d.Status = ""
		} else {
			d.Status = fmt.Sprintf("(%s)", d.Status)
		}

		// Common fields
		if o.Inspect.Wide {
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

// getShortestTopologyPath calculates the relative path to the provided topology file from
// the current working directory and returns it if it is shorted than the absolute path p.
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
		labNameI := containers[i].Labels[clabconstants.Containerlab]
		labNameJ := containers[j].Labels[clabconstants.Containerlab]

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
		labName := containers[idx].Labels[clabconstants.Containerlab]
		// Ensure labName exists, default to a placeholder if missing
		// (shouldn't happen with filters)
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

func printContainerInspectTable(contDetails []clabtypes.ContainerDetails, o *Options) {
	// Generate and render table using the summary data (which uses relative LabPath)
	tabData := toTableData(contDetails, o)
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
	if o.Inspect.Wide {
		headerBase = slices.Insert(headerBase, 0, "Owner")
	}

	var header tableWriter.Row

	var colConfigs []tableWriter.ColumnConfig

	if o.Destroy.All {
		header = append(tableWriter.Row{"Topology", "Lab Name"}, headerBase...)
		if !o.Inspect.Wide {
			colConfigs = append(
				colConfigs,
				tableWriter.ColumnConfig{
					Number:    1,
					AutoMerge: true, VAlign: text.VAlignMiddle,
				},
				tableWriter.ColumnConfig{
					Number:    2, //nolint: mnd
					AutoMerge: true, VAlign: text.VAlignMiddle,
				},
			)
		}
		// If wide, do not set AutoMerge for any columns
	} else {
		header = headerBase

		if !o.Inspect.Wide {
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
	csv := "lab_name,labPath,absLabPath,name,container_id,image,kind,state," +
		"status,ipv4_address,ipv6_address,owner\n"
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
func PrintContainerInspect(containers []clabruntime.GenericContainer, o *Options) error {
	contDetails := make([]clabtypes.ContainerDetails, 0, len(containers))

	// Gather summary details of each container
	for idx := range containers {
		absPath := containers[idx].Labels[clabconstants.TopoFile]

		shortPath, err := getShortestTopologyPath(absPath)
		if err != nil {
			log.Warnf(
				"failed to get relative topology path for container %s: %v, using raw path %q",
				containers[idx].Names[0],
				err,
				absPath,
			)

			shortPath = absPath // Use raw path as fallback for display
		}

		status := parseStatus(containers[idx].Status)

		cdet := clabtypes.ContainerDetails{
			LabName:     containers[idx].Labels[clabconstants.Containerlab],
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

		if group, ok := containers[idx].Labels[clabconstants.NodeGroup]; ok {
			cdet.Group = group
		}

		if kind, ok := containers[idx].Labels[clabconstants.NodeKind]; ok {
			cdet.Kind = kind
		}

		if owner, ok := containers[idx].Labels[clabconstants.Owner]; ok {
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

	switch o.Inspect.Format {
	case clabconstants.FormatJSON:
		err := printContainerInspectJSON(contDetails)
		if err != nil {
			return err
		}
	case "table":
		printContainerInspectTable(contDetails, o)
	case "csv":
		printContainerInspectCSV(contDetails)
	default:
		return fmt.Errorf("internal error: unhandled format %q", o.Inspect.Format)
	}

	return nil
}

// parseStatus extracts a simpler status string, focusing on health states.
func parseStatus(status string) string {
	switch {
	case strings.Contains(status, "unhealthy"):
		return "unhealthy"
	case strings.Contains(status, "health: starting"):
		return "health: starting"
	case strings.Contains(status, "healthy"):
		return "healthy"
	default:
		// Return original status if no specific health info found
		return status
	}
}

// ipWithoutPrefix removes the CIDR prefix length from an IP address string.
// Returns "N/A" if input contains "N/A".
// Returns original string if it doesn't contain exactly one "/".
func ipWithoutPrefix(ip string) string {
	if strings.Contains(ip, clabconstants.NotApplicable) {
		return ip
	}

	ipParts := strings.Split(ip, "/")
	if len(ipParts) != 2 { //nolint: mnd
		return ip // Return original if not in expected format "address/prefix"
	}

	return ipParts[0]
}
