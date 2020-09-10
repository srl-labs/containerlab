package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

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
		b, err := json.MarshalIndent(containers, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal containers struct: %v", err)
		}
		fmt.Println(string(b))
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	inspectCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
}
