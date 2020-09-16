package cmd

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "destroy a lab",

	Run: func(cmd *cobra.Command, args []string) {
		c := clab.NewContainerLab(debug)
		err := c.Init(timeout)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if prefix != "" {
			filter := filters.NewArgs()
			filter.Add("label", fmt.Sprintf("containerlab=lab-%s", prefix))
			containers, err := c.DockerClient.ContainerList(ctx, types.ContainerListOptions{
				Filters: filter,
			})
			if err != nil {
				log.Fatalf("could not list containers: %v", err)
			}
			var name string
			for _, cont := range containers {
				name = cont.ID
				if len(cont.Names) > 0 {
					name = cont.Names[0]
				}
				log.Infof("Removing container: %s", name)
				err = c.DockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{})
				if err != nil {
					log.Errorf("could not remove container '%s': %v", name, err)
				}
			}
			return
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
		c.InitVirtualWiring()
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	destroyCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
}
