// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	postDeployVersionCheckTimeout = 3 * time.Second
)

func deployCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   "deploy",
		Short: "deploy a lab",
		Long: "deploy a lab based defined by means of the topology definition " +
			"file\nreference: https://containerlab.dev/cmd/deploy/",
		Aliases:      []string{"dep"},
		SilenceUsage: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return deployFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(
		&o.Deploy.GenerateGraph,
		"graph",
		"g",
		o.Deploy.GenerateGraph,
		"generate topology graph",
	)

	c.Flags().StringVarP(
		&o.Deploy.ManagementNetworkName,
		"network",
		"",
		o.Deploy.ManagementNetworkName,
		"management network name",
	)

	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv4Subnet,
		"ipv4-subnet",
		"4",
		o.Deploy.ManagementIPv4Subnet,
		"management network IPv4 subnet range",
	)

	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv6Subnet,
		"ipv6-subnet",
		"6",
		o.Deploy.ManagementIPv6Subnet,
		"management network IPv6 subnet range",
	)

	c.Flags().StringVarP(
		&o.Inspect.Format,
		"format",
		"f",
		o.Inspect.Format,
		"output format. One of [table, json]",
	)

	c.Flags().BoolVarP(
		&o.Deploy.Reconfigure,
		"reconfigure",
		"c",
		o.Deploy.Reconfigure,
		"regenerate configuration artifacts and overwrite previous ones if any",
	)

	c.Flags().UintVarP(
		&o.Deploy.MaxWorkers,
		"max-workers",
		"",
		o.Deploy.MaxWorkers,
		"limit the maximum number of workers creating nodes and virtual wires",
	)

	c.Flags().BoolVarP(
		&o.Deploy.SkipPostDeploy,
		"skip-post-deploy", "",
		o.Deploy.SkipPostDeploy,
		"skip post deploy action",
	)

	c.Flags().StringVarP(
		&o.Deploy.ExportTemplate,
		"export-template",
		o.Deploy.ExportTemplate,
		"",
		"template file for topology data export",
	)

	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)

	c.Flags().BoolVarP(
		&o.Deploy.SkipLabDirectoryFileACLs,
		"skip-labdir-acl",
		"",
		o.Deploy.SkipLabDirectoryFileACLs,
		"skip the lab directory extended ACLs provisioning",
	)

	c.Flags().StringVarP(
		&o.Deploy.LabOwner,
		"owner",
		"",
		o.Deploy.LabOwner,
		"lab owner name (only for users in clab_admins group)",
	)

	return c, nil
}

// deployFn function runs deploy sub command.
func deployFn(cobraCmd *cobra.Command, o *Options) error {
	// when deploying we cleanup if root context is canceled
	o.Global.CleanOnCancel = true

	var err error

	log.Info("Containerlab started", "version", Version)

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	deploymentOptions, err := clabcore.NewDeployOptions(o.Deploy.MaxWorkers)
	if err != nil {
		return err
	}

	deploymentOptions.SetExportTemplate(o.Deploy.ExportTemplate).
		SetReconfigure(o.Deploy.Reconfigure).
		SetGraph(o.Deploy.GenerateGraph).
		SetSkipPostDeploy(o.Deploy.SkipPostDeploy).
		SetSkipLabDirFileACLs(o.Deploy.SkipLabDirectoryFileACLs)

	containers, err := c.Deploy(cobraCmd.Context(), deploymentOptions)
	if err != nil {
		return err
	}

	// historically i think this was 5s, but we will already have had at least some time for
	// the manager to have gone off and fetched the version, so 3s max to wrap that up and print
	// seems reasonable
	versionCheckContext, cancel := context.WithTimeout(
		cobraCmd.Context(),
		postDeployVersionCheckTimeout,
	)
	defer cancel()

	m := getVersionManager()
	m.DisplayNewVersionAvailable(versionCheckContext)

	// print table summary
	return PrintContainerInspect(containers, o)
}
