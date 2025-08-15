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
	clabcore "github.com/srl-labs/containerlab/core"
	clabcoredependency_manager "github.com/srl-labs/containerlab/core/dependency_manager"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func deployCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:          "deploy",
		Short:        "deploy a lab",
		Long:         "deploy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/deploy/",
		Aliases:      []string{"dep"},
		SilenceUsage: true,
		PreRunE:      clabutils.CheckAndGetRootPrivs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return deployFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	c.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	c.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	c.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	c.Flags().StringVarP(&deployFormat, "format", "f", "table", "output format. One of [table, json]")
	c.Flags().BoolVarP(&reconfigure, "reconfigure", "c", false,
		"regenerate configuration artifacts and overwrite previous ones if any")
	c.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers creating nodes and virtual wires")
	c.Flags().BoolVarP(&skipPostDeploy, "skip-post-deploy", "", false, "skip post deploy action")
	c.Flags().StringVarP(&exportTemplate, "export-template", "",
		"", "template file for topology data export")
	c.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	c.Flags().BoolVarP(&skipLabDirFileACLs, "skip-labdir-acl", "", false,
		"skip the lab directory extended ACLs provisioning")
	c.Flags().StringVarP(&labOwner, "owner", "", "",
		"lab owner name (only for users in clab_admins group)")

	return c, nil
}

// deployFn function runs deploy sub command.
func deployFn(cobraCmd *cobra.Command, o *Options) error {
	var err error

	log.Info("Containerlab started", "version", Version)

	// Check for owner from environment (set by generate command)
	if labOwner == "" && os.Getenv("CLAB_OWNER") != "" {
		labOwner = os.Getenv("CLAB_OWNER")
	}

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(o.Global.Timeout),
		clabcore.WithTopoPath(o.Global.TopologyFile, o.Global.VarsFile),
		clabcore.WithTopoBackup(o.Global.TopologyFile),
		clabcore.WithNodeFilter(nodeFilter),
		clabcore.WithRuntime(
			o.Global.Runtime,
			&clabruntime.RuntimeConfig{
				Debug:            o.Global.DebugCount > 0,
				Timeout:          o.Global.Timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDependencyManager(clabcoredependency_manager.NewDependencyManager()),
		clabcore.WithDebug(o.Global.DebugCount > 0),
	}

	// process optional settings
	if o.Global.TopologyName != "" {
		opts = append(opts, clabcore.WithLabName(o.Global.TopologyName))
	}
	if labOwner != "" {
		opts = append(opts, clabcore.WithLabOwner(labOwner))
	}
	if mgmtNetName != "" {
		opts = append(opts, clabcore.WithManagementNetworkName(mgmtNetName))
	}
	if v4 := mgmtIPv4Subnet.String(); v4 != "<nil>" {
		opts = append(opts, clabcore.WithManagementIpv4Subnet(v4))
	}
	if v6 := mgmtIPv6Subnet.String(); v6 != "<nil>" {
		opts = append(opts, clabcore.WithManagementIpv6Subnet(v6))
	}

	c, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	deploymentOptions, err := clabcore.NewDeployOptions(maxWorkers)
	if err != nil {
		return err
	}

	deploymentOptions.SetExportTemplate(exportTemplate).
		SetReconfigure(reconfigure).
		SetGraph(graph).
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

	m := getVersionManager()
	m.DisplayNewVersionAvailable(versionCheckContext)

	// print table summary
	return PrintContainerInspect(containers, deployFormat)
}
