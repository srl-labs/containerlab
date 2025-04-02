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
	InspectCmd.Flags().BoolVarP(&details, "details", "", false, "print all details of lab containers")
	InspectCmd.Flags().StringVarP(&inspectFormat, "format", "f", "table", "output format. One of [table, json]")
	InspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
	InspectCmd.Flags().BoolVarP(&wide, "wide", "w", false,
		"also more details about a lab and its nodes")
}

func inspectFn(_ *cobra.Command, _ []string) error {
	if common.Name == "" && common.Topo == "" && !all {
		fmt.Println("provide either a lab name (--name) or a topology file path (--topo) or the --all flag")
		return nil
	}

	if inspectFormat != "table" && inspectFormat != "json" {
		return fmt.Errorf("output format %v is not supported, use 'table' or 'json'", inspectFormat)
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
		return fmt.Errorf("could not parse the topology file: %v", err)
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	var containers []runtime.GenericContainer
	var glabels []*types.GenericFilter

	// if the topo file is available, use it
	if common.Topo != "" {
		containers, err = c.ListNodesContainers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}
	} else {
		// or when just the name is given
		if common.Name != "" {
			// if name is set, filter for name
			glabels = []*types.GenericFilter{{
				FilterType: "label", Match: common.Name,
				Field: labels.Containerlab, Operator: "=",
			}}
		} else {
			// this is the --all case
			glabels = []*types.GenericFilter{{
				FilterType: "label",
				Field:      labels.Containerlab, Operator: "exists",
			}}
		}

		containers, err = c.ListContainers(ctx, glabels)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}
	}

	if len(containers) == 0 {
		log.Info("no containers found")
		return nil
	}
	if details {
		b, err := json.MarshalIndent(containers, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal containers struct: %v", err)
		}
		fmt.Println(string(b))
		return nil
	}

	err = PrintContainerInspect(containers, inspectFormat)
	return err
}

func toTableData(contDetails []types.ContainerDetails) []tableWriter.Row {
	tabData := make([]tableWriter.Row, 0, len(contDetails))
	for i := range contDetails {
		d := &contDetails[i]

		tabRow := tableWriter.Row{}

		if all {
			tabRow = append(tabRow, d.LabPath, d.LabName)
		}

		// Display more columns
		if wide {
			tabRow = append(tabRow, d.Owner)
		}

		// Common fields
		tabRow = append(tabRow,
			d.Name,
			fmt.Sprintf("%s\n%s", d.Kind, d.Image),
			d.State,
			fmt.Sprintf("%s\n%s",
				ipWithoutPrefix(d.IPv4Address),
				ipWithoutPrefix(d.IPv6Address)))

		tabData = append(tabData, tabRow)
	}
	return tabData
}

// getTopologyPath returns the relative path to the topology file
// if the relative path is shorted than the absolute path.
func getTopologyPath(p string) (string, error) {
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

func PrintContainerInspect(containers []runtime.GenericContainer, format string) error {
	contDetails := make([]types.ContainerDetails, 0, len(containers))

	// Gather details of each container
	for _, cont := range containers {
		path, err := getTopologyPath(cont.Labels[labels.TopoFile])
		if err != nil {
			return fmt.Errorf("failed to get topology path: %v", err)
		}

		baseState, healthInfo := parseStateHealth(cont.State, cont.Status)

		// Format differently based on output format
		var stateStr string
		if format == "json" {
			if healthInfo != "" {
				stateStr = fmt.Sprintf("%s %s", baseState, healthInfo)
			} else {
				stateStr = baseState
			}
		} else {
			if healthInfo != "" {
				stateStr = fmt.Sprintf("%s\n%s", baseState, healthInfo)
			} else {
				stateStr = baseState
			}
		}

		cdet := &types.ContainerDetails{
			LabName:     cont.Labels[labels.Containerlab],
			LabPath:     path,
			Image:       cont.Image,
			State:       stateStr,
			IPv4Address: cont.GetContainerIPv4(),
			IPv6Address: cont.GetContainerIPv6(),
		}
		cdet.ContainerID = cont.ShortID

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

		contDetails = append(contDetails, *cdet)
	}

	sort.Slice(contDetails, func(i, j int) bool {
		if contDetails[i].LabName == contDetails[j].LabName {
			return contDetails[i].Name < contDetails[j].Name
		}
		return contDetails[i].LabName < contDetails[j].LabName
	})

	resultData := &types.LabData{Containers: contDetails}

	switch format {
	case "json":
		b, err := json.MarshalIndent(resultData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal container details: %v", err)
		}
		fmt.Println(string(b))
		return nil

	case "table":
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

		header := tableWriter.Row{
			"Lab Name",
			"Name",
			"Kind/Image",
			"State",
			"IPv4/6 Address",
		}

		if wide {
			header = slices.Insert(header, 1, "Owner")
			table.SetColumnConfigs([]tableWriter.ColumnConfig{
				{Number: 1, AutoMerge: true},
			})
		}

		if all {
			// merge cells with topo file path and lab name when in all mode
			table.SetColumnConfigs([]tableWriter.ColumnConfig{
				{Number: 1, AutoMerge: true},
				{Number: 2, AutoMerge: true},
			})
			table.AppendHeader(append(tableWriter.Row{"Topology"}, header...))

			if wide {
				table.SetColumnConfigs([]tableWriter.ColumnConfig{
					{Number: 1, AutoMerge: true},
					{Number: 2, AutoMerge: true},
					{Number: 3, AutoMerge: true},
				})
			}

		} else {
			table.AppendHeader(append(tableWriter.Row{}, header[1:]...))
		}

		table.AppendRows(tabData)

		table.Render()

		return nil
	}
	return nil
}

// parseStateHealth extracts base state and health info separately
func parseStateHealth(state, status string) (string, string) {
	// Default base state is from the State field
	baseState := state

	// If State field doesn't have clear info, try to extract from Status
	if baseState == "" || baseState == "running" {
		// The Status usually starts with "Up" for running containers
		if strings.HasPrefix(status, "Up") {
			baseState = "running"
		} else if strings.HasPrefix(status, "Created") {
			baseState = "created"
		} else if strings.HasPrefix(status, "Exited") {
			baseState = "exited"
		}
	}

	// Extract health information if present
	healthInfo := ""
	if strings.Contains(status, "(healthy)") {
		healthInfo = "healthy"
	} else if strings.Contains(status, "(health: starting)") {
		healthInfo = "health: starting"
	} else if strings.Contains(status, "(unhealthy)") {
		healthInfo = "unhealthy"
	}

	return baseState, healthInfo
}

type TokenFileResults struct {
	File    string
	Labname string
}

func ipWithoutPrefix(ip string) string {
	if strings.Contains(ip, "N/A") {
		return ip
	}

	ipParts := strings.Split(ip, "/")
	if len(ipParts) != 2 {
		return ip
	}

	return ipParts[0]
}
