package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "destroy a lab",

	Run: func(cmd *cobra.Command, args []string) {
		c, err := clab.NewContainerLab(debug)
		if err != nil {
			log.Fatal(err)
		}

		log.Info("Getting topology information ...")
		if err = c.GetTopology(&topo); err != nil {
			log.Fatal(err)
		}

		// Parse topology information
		log.Info("Parsing topology information ...")
		if err = c.ParseTopology(); err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Info("Destroying container lab: ... ", topo)
		// delete containers
		for shortDutName, node := range c.Nodes {
			if err = c.DeleteContainer(ctx, shortDutName, node); err != nil {
				log.Error(err)
			}
		}
		// delete container management bridge
		log.Info("Deleting docker bridge ...")
		if err = c.DeleteBridge(ctx); err != nil {
			log.Error(err)
		}
		// delete virtual wiring
		for n, link := range c.Links {
			if err = c.DeleteVirtualWiring(n, link); err != nil {
				log.Error(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
}
