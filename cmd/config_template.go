package cmd

import (
	"github.com/spf13/cobra"
	containerlabcore "github.com/srl-labs/containerlab/core"
	containerlabcoreconfig "github.com/srl-labs/containerlab/core/config"
)

// Show the template variable s.
var templateVarOnly bool

// configCmd represents the config command.
var configTemplateCmd = &cobra.Command{
	Use:          "template",
	Short:        "render a template",
	Long:         "render a template based on variables from the topology definition file\nreference: https://containerlab.dev/cmd/config/template",
	Aliases:      []string{"conf"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		containerlabcoreconfig.DebugCount = debugCount

		c, err := containerlabcore.NewContainerLab(
			containerlabcore.WithTimeout(timeout),
			containerlabcore.WithTopoPath(topoFile, varsFile),
			containerlabcore.WithDebug(debug),
		)
		if err != nil {
			return err
		}

		err = validateFilter(c.Nodes)
		if err != nil {
			return err
		}

		allConfig := containerlabcoreconfig.PrepareVars(c)
		if templateVarOnly {
			for _, n := range configFilter {
				conf := allConfig[n]
				conf.Print(true, false)
			}
			return nil
		}

		err = containerlabcoreconfig.RenderAll(allConfig)
		if err != nil {
			return err
		}

		for _, n := range configFilter {
			allConfig[n].Print(false, true)
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configTemplateCmd)
	configTemplateCmd.Flags().AddFlagSet(configCmd.Flags())
	configTemplateCmd.Flags().BoolVarP(&templateVarOnly, "vars", "v", false,
		"show variable used for template rendering")
	configTemplateCmd.Flags().SortFlags = false
}
