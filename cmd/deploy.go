package cmd

import (
	"context"
	"net"
	"path"
	"text/template"

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
		tpl, err := template.ParseFiles(rootCaCsrTemplate)
		if err != nil {
			log.Fatalf("failed to parse rootCACsrTemplate: %v", err)
		}
		rootCerts, err := c.GenerateRootCa(tpl, clab.CaRootInput{Prefix: c.Conf.Prefix})
		if err != nil {
			log.Fatalf("failed to generate rootCa: %v", err)
		}
		if debug {
			log.Debugf("root CSR: %s", string(rootCerts.Csr))
			log.Debugf("root Cert: %s", string(rootCerts.Cert))
			log.Debugf("root Key: %s", string(rootCerts.Key))
		}

		// create bridge
		if err = c.CreateBridge(ctx); err != nil {
			log.Error(err)
		}

		certTpl, err := template.ParseFiles(certCsrTemplate)
		if err != nil {
			log.Fatalf("failed to parse certCsrTemplate: %v", err)
		}
		// create directory structure and container per node
		for shortDutName, node := range c.Nodes {
			// create CERT
			certIn := clab.CertInput{
				Name:     shortDutName,
				LongName: node.LongName,
				Fqdn:     node.Fqdn,
				Prefix:   c.Conf.Prefix,
			}
			nodeCerts, err := c.GenerateCert(
				path.Join(c.Dir.LabCARoot, "root-ca.pem"),
				path.Join(c.Dir.LabCARoot, "root-ca-key.pem"),
				certTpl,
				certIn,
			)
			if err != nil {
				log.Errorf("failed to generate certificates for node %s: %v", shortDutName, err)
			}
			if debug {
				log.Debugf("%s CSR: %s", shortDutName, string(nodeCerts.Csr))
				log.Debugf("%s Cert: %s", shortDutName, string(nodeCerts.Cert))
				log.Debugf("%s Key: %s", shortDutName, string(nodeCerts.Key))
			}
			if err = c.CreateNodeDirStructure(node, shortDutName); err != nil {
				log.Error(err)
			}

			if err = c.CreateContainer(ctx, shortDutName, node); err != nil {
				log.Error(err)
			}
		}
		// cleanup hanging resources if a deployment failed before
		c.InitVirtualWiring()
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
	deployCmd.Flags().StringVarP(&bridge, "bridge", "b", "", "docker network name for management")
	deployCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
	deployCmd.Flags().IPNetVarP(&ipv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	deployCmd.Flags().IPNetVarP(&ipv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
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
