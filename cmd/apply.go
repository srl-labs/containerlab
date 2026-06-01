package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func applyCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "apply",
		Short: "Apply a topology file to a lab",
		Long: "Apply a topology definition file by deploying the lab when it does not exist, " +
			"or adding/deleting supported nodes and links without fully redeploying a running lab.",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return applyFn(cmd, o)
		},
	}

	c.Flags().BoolVar(
		&o.Apply.DryRun,
		"dry-run",
		o.Apply.DryRun,
		"show apply actions without applying them",
	)
	c.Flags().UintVarP(
		&o.Apply.MaxWorkers,
		"max-workers",
		"",
		o.Apply.MaxWorkers,
		"limit the maximum number of workers creating new nodes",
	)
	c.Flags().BoolVarP(
		&o.Apply.SkipPostDeploy,
		"skip-post-deploy",
		"",
		o.Apply.SkipPostDeploy,
		"skip post deploy action for added nodes",
	)
	c.Flags().StringVarP(
		&o.Apply.ExportTemplate,
		"export-template",
		"",
		o.Apply.ExportTemplate,
		"template file for topology data export",
	)

	return c, nil
}

func applyFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}

	log.Info("Containerlab started", "version", Version)

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	applyOptions, err := o.ToClabApplyOptions()
	if err != nil {
		return err
	}

	result, err := c.Apply(cmd.Context(), applyOptions)
	if err != nil {
		return err
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
		{label: "added links", values: result.AddedLinks},
		{label: "deleted endpoints", values: result.DeletedEndpoints},
		{label: "restarted nodes", values: result.RestartedNodes},
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

func appendApplyResultRows(table tableWriter.Writer, label string, values []string) bool {
	if len(values) == 0 {
		return false
	}

	for _, value := range values {
		table.AppendRow(tableWriter.Row{label, value})
	}

	return true
}
