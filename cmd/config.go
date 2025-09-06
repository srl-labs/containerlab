package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabcoreconfig "github.com/srl-labs/containerlab/core/config"
	clabnodes "github.com/srl-labs/containerlab/nodes"

	"github.com/charmbracelet/log"
)

func configCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "config",
		Short: "configure a lab",
		Long: "configure a lab based on templates and variables from the topology definition " +
			"file\n reference: https://containerlab.dev/cmd/config/",
		Aliases:      []string{"conf"},
		ValidArgs:    []string{"commit", "send", "compare", "template"},
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return configRun(cobraCmd, args, o)
		},
	}

	c.Flags().StringSliceVarP(
		&clabcoreconfig.TemplatePaths,
		"template-path",
		"p",
		[]string{},
		"comma separated list of paths to search for templates",
	)

	c.Flags().StringSliceVarP(
		&clabcoreconfig.TemplateNames,
		"template-list",
		"l",
		[]string{},
		"comma separated list of template names to render",
	)

	c.Flags().StringSliceVarP(
		&o.Filter.LabelFilter,
		"filter",
		"f",
		o.Filter.LabelFilter,
		"comma separated list of nodes (by labels) to include",
	)

	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)

	c.Flags().SortFlags = false

	err := c.MarkFlagDirname("template-path")
	if err != nil {
		return nil, err
	}

	configSubCmds(c, o)

	return c, nil
}

func configSubCmds(c *cobra.Command, o *Options) {
	sendC := &cobra.Command{
		Use:          "send",
		Short:        "send raw configuration to a lab",
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", args)
			}

			return configRun(cobraCmd, []string{"send"}, o)
		},
	}

	c.AddCommand(sendC)
	sendC.Flags().AddFlagSet(c.Flags())

	compareC := &cobra.Command{
		Use:          "compare",
		Short:        "compare configuration to a running lab",
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", args)
			}

			return configRun(cobraCmd, []string{"compare"}, o)
		},
	}

	c.AddCommand(compareC)
	compareC.Flags().AddFlagSet(c.Flags())

	templateC := &cobra.Command{
		Use:   "template",
		Short: "render a template",
		Long: "render a template based on variables from the topology definition file\n" +
			"reference: https://containerlab.dev/cmd/config/template",
		Aliases:      []string{"conf"},
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return configTemplate(o)
		},
	}

	c.AddCommand(templateC)
	templateC.Flags().AddFlagSet(c.Flags())
	templateC.Flags().BoolVarP(
		&o.Config.TemplateVarOnly,
		"vars",
		"v",
		o.Config.TemplateVarOnly,
		"show variable used for template rendering",
	)
	templateC.Flags().SortFlags = false
}

func configRun(_ *cobra.Command, args []string, o *Options) error {
	var err error

	clabcoreconfig.DebugCount = o.Global.DebugCount

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	err = validateFilter(c.Nodes, o)
	if err != nil {
		return err
	}

	allConfig := clabcoreconfig.PrepareVars(c)

	err = clabcoreconfig.RenderAll(allConfig)
	if err != nil {
		return err
	}

	if len(args) > 1 {
		return fmt.Errorf("unexpected arguments: %s", args)
	}

	action := "commit"
	if len(args) > 0 {
		action = args[0]
		switch action {
		case "commit":

		case "compare", "send":
			return fmt.Errorf("%s not implemented yet", action)
		default:
			return fmt.Errorf("unexpected arguments: %s", args)
		}
	}

	var wg sync.WaitGroup

	deploy := func(n string) {
		defer wg.Done()

		cs, ok := allConfig[n]
		if !ok {
			log.Errorf("Invalid node in filter: %s", n)

			return
		}

		err = clabcoreconfig.Send(cs, action, o.Global.DebugCount > 0)
		if err != nil {
			log.Warnf("%s: %s", cs.TargetNode.ShortName, err)
		}
	}

	wg.Add(len(o.Filter.LabelFilter))

	for _, node := range o.Filter.LabelFilter {
		// On debug this will not be executed concurrently
		if log.GetLevel() == (log.DebugLevel) {
			deploy(node)
		} else {
			go deploy(node)
		}
	}

	wg.Wait()

	return nil
}

func configTemplate(o *Options) error {
	var err error

	clabcoreconfig.DebugCount = o.Global.DebugCount

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	err = validateFilter(c.Nodes, o)
	if err != nil {
		return err
	}

	allConfig := clabcoreconfig.PrepareVars(c)

	if o.Config.TemplateVarOnly {
		for _, n := range o.Filter.LabelFilter {
			conf := allConfig[n]
			conf.Print(true, false)
		}

		return nil
	}

	err = clabcoreconfig.RenderAll(allConfig)
	if err != nil {
		return err
	}

	for _, n := range o.Filter.LabelFilter {
		allConfig[n].Print(false, true)
	}

	return nil
}

func validateFilter(nodes map[string]clabnodes.Node, o *Options) error {
	if len(o.Filter.LabelFilter) == 0 {
		for n := range nodes {
			o.Filter.LabelFilter = append(o.Filter.LabelFilter, n)
		}

		return nil
	}

	var mis []string

	for _, nn := range o.Filter.LabelFilter {
		if _, ok := nodes[nn]; !ok {
			mis = append(mis, nn)
		}
	}

	if len(mis) > 0 {
		return fmt.Errorf("invalid nodes in filter: %s", strings.Join(mis, ", "))
	}

	return nil
}
