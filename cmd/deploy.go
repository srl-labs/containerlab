// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/log"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabutils "github.com/srl-labs/containerlab/utils"
	"sigs.k8s.io/yaml"
)

const (
	postDeployVersionCheckTimeout = 3 * time.Second

	noTopologyCRFlagHelp = "do not create a clabernetes Topology resource; compile the topology " +
		"client-side and manage the lab's Node, Link, and NodeProfile resources directly " +
		"(clabernetes runtime only)"

	imagePullSecretFlagHelp = "name of the image pull secret populated in the clabernetes " +
		"Topology CR; the secret must exist in the lab namespace, and when unset no pull " +
		"secret is referenced at all (clabernetes runtime only)"

	exposeTypeFlagHelp = "Kubernetes Service type used to expose lab nodes; one of " +
		"ClusterIP, Headless, LoadBalancer, or None (c9s runtime only)"

	noPersistenceFlagHelp = "deploy lab nodes on ephemeral storage instead of the default " +
		"per-node persistent volumes; saved device configuration is then lost on any pod " +
		"replacement, but no dynamically provisionable storage class is required " +
		"(clabernetes runtime only)"

	emitCRsFlagHelp = "print the Kubernetes manifests deploy would create to stdout instead " +
		"of applying them; the Topology resource by default, or the compiled NodeProfile, " +
		"Link, and Node resources with --no-topology-cr, together with the lab Namespace " +
		"and ConfigMaps they depend on (clabernetes runtime only)"
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
			if commandSkipsRoot(o.Global.Runtime) {
				return nil
			}

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

	c.Flags().BoolVar(
		&o.Deploy.NoTopologyCR,
		"no-topology-cr",
		o.Deploy.NoTopologyCR,
		noTopologyCRFlagHelp,
	)

	c.Flags().StringVar(
		&o.Deploy.ImagePullSecret,
		"image-pull-secret",
		o.Deploy.ImagePullSecret,
		imagePullSecretFlagHelp,
	)

	c.Flags().StringVar(
		&o.Deploy.ExposeType,
		"expose-type",
		o.Deploy.ExposeType,
		exposeTypeFlagHelp,
	)

	c.Flags().BoolVar(
		&o.Deploy.NoPersistence,
		"no-persistence",
		o.Deploy.NoPersistence,
		noPersistenceFlagHelp,
	)

	c.Flags().BoolVar(
		&o.Deploy.EmitCRs,
		"emit-crs",
		o.Deploy.EmitCRs,
		emitCRsFlagHelp,
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

	if o.Deploy.NoTopologyCR && !clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return fmt.Errorf("--no-topology-cr is only supported with the %q runtime",
			clablabruntime.ClabernetesRuntimeName)
	}

	if o.Deploy.ImagePullSecret != "" &&
		!clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return fmt.Errorf("--image-pull-secret is only supported with the %q runtime",
			clablabruntime.ClabernetesRuntimeName)
	}

	if err := normalizeExposeTypeFlag(o); err != nil {
		return err
	}

	if o.Deploy.NoPersistence && !clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return fmt.Errorf("--no-persistence is only supported with the %q runtime",
			clablabruntime.ClabernetesRuntimeName)
	}

	if err := validateEmitCRsFlags(o); err != nil {
		return err
	}

	// Neither preview mode changes the lab, so neither should leave a topology backup behind.
	o.Global.BackupTopologyFile = !o.Deploy.DryRun && !o.Deploy.EmitCRs

	var err error

	log.Info("Containerlab started", "version", Version)

	clabcore.ExportRenderedTopology = o.Deploy.ExportRenderedTopology

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
		SetDryRun(o.Deploy.DryRun).
		SetGraph(o.Deploy.GenerateGraph).
		SetSkipPostDeploy(o.Deploy.SkipPostDeploy).
		SetSkipLabDirFileACLs(o.Deploy.SkipLabDirectoryFileACLs).
		SetRestoreAll(o.Deploy.RestoreAll).
		SetRestoreNodeSnapshots(o.Deploy.RestoreNodeSnapshots).
		SetNoTopologyCR(o.Deploy.NoTopologyCR).
		SetImagePullSecret(o.Deploy.ImagePullSecret).
		SetExposeType(o.Deploy.ExposeType).
		SetNoPersistence(o.Deploy.NoPersistence)

	// Emitting manifests never touches the cluster, so it returns before deploy probes the
	// lab state or arms destroy-on-cancel.
	if o.Deploy.EmitCRs {
		manifests, err := c.LabRuntimeManifests(cobraCmd.Context(), deploymentOptions)
		if err != nil {
			return err
		}

		return printLabRuntimeManifests(os.Stdout, manifests, o.Inspect.Format)
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

		o.Global.CleanOnCancel.Store(cleanOnCancel)
	}

	result, err := c.Deploy(cobraCmd.Context(), deploymentOptions)
	if err != nil {
		// a deployment interrupted by the user leaves behind whatever it managed to
		// create, so tear it down before surfacing the cancellation
		if o.Global.CleanOnCancel.Load() && cancellationRequested() {
			destroyCancelledDeploy(o)
		}

		return err
	}

	if o.Deploy.DryRun {
		return printDryRunResult(result, o)
	}

	// keep stdout machine-readable for non-table formats: the reconciliation summary
	// table is only printed when the inspect output is a table as well
	if result.Apply != nil && o.Inspect.Format == clabconstants.FormatTable {
		printApplyResult(result.Apply)
	}

	if shouldDisplayPostDeployVersion(o.Global.Runtime) {
		// The manager has fetched in the background during deploy, so allow at most three more
		// seconds to finish and print the available containerlab release.
		versionCheckContext, cancel := context.WithTimeout(
			cobraCmd.Context(),
			postDeployVersionCheckTimeout,
		)
		defer cancel()

		m := getVersionManager()
		m.DisplayNewVersionAvailable(versionCheckContext, false)
	}

	// print table summary
	return PrintContainerInspect(result.Containers, o)
}

func normalizeExposeTypeFlag(o *Options) error {
	if o.Deploy.ExposeType != "" && !clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return fmt.Errorf("--expose-type is only supported with the %q runtime",
			clablabruntime.ClabernetesRuntimeName)
	}

	exposeType, err := clablabruntime.NormalizeExposeType(o.Deploy.ExposeType)
	if err != nil {
		return fmt.Errorf("invalid --expose-type: %w", err)
	}
	o.Deploy.ExposeType = exposeType

	return nil
}

func shouldDisplayPostDeployVersion(runtimeName string) bool {
	return !clablabruntime.IsLabRuntimeName(runtimeName)
}

// validateEmitCRsFlags rejects --emit-crs outside the lab runtime that can render manifests and
// in combination with the modes that imply a real or simulated deployment.
func validateEmitCRsFlags(o *Options) error {
	if !o.Deploy.EmitCRs {
		return nil
	}

	if !clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return fmt.Errorf("--emit-crs is only supported with the %q runtime",
			clablabruntime.ClabernetesRuntimeName)
	}
	if o.Deploy.DryRun {
		return fmt.Errorf(
			"--emit-crs cannot be combined with --dry-run: " +
				"dry-run plans against the cluster while emit-crs never contacts it",
		)
	}
	if o.Deploy.Reconfigure {
		return fmt.Errorf(
			"--emit-crs cannot be combined with --reconfigure: " +
				"reconfigure always destroys and redeploys the full lab",
		)
	}

	return nil
}

// printLabRuntimeManifests writes the emitted resources as a multi-document YAML stream, or as
// a v1 List for the json format, so the output can be redirected to a file and applied as is.
func printLabRuntimeManifests(
	w io.Writer,
	manifests []clablabruntime.Manifest,
	format string,
) error {
	if format == clabconstants.FormatJSON {
		items := make([]map[string]any, 0, len(manifests))
		for _, manifest := range manifests {
			items = append(items, manifest.Object)
		}

		b, err := json.MarshalIndent(map[string]any{
			"apiVersion": "v1",
			"kind":       "List",
			"items":      items,
		}, "", "  ")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(w, string(b))

		return err
	}

	for _, manifest := range manifests {
		b, err := yaml.Marshal(manifest.Object)
		if err != nil {
			return fmt.Errorf("failed to render %s %s: %w", manifest.Kind, manifest.Name, err)
		}

		if _, err = fmt.Fprintf(w, "---\n%s", b); err != nil {
			return err
		}
	}

	return nil
}

// printDryRunResult prints the planned changes of a dry run, as JSON when requested via
// the --format flag and as a table otherwise.
func printDryRunResult(result *clabcore.DeployResult, o *Options) error {
	if o.Inspect.Format == clabconstants.FormatJSON {
		value := any(result.Apply)
		if result.RuntimePlan != nil {
			value = result.RuntimePlan
		}

		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))

		return nil
	}

	if result.RuntimePlan != nil {
		printLabRuntimePlan(result.RuntimePlan)

		return nil
	}

	printApplyResult(result.Apply)

	return nil
}

func printLabRuntimePlan(plan *clablabruntime.DeployPlan) {
	log.Info("Lab runtime plan", "name", plan.LabName, "namespace", plan.Namespace)

	table := tableWriter.NewWriter()
	table.SetOutputMirror(os.Stdout)
	table.SetStyle(tableWriter.StyleRounded)
	table.Style().Format.Header = text.FormatTitle
	table.Style().Format.HeaderAlign = text.AlignCenter
	table.AppendHeader(tableWriter.Row{"Action", "Kind", "Resource"})

	for _, change := range plan.Changes {
		resourceName := change.Name
		if change.Namespace != "" {
			resourceName = change.Namespace + "/" + resourceName
		}
		table.AppendRow(tableWriter.Row{change.Action, change.Kind, resourceName})
	}
	if len(plan.Changes) == 0 {
		table.AppendRow(tableWriter.Row{"no changes", "-", "-"})
	}

	table.Render()
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
		{
			label:  "recreated nodes",
			values: withNodeChangeReasons(result.RecreatedNodes, result.NodeChangeReasons),
		},
		{label: "started nodes", values: result.StartedNodes},
		{label: "added links", values: result.AddedLinks},
		{label: "deleted endpoints", values: result.DeletedEndpoints},
		{
			label:  "restarted nodes",
			values: withNodeChangeReasons(result.RestartedNodes, result.NodeChangeReasons),
		},
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
