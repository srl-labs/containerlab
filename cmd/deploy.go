package cmd

import (
	"context"
	"fmt"
	"net"
	"path"
	"sync"
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
	Use:          "deploy",
	Short:        "deploy a lab",
	Aliases:      []string{"dep"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := clab.NewContainerLab(debug)
		err := c.Init(timeout)
		if err != nil {
			return err
		}

		if err = c.GetTopology(&topo); err != nil {
			return err
		}
		setFlags(c.Conf)
		log.Debugf("lab Conf: %+v", c.Conf)
		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// create lab directory
		log.Info("Creating lab directory: ", c.Dir.Lab)
		clab.CreateDirectory(c.Dir.Lab, 0755)

		// create root CA
		tpl, err := template.ParseFiles(rootCaCsrTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse rootCACsrTemplate: %v", err)
		}
		rootCerts, err := c.GenerateRootCa(tpl, clab.CaRootInput{Prefix: c.Conf.Prefix})
		if err != nil {
			return fmt.Errorf("failed to generate rootCa: %v", err)
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
			return fmt.Errorf("failed to parse certCsrTemplate: %v", err)
		}
		// create directory structure and container per node
		wg := new(sync.WaitGroup)
		wg.Add(len(c.Nodes))
		for _, node := range c.Nodes {
			go func(node *clab.Node) {
				defer wg.Done()
				if node.Kind == "bridge" {
					return
				}
				// create CERT
				nodeCerts, err := c.GenerateCert(
					path.Join(c.Dir.LabCARoot, "root-ca.pem"),
					path.Join(c.Dir.LabCARoot, "root-ca-key.pem"),
					certTpl,
					node,
				)
				if err != nil {
					log.Errorf("failed to generate certificates for node %s: %v", node.ShortName, err)
				}
				log.Debugf("%s CSR: %s", node.ShortName, string(nodeCerts.Csr))
				log.Debugf("%s Cert: %s", node.ShortName, string(nodeCerts.Cert))
				log.Debugf("%s Key: %s", node.ShortName, string(nodeCerts.Key))
				err = c.CreateNode(ctx, node, nodeCerts)
				if err != nil {
					log.Errorf("failed to create node %s: %v", node.ShortName, err)
				}
			}(node)
		}
		wg.Wait()
		err = c.SetNodesDetails(ctx)
		if err != nil {
			return err
		}
		// cleanup hanging resources if a deployment failed before
		c.InitVirtualWiring()
		wg = new(sync.WaitGroup)
		wg.Add(len(c.Links))
		// wire the links between the nodes based on cabling plan
		for _, link := range c.Links {
			go func(link *clab.Link) {
				defer wg.Done()
				if err = c.CreateVirtualWiring(link); err != nil {
					log.Error(err)
				}
			}(link)
		}
		wg.Wait()
		// generate graph of the lab topology
		if graph {
			if err = c.GenerateGraph(topo); err != nil {
				log.Error(err)
			}
		}

		// show topology output
		c.CreateHostsFile()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	deployCmd.Flags().StringVarP(&bridge, "bridge", "b", "", "docker network name for management")
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
