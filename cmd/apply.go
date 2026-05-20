package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
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

	fmt.Fprintln(os.Stdout, title)
	if result.DeployedLab {
		label := "deployed lab"
		if result.DryRun {
			label = "deploy lab"
		}
		fmt.Fprintf(os.Stdout, "  %s: %s\n", label, result.LabName)
	}
	printApplyResultLine("added nodes", result.AddedNodes)
	printApplyResultLine("deleted nodes", result.DeletedNodes)
	printApplyResultLine("added links", result.AddedLinks)
	printApplyResultLine("deleted endpoints", result.DeletedEndpoints)
	printApplyResultLine("restarted nodes", result.RestartedNodes)

	if !result.DeployedLab &&
		len(result.AddedNodes) == 0 &&
		len(result.DeletedNodes) == 0 &&
		len(result.AddedLinks) == 0 &&
		len(result.DeletedEndpoints) == 0 &&
		len(result.RestartedNodes) == 0 {
		fmt.Fprintln(os.Stdout, "  no changes")
	}
}

func printApplyResultLine(label string, values []string) {
	if len(values) == 0 {
		return
	}

	fmt.Fprintf(os.Stdout, "  %s: %s\n", label, strings.Join(values, ", "))
}
