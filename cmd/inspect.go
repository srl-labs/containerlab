package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

type containerDetails struct {
	Name        string `json:"name,omitempty"`
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
	Use:   "inspect",
	Short: "inspect lab details",

	Run: func(cmd *cobra.Command, args []string) {
		if prefix == "" && topo == "" {
			fmt.Println("provide either a lab prefix (--prefix) or a topology file path (--topo)")
			return
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)
		if prefix == "" {
			prefix = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		labels = append(labels, "containerlab=lab-"+prefix)
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
	inspectCmd.Flags().StringVarP(&format, "format", "f", "", "lab name prefix")
}

func toTableData(det []containerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for _, d := range det {
		tabData = append(tabData, []string{d.Name, d.Image, d.Kind, d.Group, d.State, d.IPv4Address, d.IPv6Address})
	}
	sort.Slice(tabData, func(i, j int) bool { return tabData[i][0] < tabData[j][0] })
	return tabData
}

func printContainerInspect(containers []types.Container, bridgeName string, format string) {
	contDetails := make([]containerDetails, 0, len(containers))
	for _, cont := range containers {
		cdet := containerDetails{
			Image:       cont.Image,
			State:       cont.State,
			IPv4Address: getContainerIPv4(cont, bridgeName),
			IPv6Address: getContainerIPv6(cont, bridgeName),
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
	table.SetHeader([]string{
		"Name",
		"Image",
		"Kind",
		"Group",
		"State",
		"IPv4 Address",
		"IPv6 Address",
	})
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
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
			return fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
		}
	}
	for _, br := range container.NetworkSettings.Networks {
		return fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
	}
	return ""
}
