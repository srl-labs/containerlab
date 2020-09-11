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

type containerDetails struct {
	Name        string
	Image       string
	Kind        string
	Group       string
	State       string
	IPv4Address string
	IPv6Address string
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
		err := c.Init()
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
		if format == "json" {
			b, err := json.MarshalIndent(containers, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal containers struct: %v", err)
			}
			fmt.Println(string(b))
			return
		}
		details := make([]containerDetails, 0, len(containers))
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
			details = append(details, cdet)
		}
		tabData := toTableData(details)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Name",
			"Image",
			"Kind",
			"Group",
			"State",
			"IPv4Address",
			"IPv6Address",
		})
		table.SetAutoWrapText(false)
		table.AppendBulk(tabData)
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	inspectCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
	inspectCmd.Flags().StringVarP(&format, "format", "f", "", "lab name prefix")
}

func toTableData(det []containerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for _, d := range det {
		tabData = append(tabData, []string{d.Name, d.Image, d.Kind, d.Group, d.State, d.IPv4Address, d.IPv6Address})
	}
	return tabData
}
