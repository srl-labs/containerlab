package main

import (
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
	pflag.StringVarP(&topo, "topo", "t", "labs/wan-topo.yml", "YAML file with topology information")
	pflag.StringVarP(&action, "action", "a", "", "action: deploy or destroy")
	pflag.BoolVarP(&graph, "graph", "g", false, "generate a graph of the topology")
	pflag.BoolVarP(&debug, "debug", "d", false, "set log level to debug")
	pflag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	c, err := newContainerLab()
	if err != nil {
		log.Fatalf("failed initializing: %v", err)
	}
	// Get

	log.Info("Getting topology information ...")
	if err = c.getTopology(&topo); err != nil {
		panic(err)
	}

	log.Info("Parsing topology information ...")
	if err = c.parseTopology(); err != nil {
		panic(err)
	}

	// var d *Docker
	// if d, err = NewDocker(); err != nil {
	// 	panic(err)
	// }

	switch action {
	case "deploy":
		log.Info("Creating container lab: ", topo)
		// create lab directory
		path := c.Conf.ConfigPath + "/" + "lab" + "-" + c.Conf.Prefix
		createDirectory(path, 0755)

		log.Info("Creating docker bridge")
		// create bridge
		if err = c.createBridge(); err != nil {
			log.Error(err)
		}

		// create directory structure and container per node
		for dutName, node := range c.Nodes {
			log.Info("Create directory structure and create container:", dutName)
			if err = c.createNodeDirStructure(node, dutName); err != nil {
				log.Error(err)
			}

			if err = c.createContainer(dutName, node); err != nil {
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
		// start container per node
		// for dutName, node := range Nodes {
		// 	log.Info("Start container:", dutName)

		// 	if err = d.startContainer(dutName, node); err != nil {
		// 		log.Error(err)
		// 	}
		//}
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
		for n, node := range c.Nodes {
			if err = c.deleteContainer(n, node); err != nil {
				log.Error(err)
			}
		}
		// delete container management bridge
		log.Info("Deleting docker bridge ...")
		if err = c.deleteBridge(); err != nil {
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
