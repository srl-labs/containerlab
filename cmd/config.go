package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabcoreconfig "github.com/srl-labs/containerlab/core/config"
	"github.com/srl-labs/containerlab/core/config/transport"
	clabnodes "github.com/srl-labs/containerlab/nodes"

	"github.com/charmbracelet/log"
)

func configCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "config",
		Short:        "configure a lab",
		Long:         "configure a lab based on templates and variables from the topology definition file\nreference: https://containerlab.dev/cmd/config/",
		Aliases:      []string{"conf"},
		ValidArgs:    []string{"commit", "send", "compare", "template"},
		SilenceUsage: true,
		RunE:         configRun,
	}

	c.Flags().StringSliceVarP(&clabcoreconfig.TemplatePaths, "template-path", "p", []string{},
		"comma separated list of paths to search for templates")
	_ = c.MarkFlagDirname("template-path")
	c.Flags().StringSliceVarP(&clabcoreconfig.TemplateNames, "template-list", "l", []string{},
		"comma separated list of template names to render")
	c.Flags().StringSliceVarP(&configFilter, "filter", "f", []string{},
		"comma separated list of nodes to include")
	c.Flags().SortFlags = false

	c.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")

	sendC := &cobra.Command{
		Use:          "send",
		Short:        "send raw configuration to a lab",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", args)
			}
			return configRun(cmd, []string{"send"})
		},
	}

	c.AddCommand(sendC)
	sendC.Flags().AddFlagSet(c.Flags())

	compareC := &cobra.Command{
		Use:          "compare",
		Short:        "compare configuration to a running lab",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", args)
			}
			return configRun(cmd, []string{"compare"})
		},
	}

	c.AddCommand(compareC)
	compareC.Flags().AddFlagSet(c.Flags())

	templateC := &cobra.Command{
		Use:          "template",
		Short:        "render a template",
		Long:         "render a template based on variables from the topology definition file\nreference: https://containerlab.dev/cmd/config/template",
		Aliases:      []string{"conf"},
		SilenceUsage: true,
		RunE:         configTemplate,
	}

	c.AddCommand(templateC)
	templateC.Flags().AddFlagSet(c.Flags())
	templateC.Flags().BoolVarP(&templateVarOnly, "vars", "v", false,
		"show variable used for template rendering")
	templateC.Flags().SortFlags = false

	return c
}

func configRun(_ *cobra.Command, args []string) error {
	var err error

	transport.DebugCount = debugCount
	clabcoreconfig.DebugCount = debugCount

	c, err := clabcore.NewContainerLab(
		clabcore.WithTimeout(timeout),
		clabcore.WithTopoPath(topoFile, varsFile),
		clabcore.WithNodeFilter(nodeFilter),
		clabcore.WithDebug(debug),
	)
	if err != nil {
		return err
	}

	err = validateFilter(c.Nodes)
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

		err = clabcoreconfig.Send(cs, action)
		if err != nil {
			log.Warnf("%s: %s", cs.TargetNode.ShortName, err)
		}
	}
	wg.Add(len(configFilter))
	for _, node := range configFilter {
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

func configTemplate(cmd *cobra.Command, args []string) error {
	var err error

	clabcoreconfig.DebugCount = debugCount

	c, err := clabcore.NewContainerLab(
		clabcore.WithTimeout(timeout),
		clabcore.WithTopoPath(topoFile, varsFile),
		clabcore.WithDebug(debug),
	)
	if err != nil {
		return err
	}

	err = validateFilter(c.Nodes)
	if err != nil {
		return err
	}

	allConfig := clabcoreconfig.PrepareVars(c)
	if templateVarOnly {
		for _, n := range configFilter {
			conf := allConfig[n]
			conf.Print(true, false)
		}
		return nil
	}

	err = clabcoreconfig.RenderAll(allConfig)
	if err != nil {
		return err
	}

	for _, n := range configFilter {
		allConfig[n].Print(false, true)
	}

	return nil
}

func validateFilter(nodes map[string]clabnodes.Node) error {
	if len(configFilter) == 0 {
		for n := range nodes {
			configFilter = append(configFilter, n)
		}
		return nil
	}

	var mis []string
	for _, nn := range configFilter {
		if _, ok := nodes[nn]; !ok {
			mis = append(mis, nn)
		}
	}
	if len(mis) > 0 {
		return fmt.Errorf("invalid nodes in filter: %s", strings.Join(mis, ", "))
	}
	return nil
}
