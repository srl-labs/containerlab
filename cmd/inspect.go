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
	"strings"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/types"
)

var format string
var details bool
var all bool

type containerDetails struct {
	LabName     string `json:"lab_name,omitempty"`
	LabPath     string `json:"labPath,omitempty"`
	Name        string `json:"name,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	Image       string `json:"image,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Group       string `json:"group,omitempty"`
	State       string `json:"state,omitempty"`
	IPv4Address string `json:"ipv4_address,omitempty"`
	IPv6Address string `json:"ipv6_address,omitempty"`
}
type BridgeDetails struct{}

// inspectCmd represents the inspect command
var inspectCmd = &cobra.Command{
	Use:     "inspect",
	Short:   "inspect lab details",
	Long:    "show details about a particular lab or all running labs\nreference: https://containerlab.srlinux.dev/cmd/inspect/",
	Aliases: []string{"ins", "i"},
	PreRunE: sudoCheck,
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" && topo == "" && !all {
			fmt.Println("provide either a lab name (--name) or a topology file path (--topo) or the flag --all")
			return
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt, debug, timeout, graceful),
		}
		if topo != "" {
			opts = append(opts, clab.WithTopoFile(topo))
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			fmt.Errorf("could not parse the topology file: %v", err)
		}

		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var glabels []*types.GenericFilter
		if all {
			glabels = []*types.GenericFilter{{FilterType: "label", Field: "containerlab", Operator: "exists"}}
		} else {
			if name != "" {
				glabels = []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
			} else if topo != "" {
				glabels = []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "="}}
			}
		}

		containers, err := c.ListContainers(ctx, glabels)
		if err != nil {
			log.Fatalf("failed to list containers: %s", err)
		}

		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		if details {
			b, err := json.MarshalIndent(containers, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal containers struct: %v", err)
			}
			fmt.Println(string(b))
			return
		}
		printContainerInspect(c, containers, c.Config.Mgmt.Network, format)
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().BoolVarP(&details, "details", "", false, "print all details of lab containers")
	inspectCmd.Flags().StringVarP(&format, "format", "f", "table", "output format. One of [table, json]")
	inspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
}

func toTableData(det []containerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for i, d := range det {
		if all {
			tabData = append(tabData, []string{fmt.Sprintf("%d", i+1), d.LabPath, d.LabName, d.Name, d.ContainerID, d.Image, d.Kind, d.Group, d.State, d.IPv4Address, d.IPv6Address})
			continue
		}
		tabData = append(tabData, []string{fmt.Sprintf("%d", i+1), d.Name, d.ContainerID, d.Image, d.Kind, d.Group, d.State, d.IPv4Address, d.IPv6Address})
	}
	return tabData
}

func printContainerInspect(c *clab.CLab, containers []types.GenericContainer, bridgeName, format string) {
	contDetails := make([]containerDetails, 0, len(containers))
	// do not print published ports unless mysocketio kind is found
	printMysocket := false
	var mysocketCID string

	for _, cont := range containers {
		// get topo file path relative of the cwd
		cwd, _ := os.Getwd()
		path, _ := filepath.Rel(cwd, cont.Labels["clab-topo-file"])

		cdet := containerDetails{
			LabName:     cont.Labels["containerlab"],
			LabPath:     path,
			Image:       cont.Image,
			State:       cont.State,
			IPv4Address: getContainerIPv4(cont, bridgeName),
			IPv6Address: getContainerIPv6(cont, bridgeName),
		}
		cdet.ContainerID = cont.ShortID

		if len(cont.Names) > 0 {
			cdet.Name = strings.TrimLeft(cont.Names[0], "/")
		}
		if kind, ok := cont.Labels["clab-node-kind"]; ok {
			cdet.Kind = kind
			if kind == "mysocketio" {
				printMysocket = true
				mysocketCID = cont.ID
			}
		}
		if group, ok := cont.Labels["clab-node-group"]; ok {
			cdet.Group = group
		}
		contDetails = append(contDetails, cdet)
	}
	sort.Slice(contDetails, func(i, j int) bool {
		if contDetails[i].LabName == contDetails[j].LabName {
			return contDetails[i].Name < contDetails[j].Name
		}
		return contDetails[i].LabName < contDetails[j].LabName
	})
	if format == "json" {
		b, err := json.MarshalIndent(contDetails, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal container details: %v", err)
		}
		fmt.Println(string(b))
		return
	}
	tabData := toTableData(contDetails)
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{
		"Lab Name",
		"Name",
		"Container ID",
		"Image",
		"Kind",
		"Group",
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

	if !printMysocket {
		return
	}

	nodeRuntime, err := c.GetNodeRuntime(cntName)
	if err != nil {
		log.Fatal(err)
	}

	stdout, stderr, err := nodeRuntime.Exec(context.Background(), mysocketCID, []string{"mysocketctl", "socket", "ls"})
	if err != nil {
		log.Errorf("failed to execute cmd: %v", err)

	}
	if len(stderr) > 0 {
		log.Infof("errors during listing mysocketio sockets: %s", string(stderr))
	}
	fmt.Println("Published ports:")
	fmt.Println(string(stdout))
}

func getContainerIPv4(ctr types.GenericContainer, bridgeName string) string {
	if !ctr.NetworkSettings.Set {
		return ""
	}

	if ctr.NetworkSettings.IPv4addr == "" {
		return "NA"
	}

	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv4addr, ctr.NetworkSettings.IPv4pLen)

}

func getContainerIPv6(ctr types.GenericContainer, bridgeName string) string {
	if !ctr.NetworkSettings.Set {
		return ""
	}

	if ctr.NetworkSettings.IPv6addr == "" {
		return "NA"
	}

	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv6addr, ctr.NetworkSettings.IPv6pLen)
}
