package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
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

	Run: func(cmd *cobra.Command, args []string) {
		if name == "" && topo == "" && !all {
			fmt.Println("provide either a lab name (--name) or a topology file path (--topo) or the flag --all")
			return
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)
		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if all {
			labels = append(labels, "containerlab")
		} else {
			labels = append(labels, "containerlab=lab-"+name)
		}
		containers, err := c.ListContainers(ctx, labels)
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
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
		printContainerInspect(containers, c.Config.Mgmt.Network, format)
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

func printContainerInspect(containers []types.Container, bridgeName string, format string) {
	contDetails := make([]containerDetails, 0, len(containers))
	for _, cont := range containers {
		// get topo file path relative of the cwd
		cwd, _ := os.Getwd()
		path, _ := filepath.Rel(cwd, cont.Labels["clab-topo-file"])

		cdet := containerDetails{
			LabName:     strings.TrimPrefix(cont.Labels["containerlab"], "lab-"),
			LabPath:     path,
			Image:       cont.Image,
			State:       cont.State,
			IPv4Address: getContainerIPv4(cont, bridgeName),
			IPv6Address: getContainerIPv6(cont, bridgeName),
		}
		if len(cont.ID) > 11 {
			cdet.ContainerID = cont.ID[:12]
		}
		if len(cont.Names) > 0 {
			cdet.Name = strings.TrimLeft(cont.Names[0], "/")
		}
		if kind, ok := cont.Labels["kind"]; ok {
			cdet.Kind = kind
		}
		if group, ok := cont.Labels["group"]; ok {
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
}

func getContainerIPv4(container types.Container, bridgeName string) string {
	if container.NetworkSettings == nil {
		return ""
	}
	if bridgeName != "" {
		if br, ok := container.NetworkSettings.Networks[bridgeName]; ok {
			return fmt.Sprintf("%s/%d", br.IPAddress, br.IPPrefixLen)
		}
	}
	for _, br := range container.NetworkSettings.Networks {
		return fmt.Sprintf("%s/%d", br.IPAddress, br.IPPrefixLen)
	}
	return ""
}

func getContainerIPv6(container types.Container, bridgeName string) string {
	if container.NetworkSettings == nil {
		return ""
	}
	if bridgeName != "" {
		if br, ok := container.NetworkSettings.Networks[bridgeName]; ok {
			if br.GlobalIPv6Address == "" {
				return "NA"
			}
			return fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
		}
	}
	for _, br := range container.NetworkSettings.Networks {
		if br.GlobalIPv6Address == "" {
			return "NA"
		}
		return fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
	}
	return ""
}
