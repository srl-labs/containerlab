package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// path to the topology file
var topo string

var graph bool

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy a lab",

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

		// create lab directory
		log.Info("Creating container lab directory: ", topo)
		clab.CreateDirectory(c.Dir.Lab, 0755)

		// create root CA
		log.Info("Creating root CA")
		if err = c.CreateRootCA(); err != nil {
			log.Error(err)
		}

		// create bridge
		log.Info("Creating docker bridge")
		if err = c.CreateBridge(ctx); err != nil {
			log.Error(err)
		}

		// create directory structure and container per node
		for shortDutName, node := range c.Nodes {
			// create CERT
			log.Info("Creating CA for dut: ", shortDutName)
			if err = c.CreateCERT(shortDutName); err != nil {
				log.Error(err)
			}

			log.Info("Create directory structure:", shortDutName)
			if err = c.CreateNodeDirStructure(node, shortDutName); err != nil {
				log.Error(err)
			}

			log.Info("Create container:", shortDutName)
			if err = c.CreateContainer(ctx, shortDutName, node); err != nil {
				log.Error(err)
			}
		}
		// wire the links between the nodes based on cabling plan
		for i, link := range c.Links {
			if err = c.CreateVirtualWiring(i, link); err != nil {
				log.Error(err)
			}

		}
		// generate graph of the lab topology
		if graph {
			log.Info("Generating lab graph ...")
			if err = c.GenerateGraph(topo); err != nil {
				log.Error(err)
			}
		}

		// show management ip addresses per Node
		for dutName, node := range c.Nodes {
			log.Info(fmt.Sprintf("Mgmt IP addresses of container: %s, IPv4: %s, IPv6: %s, MAC: %s", dutName, node.MgmtIPv4, node.MgmtIPv6, node.MgmtMac))
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	deployCmd.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
}
