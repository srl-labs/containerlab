// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
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
		Long: "deploy a lab defined by means of the topology definition file; " +
			"a lab that is already deployed is reconciled with the topology instead of " +
			"being recreated\nreference: https://containerlab.dev/cmd/deploy/",
		Aliases:      []string{"dep", "apply"},
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

	c.Flags().BoolVar(
		&o.Deploy.DryRun,
		"dry-run",
		o.Deploy.DryRun,
		"show the planned changes without applying them",
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

	c.Flags().StringVar(
		&o.Deploy.RestoreAll,
		"restore-all",
		"",
		"restore all nodes that have snapshots in this directory (default: ./snapshots)",
	)
	// Allow flag without value to default to ./snapshots
	c.Flags().Lookup("restore-all").NoOptDefVal = "./snapshots"

	c.Flags().StringVarP(
		&o.Deploy.ExportRenderedTopology,
		"export-rendered",
		"",
		"",
		"write the rendered topology YAML (after template and env expansion) to the given file path (required)",
	)

	c.Flags().StringArrayVar(
		&o.Deploy.RestoreNodeSnapshots,
		"restore",
		nil,
		"restore specific node from snapshot file (format: node=path/to/snapshot.tar). "+
			"Can be specified multiple times. Overrides --restore-all for specified nodes.",
	)

	return c, nil
}

// deployFn function runs deploy sub command.
func deployFn(cobraCmd *cobra.Command, o *Options) error {
	if o.Deploy.DryRun && o.Deploy.Reconfigure {
		return fmt.Errorf(
			"--dry-run cannot be combined with --reconfigure: " +
				"reconfigure always destroys and redeploys the full lab",
		)
	}

	o.Global.BackupTopologyFile = !o.Deploy.DryRun

	var err error

	log.Info("Containerlab started", "version", Version)

	clabcore.ExportRenderedTopology = o.Deploy.ExportRenderedTopology

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	// destroy-on-cancel must only be armed when deploy creates the lab from scratch;
	// canceling a reconciliation of an already deployed lab must not destroy it
	if !o.Deploy.DryRun {
		cleanOnCancel := o.Deploy.Reconfigure
		if !cleanOnCancel {
			cleanOnCancel, err = c.NeedsInitialDeploy(cobraCmd.Context())
			if err != nil {
				return err
			}
		}

		o.Global.CleanOnCancel = cleanOnCancel
	}

	deploymentOptions, err := clabcore.NewDeployOptions(o.Deploy.MaxWorkers)
	if err != nil {
		return err
	}

	deploymentOptions.SetExportTemplate(o.Deploy.ExportTemplate).
		SetReconfigure(o.Deploy.Reconfigure).
		SetDryRun(o.Deploy.DryRun).
		SetGraph(o.Deploy.GenerateGraph).
		SetSkipPostDeploy(o.Deploy.SkipPostDeploy).
		SetSkipLabDirFileACLs(o.Deploy.SkipLabDirectoryFileACLs).
		SetRestoreAll(o.Deploy.RestoreAll).
		SetRestoreNodeSnapshots(o.Deploy.RestoreNodeSnapshots)

	result, err := c.Deploy(cobraCmd.Context(), deploymentOptions)
	if err != nil {
		return err
	}

	if o.Deploy.DryRun {
		return printDryRunResult(result.Apply, o)
	}

	// keep stdout machine-readable for non-table formats: the reconciliation summary
	// table is only printed when the inspect output is a table as well
	if result.Apply != nil && o.Inspect.Format == clabconstants.FormatTable {
		printApplyResult(result.Apply)
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
	m.DisplayNewVersionAvailable(versionCheckContext, false)

	// print table summary
	return PrintContainerInspect(result.Containers, o)
}

// printDryRunResult prints the planned changes of a dry run, as JSON when requested via
// the --format flag and as a table otherwise.
func printDryRunResult(result *clabcore.ApplyResult, o *Options) error {
	if o.Inspect.Format == clabconstants.FormatJSON {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))

		return nil
	}

	printApplyResult(result)

	return nil
}

func printApplyResult(result *clabcore.ApplyResult) {
	title := "Apply summary"
	if result.DryRun {
		title = "Apply plan"
	}

	log.Info(title)

	table := tableWriter.NewWriter()
	table.SetOutputMirror(os.Stdout)
	table.SetStyle(tableWriter.StyleRounded)
	table.Style().Format.Header = text.FormatTitle
	table.Style().Format.HeaderAlign = text.AlignCenter
	table.AppendHeader(tableWriter.Row{"Action", "Details"})

	hasRows := false
	if result.DeployedLab {
		label := "deployed lab"
		if result.DryRun {
			label = "deploy lab"
		}
		table.AppendRow(tableWriter.Row{label, result.LabName})
		hasRows = true
	}

	rows := []struct {
		label  string
		values []string
	}{
		{label: "added nodes", values: result.AddedNodes},
		{label: "deleted nodes", values: result.DeletedNodes},
		{label: "recreated nodes", values: withNodeChangeReasons(result.RecreatedNodes, result.NodeChangeReasons)},
		{label: "started nodes", values: result.StartedNodes},
		{label: "added links", values: result.AddedLinks},
		{label: "deleted endpoints", values: result.DeletedEndpoints},
		{label: "restarted nodes", values: withNodeChangeReasons(result.RestartedNodes, result.NodeChangeReasons)},
	}

	for _, row := range rows {
		if appendApplyResultRows(table, row.label, row.values) {
			hasRows = true
		}
	}

	if !hasRows {
		table.AppendRow(tableWriter.Row{"no changes", "-"})
	}

	table.Render()
}

func withNodeChangeReasons(nodeNames []string, reasons map[string]string) []string {
	if len(reasons) == 0 {
		return nodeNames
	}

	values := make([]string, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		if reason, ok := reasons[nodeName]; ok && reason != "" {
			values = append(values, fmt.Sprintf("%s (%s)", nodeName, reason))
			continue
		}
		values = append(values, nodeName)
	}

	return values
}

func appendApplyResultRows(table tableWriter.Writer, label string, values []string) bool {
	if len(values) == 0 {
		return false
	}

	for _, value := range values {
		table.AppendRow(tableWriter.Row{label, value})
	}

	return true
}
