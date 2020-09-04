package cmd

import (
	"context"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// path to the topology file
var topo string
var graph bool
var bridge string
var prefix string
var ipv4Subnet net.IPNet
var ipv6Subnet net.IPNet

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy a lab",

	Run: func(cmd *cobra.Command, args []string) {
		c := clab.NewContainerLab(debug)
		err := c.Init()
		if err != nil {
			log.Fatal(err)
		}

		if err = c.GetTopology(&topo); err != nil {
			log.Fatal(err)
		}
		setFlags(c.Conf)
		log.Debugf("lab Conf: %+v", c.Conf)
		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// create lab directory
		log.Info("Creating container lab directory: ", topo)
		clab.CreateDirectory(c.Dir.Lab, 0755)

		// create root CA
		if err = c.CreateRootCA(); err != nil {
			log.Error(err)
		}

		// create bridge
		if err = c.CreateBridge(ctx); err != nil {
			log.Error(err)
		}

		// create directory structure and container per node
		for shortDutName, node := range c.Nodes {
			// create CERT

			if err = c.CreateCERT(shortDutName); err != nil {
				log.Error(err)
			}

			if err = c.CreateNodeDirStructure(node, shortDutName); err != nil {
				log.Error(err)
			}

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
			if err = c.GenerateGraph(topo); err != nil {
				log.Error(err)
			}
		}

		// show topology output
		if err = c.CreateLabOutput(); err != nil {
			log.Error(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "path to the file with topology information")
	deployCmd.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	deployCmd.Flags().StringVarP(&bridge, "bridge", "b", "", "path to the file with topology information")
	deployCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "path to the file with topology information")
	deployCmd.Flags().IPNetVarP(&ipv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "path to the file with topology information")
	deployCmd.Flags().IPNetVarP(&ipv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "path to the file with topology information")
}

func setFlags(conf *clab.Conf) {
	if prefix != "" {
		conf.Prefix = prefix
	}
	if bridge != "" {
		conf.DockerInfo.Bridge = bridge
	}
	if ipv4Subnet.String() != "<nil>" {
		conf.DockerInfo.Ipv4Subnet = ipv4Subnet.String()
	}
	if ipv6Subnet.String() != "<nil>" {
		conf.DockerInfo.Ipv6Subnet = ipv6Subnet.String()
	}
}
