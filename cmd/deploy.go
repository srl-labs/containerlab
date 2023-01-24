// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/utils"
)

const (
	// file name of a topology export data.
	topoExportFName            = "topology-data.json"
	defaultExportTemplateFPath = "/etc/containerlab/templates/export/auto.tmpl"
)

// name of the container management network.
var mgmtNetName string

// IPv4/6 address range for container management network.
var mgmtIPv4Subnet net.IPNet
var mgmtIPv6Subnet net.IPNet

// reconfigure flag.
var reconfigure bool

// max-workers flag.
var maxWorkers uint

// skipPostDeploy flag.
var skipPostDeploy bool

// template file for topology data export.
var exportTemplate string

var deployFormat string

// deployCmd represents the deploy command.
var deployCmd = &cobra.Command{
	Use:          "deploy",
	Short:        "deploy a lab",
	Long:         "deploy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/deploy/",
	Aliases:      []string{"dep"},
	SilenceUsage: true,
	PreRunE:      sudoCheck,
	RunE:         deployFn,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	deployCmd.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	deployCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	deployCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	deployCmd.Flags().StringVarP(&deployFormat, "format", "f", "table", "output format. One of [table, json]")
	deployCmd.Flags().BoolVarP(&reconfigure, "reconfigure", "c", false,
		"regenerate configuration artifacts and overwrite previous ones if any")
	deployCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers creating nodes and virtual wires")
	deployCmd.Flags().BoolVarP(&skipPostDeploy, "skip-post-deploy", "", false, "skip post deploy action")
	deployCmd.Flags().StringVarP(&exportTemplate, "export-template", "",
		defaultExportTemplateFPath, "template file for topology data export")
}

// deployFn function runs deploy sub command.
func deployFn(_ *cobra.Command, _ []string) error {
	var err error

	log.Infof("Containerlab v%s started", version)

	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
		clab.WithTopoFile(topo, varsFile),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
	}
	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setFlags(c.Config)
	log.Debugf("lab Conf: %+v", c.Config)

	// dispatch a version check that will run in background
	vCh := getLatestClabVersion()

	if reconfigure {
		if err != nil {
			return err
		}
		_ = destroyLab(ctx, c)
		log.Infof("Removing %s directory...", c.Dir.Lab)
		if err := os.RemoveAll(c.Dir.Lab); err != nil {
			return err
		}
	}

	if err = c.CheckTopologyDefinition(ctx); err != nil {
		return err
	}

	if err = c.CheckResources(); err != nil {
		return err
	}

	log.Info("Creating lab directory: ", c.Dir.Lab)
	utils.CreateDirectory(c.Dir.Lab, 0755)

	// create an empty ansible inventory file that will get populated later
	// we create it here first, so that bind mounts of ansible-inventory.yml file could work
	ansibleInvFPath := filepath.Join(c.Dir.Lab, "ansible-inventory.yml")
	_, err = os.Create(ansibleInvFPath)
	if err != nil {
		return err
	}

	// in an similar fashion, create an empty topology data file
	topoDataFPath := filepath.Join(c.Dir.Lab, topoExportFName)
	topoDataF, err := os.Create(topoDataFPath)
	if err != nil {
		return err
	}

	// define the attributes used to generate the Root-CA Cert
	caCertInput := &cert.CsrInputCa{
		Prefix: c.Config.Name,
	}
	if err := c.LoadOrGenerateRootCA(caCertInput); err != nil {
		return err
	}

	if err := c.GenerateMissingNodeCerts(); err != nil {
		return err
	}

	if err := c.CreateAuthzKeysFile(); err != nil {
		return err
	}

	// create management network or use existing one
	if err = c.CreateNetwork(ctx); err != nil {
		return err
	}

	nodeWorkers := uint(len(c.Nodes))
	linkWorkers := uint(len(c.Links))

	if maxWorkers > 0 && maxWorkers < nodeWorkers {
		nodeWorkers = maxWorkers
	}

	if maxWorkers > 0 && maxWorkers < linkWorkers {
		linkWorkers = maxWorkers
	}

	// extraHosts holds host entries for nodes with static IPv4/6 addresses
	// these entries will be used by container runtime to populate /etc/hosts file
	extraHosts := make([]string, 0, len(c.Nodes))

	for _, n := range c.Nodes {
		if n.Config().MgmtIPv4Address != "" {
			log.Debugf("Adding static ipv4 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv4Address)
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv4Address)
		}

		if n.Config().MgmtIPv6Address != "" {
			log.Debugf("Adding static ipv6 /etc/hosts entry for %s:%s",
				n.Config().ShortName, n.Config().MgmtIPv6Address)
			extraHosts = append(extraHosts, n.Config().ShortName+":"+n.Config().MgmtIPv6Address)
		}
	}

	for _, n := range c.Nodes {
		n.Config().ExtraHosts = extraHosts
	}

	nodesWg, err := c.CreateNodes(ctx, nodeWorkers)
	if err != nil {
		return err
	}
	c.CreateLinks(ctx, linkWorkers)
	if nodesWg != nil {
		nodesWg.Wait()
	}

	log.Debug("containers created, retrieving state and IP addresses...")
	// updating nodes with runtime information such as IP addresses assigned by the runtime dynamically
	for _, n := range c.Nodes {
		err = n.UpdateConfigWithRuntimeInfo(ctx)
		if err != nil {
			log.Errorf("failed to update node runtime infromation for node %s: %v", n.Config().ShortName, err)
		}
	}

	if err := c.GenerateInventories(); err != nil {
		return err
	}

	if err := c.GenerateExports(topoDataF, exportTemplate); err != nil {
		return err
	}

	if !skipPostDeploy {
		wg := &sync.WaitGroup{}
		wg.Add(len(c.Nodes))

		for _, node := range c.Nodes {
			go func(node nodes.Node, wg *sync.WaitGroup) {
				defer wg.Done()
				err := node.PostDeploy(ctx, c.Nodes)
				if err != nil {
					log.Errorf("failed to run postdeploy task for node %s: %v", node.Config().ShortName, err)
				}
			}(node, wg)
		}
		wg.Wait()
	}

	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return err
	}

	// generate graph of the lab topology
	if graph {
		if err = c.GenerateGraph(topo); err != nil {
			log.Error(err)
		}
	}

	log.Info("Adding containerlab host entries to /etc/hosts file")
	err = clab.AppendHostsFileEntries(containers, c.Config.Name)
	if err != nil {
		log.Errorf("failed to create hosts file: %v", err)
	}

	// execute commands specified for nodes with `exec` node parameter
	execCollection := exec.NewExecCollection()
	for _, n := range c.Nodes {
		for _, e := range n.Config().Exec {
			exec, err := exec.NewExecCmdFromString(e)
			if err != nil {
				log.Warnf("Failed to parse the command string: %s, %v", e, err)
			}

			res, err := n.RunExec(ctx, exec)
			if err != nil {
				// kinds which do not support exec functionality are skipped
				continue
			}

			execCollection.Add(n.Config().ShortName, res)
		}
	}

	// write to log
	execCollection.Log()

	// log new version availability info if ready
	newVerNotification(vCh)

	// print table summary
	return printContainerInspect(containers, deployFormat)
}

func setFlags(conf *clab.Config) {
	if name != "" {
		conf.Name = name
	}
	if mgmtNetName != "" {
		conf.Mgmt.Network = mgmtNetName
	}
	if v4 := mgmtIPv4Subnet.String(); v4 != "<nil>" {
		conf.Mgmt.IPv4Subnet = v4
	}
	if v6 := mgmtIPv6Subnet.String(); v6 != "<nil>" {
		conf.Mgmt.IPv6Subnet = v6
	}
}
