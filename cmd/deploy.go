// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/utils"
	"github.com/tklauser/numcpus"
)

const (
	// file name of a topology export data.
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

// subset of nodes to work with.
var nodeFilter []string

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
	deployCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
}

// deployFn function runs deploy sub command.
func deployFn(_ *cobra.Command, _ []string) error {
	var err error

	log.Infof("Containerlab v%s started", version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// handle CTRL-C signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		log.Errorf("Caught CTRL-C. Stopping deployment and cleaning up!")
		cancel()

		// when interrupted, destroy the interrupted lab deployment with cleanup
		cleanup = true
		if err := destroyFn(destroyCmd, []string{}); err != nil {
			log.Errorf("Failed to destroy lab: %v", err)
		}

		os.Exit(1) // skipcq: RVV-A0003
	}()

	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
		clab.WithTopoFile(topo, varsFile),
		clab.WithNodeFilter(nodeFilter),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
		clab.WithDebug(debug),
	}
	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	setFlags(c.Config)
	log.Debugf("lab Conf: %+v", c.Config)

	// dispatch a version check that will run in background
	vCh := getLatestClabVersion(ctx)

	if reconfigure {
		if err != nil {
			return err
		}
		_ = destroyLab(ctx, c)
		log.Infof("Removing %s directory...", c.TopoPaths.TopologyLabDir())
		if err := os.RemoveAll(c.TopoPaths.TopologyLabDir()); err != nil {
			return err
		}
	}

	if err = c.CheckTopologyDefinition(ctx); err != nil {
		return err
	}

	log.Info("Creating lab directory: ", c.TopoPaths.TopologyLabDir())
	utils.CreateDirectory(c.TopoPaths.TopologyLabDir(), 0755)

	// create an empty ansible inventory file that will get populated later
	// we create it here first, so that bind mounts of ansible-inventory.yml file could work
	ansibleInvFPath := c.TopoPaths.AnsibleInventoryFileAbsPath()
	_, err = os.Create(ansibleInvFPath)
	if err != nil {
		return err
	}

	// in an similar fashion, create an empty topology data file
	topoDataFPath := c.TopoPaths.TopoExportFile()
	topoDataF, err := os.Create(topoDataFPath)
	if err != nil {
		return err
	}

	// define the attributes used to generate the CA Cert
	caCertInput := &cert.CACSRInput{
		CommonName:   c.Config.Name + " lab CA",
		Expiry:       "87600h",
		Organization: "containerlab",
	}

	if err := c.LoadOrGenerateCA(caCertInput); err != nil {
		return err
	}

	if err := c.CreateAuthzKeysFile(); err != nil {
		return err
	}

	// create management network or use existing one
	if err = c.CreateNetwork(ctx); err != nil {
		return err
	}

	// determine the number of node and link worker
	nodeWorkers, linkWorkers, err := countWorkers(uint(len(c.Nodes)), uint(len(c.Links)), maxWorkers)
	if err != nil {
		return err
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

				err := node.PostDeploy(ctx, &nodes.PostDeployParams{Nodes: c.Nodes})
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

// countWorkers calculates the number workers used for the creation of links and nodes.
// If a user provided --max-workers value it is used for both links and nodes workers.
// If maxWorkers is not set then the workers are limited by the number of available CPUs when
// number of nodes/links exceeds the number of available CPUs.
func countWorkers(nodeCount, linkCount, maxWorkers uint) (nodeWorkers, linkWorkers uint, err error) {
	// init number of Workers to the number of links and nodes
	nodeWorkers = nodeCount
	linkWorkers = linkCount

	switch {
	// if maxWorkers is not set, limit workers number by number of available CPUs
	case maxWorkers <= 0:
		// retrieve vCPU count
		vCpus, err := numcpus.GetOnline()
		if err != nil {
			return 0, 0, err
		}

		numCPUs := uint(vCpus)

		// limit node/link workers only if there is more node/links thans CPU cores available
		if nodeCount > numCPUs {
			nodeWorkers = numCPUs
		}

		if linkCount > numCPUs {
			linkWorkers = numCPUs
		}
	case maxWorkers > 0:
		nodeWorkers = maxWorkers
		linkWorkers = maxWorkers
	}

	log.Debugf("Number of Node workers: %d, Number of Link workers: %d", nodeWorkers, linkWorkers)

	return nodeWorkers, linkWorkers, nil
}
