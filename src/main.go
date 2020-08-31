package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/pflag"
)

func main() {
	// Flags variables + initialization
	var topo string
	var action string
	var graph bool
	var debug bool
	pflag.StringVarP(&topo, "topo", "t", "labs/wan-topo.yml", "YAML file with topology information")
	pflag.StringVarP(&action, "action", "a", "", "action: deploy or destroy")
	pflag.BoolVarP(&graph, "graph", "g", false, "generate a graph of the topology")
	pflag.BoolVarP(&debug, "debug", "d", false, "set log level to debug")
	pflag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	// Get
	var t *conf
	var err error
	log.Info("Getting topology information ...")
	if t, err = getTopology(&topo); err != nil {
		panic(err)
	}

	log.Info("Parsing topology information ...")
	if err = parseTopology(t); err != nil {
		panic(err)
	}

	switch action {
	case "deploy":
		log.Info("Creating container lab: ", topo)
		// create lab directory
		path := Path + "/" + "lab" + "-" + Prefix
		createDirectory(path, 0755)

		log.Info("Creating docker bridge")
		// create bridge
		if err = d.createBridge(); err != nil {
			log.Error(err)
		}

		// create directory structure and container per node
		for dutName, node := range Nodes {
			log.Info("Create directory structure and create container:", dutName)
			if err = createNodeDirStructure(node, dutName); err != nil {
				log.Error(err)
			}

			if err = d.createContainer(dutName, node); err != nil {
				log.Error(err)
			}
		}
		// wire the links between the nodes based on cabling plan
		for i, link := range Links {
			log.Info("Wire link between containers :", link.a.EndpointName, link.b.EndpointName)
			if err = createVirtualWiring(i, link); err != nil {
				log.Error(err)
			}

		}

		// generate graph of the lab topology
		if graph {
			log.Info("Generating lab graph ...")
			if err = generateGraph(topo); err != nil {
				log.Error(err)
			}
		}

		//show management ip addresses per Node
		for dutName, node := range Nodes {
			log.Info(fmt.Sprintf("Mgmt IP addresses of container: %s, IPv4: %s, IPv6: %s, MAC: %s", dutName, node.MgmtIPv4, node.MgmtIPv6, node.MgmtMac))
		}

	case "destroy":
		log.Info("Destroying container lab: ... ", topo)
		// delete containers
		for n, node := range Nodes {
			if err = d.deleteContainer(n, node); err != nil {
				log.Error(err)
			}
		}
		// delete container management bridge
		log.Info("Deleting docker bridge ...")
		if err = d.deleteBridge(); err != nil {
			log.Error(err)
		}
		// delete virtual wiring
		for n, link := range Links {
			log.Info("Delete virtual wire:", link.a.EndpointName, link.b.EndpointName)
			if err = deleteVirtualWiring(n, link); err != nil {
				log.Error(err)
			}

		}
	default:
		// undefined action
		log.Info("Empty action, if you want to deploy or destoy command should be ./containerlab -a deploy|destroy")
		if graph {
			log.Info("Generating lab graph ...")
			if err = generateGraph(topo); err != nil {
				log.Error(err)
			}
		}
	}

}
