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

	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
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

// inspectCmd represents the inspect command.
var inspectCmd = &cobra.Command{
	Use:     "inspect",
	Short:   "inspect lab details",
	Long:    "show details about a particular lab or all running labs\nreference: https://containerlab.dev/cmd/inspect/",
	Aliases: []string{"ins", "i"},
	PreRunE: sudoCheck,
	RunE:    inspectFn,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().BoolVarP(&details, "details", "", false, "print all details of lab containers")
	inspectCmd.Flags().StringVarP(&inspectFormat, "format", "f", "table", "output format. One of [table, json]")
	inspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
	inspectCmd.Flags().BoolVarP(&wide, "wide", "w", false,
		"also more details about a lab and its nodes")
}

func inspectFn(_ *cobra.Command, _ []string) error {
	if name == "" && topo == "" && !all {
		fmt.Println("provide either a lab name (--name) or a topology file path (--topo) or the --all flag")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	}

	if topo != "" {
		opts = append(opts,
			clab.WithTopoPath(topo, varsFile),
			clab.WithNodeFilter(nodeFilter),
		)
	}

	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return fmt.Errorf("could not parse the topology file: %v", err)
	}

	var containers []runtime.GenericContainer
	var glabels []*types.GenericFilter

	// if the topo file is available, use it
	if topo != "" {
		containers, err = c.ListNodesContainers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}
	} else {
		// or when just the name is given
		if name != "" {
			// if name is set, filter for name
			glabels = []*types.GenericFilter{{
				FilterType: "label", Match: name,
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
		log.Println("no containers found")
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

	err = printContainerInspect(containers, inspectFormat)
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

func printContainerInspect(containers []runtime.GenericContainer, format string) error {
	contDetails := make([]types.ContainerDetails, 0, len(containers))

	// Gather details of each container
	for _, cont := range containers {

		path, err := getTopologyPath(cont.Labels[labels.TopoFile])
		if err != nil {
			return fmt.Errorf("failed to get topology path: %v", err)
		}

		cdet := &types.ContainerDetails{
			LabName:     cont.Labels[labels.Containerlab],
			LabPath:     path,
			Image:       cont.Image,
			State:       cont.State,
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
