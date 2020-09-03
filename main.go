package main

import (
	"context"
	"fmt"

	"github.com/wim-srl/container-lab/clab"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/pflag"
)

var debug bool

func main() {
	// Flags variables + initialization
	var topo string
	var action string
	var graph bool
	var certGen bool
	pflag.StringVarP(&topo, "topo", "t", "/etc/containerlab/lab-examples/wan-topo.yml", "YAML file with topology information")
	pflag.StringVarP(&action, "action", "a", "", "action: deploy or destroy")
	pflag.BoolVarP(&graph, "graph", "g", false, "generate a graph of the topology")
	pflag.BoolVarP(&certGen, "gen-certs", "c", true, "generate a certificate per container")
	pflag.BoolVarP(&debug, "debug", "d", false, "set log level to debug")
	pflag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	}
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

	switch action {
	case "deploy":
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

		//show management ip addresses per Node
		for dutName, node := range c.Nodes {
			log.Info(fmt.Sprintf("Mgmt IP addresses of container: %s, IPv4: %s, IPv6: %s, MAC: %s", dutName, node.MgmtIPv4, node.MgmtIPv6, node.MgmtMac))
		}

	case "destroy":
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
	default:
		// undefined action
		log.Info("Empty action, if you want to deploy or destoy command should be ./containerlab -a deploy|destroy")
		if graph {
			log.Info("Generating lab graph ...")
			if err = c.GenerateGraph(topo); err != nil {
				log.Error(err)
			}
		}
	}
}
