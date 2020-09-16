package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
			fmt.Println("provide either lab prefix (--prefix) or topology file path (--topo)")
			return
		}
		c := clab.NewContainerLab(debug)
		err := c.Init(timeout)
		if err != nil {
			log.Fatal(err)
		}
		if prefix == "" {
			if err = c.GetTopology(&topo); err != nil {
				log.Fatal(err)
			}
			prefix = c.Conf.Prefix
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
		contDetails := make([]containerDetails, 0, len(containers))
		for _, cont := range containers {
			cdet := containerDetails{
				Image: cont.Image,
				State: cont.State,
			}
			if len(cont.Names) > 0 {
				cdet.Name = cont.Names[0]
			}
			if kind, ok := cont.Labels["kind"]; ok {
				cdet.Kind = kind
			}
			if group, ok := cont.Labels["group"]; ok {
				cdet.Group = group
			}
			if cont.NetworkSettings != nil {
				if c.Conf.DockerInfo.Bridge != "" {
					if br, ok := cont.NetworkSettings.Networks[c.Conf.DockerInfo.Bridge]; ok {
						cdet.IPv4Address = fmt.Sprintf("%s/%d", br.IPAddress, br.IPPrefixLen)
						cdet.IPv6Address = fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
					}
				}
				if cdet.IPv4Address == "" && cdet.IPv6Address == "" {
					for _, br := range cont.NetworkSettings.Networks {
						cdet.IPv4Address = fmt.Sprintf("%s/%d", br.IPAddress, br.IPPrefixLen)
						cdet.IPv6Address = fmt.Sprintf("%s/%d", br.GlobalIPv6Address, br.GlobalIPv6PrefixLen)
						break
					}
				}
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
	return tabData
}
