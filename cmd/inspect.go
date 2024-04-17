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
	"sort"

	"github.com/olekukonko/tablewriter"
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

func toTableData(det []types.ContainerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for i := range det {
		d := &det[i]

		if all {
			tabData = append(tabData, []string{
				fmt.Sprintf("%d", i+1), d.LabPath,
				d.LabName, d.Name, d.ContainerID, d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address,
			})
			continue
		}
		tabData = append(tabData, []string{
			fmt.Sprintf("%d", i+1), d.Name, d.ContainerID,
			d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address,
		})
	}
	return tabData
}

func printContainerInspect(containers []runtime.GenericContainer, format string) error {
	contDetails := make([]types.ContainerDetails, 0, len(containers))

	// Gather details of each container
	for _, cont := range containers {

		// get topo file path relative of the cwd
		cwd, _ := os.Getwd()
		path, _ := filepath.Rel(cwd, cont.Labels[labels.TopoFile])

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
		table := tablewriter.NewWriter(os.Stdout)
		header := []string{
			"Lab Name",
			"Name",
			"Container ID",
			"Image",
			"Kind",
			"State",
			"IPv4 Address",
			"IPv6 Address",
		}
		if all {
			table.SetHeader(append([]string{"#", "Topo Path"}, header...))
		} else {
			table.SetHeader(append([]string{"#"}, header[1:]...))
		}
		table.SetAutoFormatHeaders(false)
		table.SetAutoWrapText(false)
		// merge cells with lab name and topo file path
		table.SetAutoMergeCellsByColumnIndex([]int{1, 2})
		table.AppendBulk(tabData)
		table.Render()

		return nil
	}
	return nil
}

type TokenFileResults struct {
	File    string
	Labname string
}
