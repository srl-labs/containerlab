// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/cmd/inspect"
	"github.com/srl-labs/containerlab/cmd/version"
	"github.com/srl-labs/containerlab/core"
	"github.com/srl-labs/containerlab/core/dependency_manager"
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

// skipLabDirFileACLs skips provisioning of extended File ACLs for the Lab directory.
var skipLabDirFileACLs bool

// labOwner flag for setting the owner label.
var labOwner string

// deployCmd represents the deploy command.
var deployCmd = &cobra.Command{
	Use:          "deploy",
	Short:        "deploy a lab",
	Long:         "deploy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/deploy/",
	Aliases:      []string{"dep"},
	SilenceUsage: true,
	PreRunE:      common.CheckAndGetRootPrivs,
	RunE:         deployFn,
}

func init() {
	RootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&common.Graph, "graph", "g", false, "generate topology graph")
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
	deployCmd.Flags().StringSliceVarP(&common.NodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	deployCmd.Flags().BoolVarP(&skipLabDirFileACLs, "skip-labdir-acl", "", false,
		"skip the lab directory extended ACLs provisioning")
	deployCmd.Flags().StringVarP(&labOwner, "owner", "", "",
		"lab owner name (only for users in clab_admins group)")
}

// deployFn function runs deploy sub command.
func deployFn(cobraCmd *cobra.Command, _ []string) error {
	var err error

	log.Info("Containerlab started", "version", version.Version)

	// Check for owner from environment (set by generate command)
	if labOwner == "" && os.Getenv("CLAB_OWNER") != "" {
		labOwner = os.Getenv("CLAB_OWNER")
	}

	opts := []core.ClabOption{
		core.WithTimeout(common.Timeout),
		core.WithTopoPath(common.Topo, common.VarsFile),
		core.WithTopoBackup(common.Topo),
		core.WithNodeFilter(common.NodeFilter),
		core.WithRuntime(
			common.Runtime,
			&runtime.RuntimeConfig{
				Debug:            common.Debug,
				Timeout:          common.Timeout,
				GracefulShutdown: common.Graceful,
			},
		),
		core.WithDependencyManager(dependency_manager.NewDependencyManager()),
		core.WithDebug(common.Debug),
	}

	// process optional settings
	if common.Name != "" {
		opts = append(opts, core.WithLabName(common.Name))
	}
	if labOwner != "" {
		opts = append(opts, core.WithLabOwner(labOwner))
	}
	if mgmtNetName != "" {
		opts = append(opts, core.WithManagementNetworkName(mgmtNetName))
	}
	if v4 := mgmtIPv4Subnet.String(); v4 != "<nil>" {
		opts = append(opts, core.WithManagementIpv4Subnet(v4))
	}
	if v6 := mgmtIPv6Subnet.String(); v6 != "<nil>" {
		opts = append(opts, core.WithManagementIpv6Subnet(v6))
	}

	c, err := core.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	deploymentOptions, err := core.NewDeployOptions(maxWorkers)
	if err != nil {
		return err
	}

	deploymentOptions.SetExportTemplate(exportTemplate).
		SetReconfigure(reconfigure).
		SetGraph(common.Graph).
		SetSkipPostDeploy(skipPostDeploy).
		SetSkipLabDirFileACLs(skipLabDirFileACLs)

	containers, err := c.Deploy(cobraCmd.Context(), deploymentOptions)
	if err != nil {
		return err
	}

	// historically i think this was 5s, but we will already have had at least some time for
	// the manager to have gone off and fetched the version, so 3s max to wrap that up and print
	// seems reasonable
	versionCheckContext, cancel := context.WithTimeout(cobraCmd.Context(), 3*time.Second)
	defer cancel()

	m := version.GetManager()
	m.DisplayNewVersionAvailable(versionCheckContext)

	// print table summary
	return inspect.PrintContainerInspect(containers, deployFormat)
}
