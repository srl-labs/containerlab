// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/dependency_manager"
	"github.com/srl-labs/containerlab/runtime"
)

// name of the container management network.
var mgmtNetName string

// IPv4/6 address range for container management network.
var (
	mgmtIPv4Subnet net.IPNet
	mgmtIPv6Subnet net.IPNet
)

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

// skipLabDirFileACLs skips provisioning of extended File ACLs for the Lab directory.
var skipLabDirFileACLs bool

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
		"", "template file for topology data export")
	deployCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	deployCmd.Flags().BoolVarP(&skipLabDirFileACLs, "skip-labdir-acl", "", false,
		"skip the lab directory extended ACLs provisioning")
}

// deployFn function runs deploy sub command.
func deployFn(_ *cobra.Command, _ []string) error {
	var err error

	log.Infof("Containerlab v%s started", version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupCTRLCHandler(cancel)

	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
		clab.WithTopoPath(topo, varsFile),
		clab.WithNodeFilter(nodeFilter),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
		clab.WithDependencyManager(dependency_manager.NewDependencyManager()),
		clab.WithDebug(debug),
	}

	// process optional settings
	if name != "" {
		opts = append(opts, clab.WithLabName(name))
	}
	if mgmtNetName != "" {
		opts = append(opts, clab.WithManagementNetworkName(mgmtNetName))
	}
	if v4 := mgmtIPv4Subnet.String(); v4 != "<nil>" {
		opts = append(opts, clab.WithManagementIpv4Subnet(v4))
	}
	if v6 := mgmtIPv6Subnet.String(); v6 != "<nil>" {
		opts = append(opts, clab.WithManagementIpv6Subnet(v6))
	}

	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	// dispatch a version check that will run in background
	vCh := getLatestClabVersion(ctx)

	deploymentOptions, err := clab.NewDeployOptions(maxWorkers)
	if err != nil {
		return err
	}

	deploymentOptions.SetExportTemplate(exportTemplate).
		SetReconfigure(reconfigure).
		SetGraph(graph).
		SetSkipPostDeploy(skipPostDeploy).
		SetSkipLabDirFileACLs(skipLabDirFileACLs)

	containers, err := c.Deploy(ctx, deploymentOptions)
	if err != nil {
		return err
	}

	// log new version availability info if ready
	newVerNotification(vCh)

	// print table summary
	return printContainerInspect(containers, deployFormat)
}

// setupCTRLCHandler sets-up the handler for CTRL-C
// The deployment will be stopped and a destroy action is
// performed when interrupt signal is received.
func setupCTRLCHandler(cancel context.CancelFunc) {
	// handle CTRL-C signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		log.Errorf("Caught CTRL-C. Stopping deployment!")
		cancel()

		// when interrupted, destroy the interrupted lab deployment
		cleanup = false
		if err := destroyFn(destroyCmd, []string{}); err != nil {
			log.Errorf("Failed to destroy lab: %v", err)
		}

		os.Exit(1) // skipcq: RVV-A0003
	}()
}
