package main

import (
	"context"
	"fmt"

	"docker.io/go-docker"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/pflag"
)

type cLab struct {
	Conf         *conf
	FileInfo     *File
	Nodes        map[string]*Node
	Links        map[int]*Link
	DockerClient *docker.Client
	Dir          *cLabDirectory
}

type cLabDirectory struct {
	Lab       string
	LabCA     string
	LabCARoot string
	LabGraph  string
}

func newContainerLab() (*cLab, error) {
	c := &cLab{
		Conf:     new(conf),
		FileInfo: new(File),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
	}
	var err error
	c.DockerClient, err = docker.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func main() {
	// Flags variables + initialization
	var topo string
	var action string
	var graph bool
	var debug bool
	var certGen bool
	pflag.StringVarP(&topo, "topo", "t", "labs/wan-topo.yml", "YAML file with topology information")
	pflag.StringVarP(&action, "action", "a", "", "action: deploy or destroy")
	pflag.BoolVarP(&graph, "graph", "g", false, "generate a graph of the topology")
	pflag.BoolVarP(&certGen, "gen-certs", "c", true, "generate a certificate per container")
	pflag.BoolVarP(&debug, "debug", "d", false, "set log level to debug")
	pflag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	}
	c, err := newContainerLab()
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Getting topology information ...")
	if err = c.getTopology(&topo); err != nil {
		log.Fatal(err)
	}

	// Parse topology information
	log.Info("Parsing topology information ...")
	if err = c.parseTopology(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch action {
	case "deploy":
		log.Info("Creating container lab: ", topo)
		// create lab directory
		createDirectory(c.Dir.Lab, 0755)

		// create root CA
		log.Info("Creating root CA")
		if err = c.createRootCA(); err != nil {
			log.Error(err)
		}

		log.Info("Creating docker bridge")
		// create bridge
		if err = c.createBridge(ctx); err != nil {
			log.Error(err)
		}

		// create directory structure and container per node
		for shortDutName, node := range c.Nodes {
			// create CERT
			log.Info("Creating CA for dut: ", shortDutName)
			if err = c.createCERT(shortDutName); err != nil {
				log.Error(err)
			}

			log.Info("Create directory structure:", shortDutName)
			if err = c.createNodeDirStructure(node, shortDutName); err != nil {
				log.Error(err)
			}

			log.Info("Create contaianer:", shortDutName)
			if err = c.createContainer(ctx, shortDutName, node); err != nil {
				log.Error(err)
			}
		}
		// wire the links between the nodes based on cabling plan
		for i, link := range c.Links {
			log.Info("Wire link between containers :", link.a.EndpointName, link.b.EndpointName)
			if err = c.createVirtualWiring(i, link); err != nil {
				log.Error(err)
			}

		}
		// generate graph of the lab topology
		if graph {
			log.Info("Generating lab graph ...")
			if err = c.generateGraph(topo); err != nil {
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
			if err = c.deleteContainer(ctx, shortDutName, node); err != nil {
				log.Error(err)
			}
		}
		// delete container management bridge
		log.Info("Deleting docker bridge ...")
		if err = c.deleteBridge(ctx); err != nil {
			log.Error(err)
		}
		// delete virtual wiring
		for n, link := range c.Links {
			log.Info("Delete virtual wire:", link.a.EndpointName, link.b.EndpointName)
			if err = c.deleteVirtualWiring(n, link); err != nil {
				log.Error(err)
			}

		}
	default:
		// undefined action
		log.Info("Empty action, if you want to deploy or destoy command should be ./containerlab -a deploy|destroy")
		if graph {
			log.Info("Generating lab graph ...")
			if err = c.generateGraph(topo); err != nil {
				log.Error(err)
			}
		}
	}
}
