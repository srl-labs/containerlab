package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/config"
	"github.com/srl-labs/containerlab/clab/config/transport"
	"github.com/srl-labs/containerlab/nodes"

	log "github.com/sirupsen/logrus"
)

// Node Filter for config.
var configFilter []string

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:          "config",
	Short:        "configure a lab",
	Long:         "configure a lab based on templates and variables from the topology definition file\nreference: https://containerlab.dev/cmd/config/",
	Aliases:      []string{"conf"},
	ValidArgs:    []string{"commit", "send", "compare", "template"},
	SilenceUsage: true,
	RunE:         configRun,
}

var configSendCmd = &cobra.Command{
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

var configCompareCmd = &cobra.Command{
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

func configRun(_ *cobra.Command, args []string) error {
	var err error

	transport.DebugCount = debugCount
	config.DebugCount = debugCount

	c, err := clab.NewContainerLab(
		clab.WithTimeout(timeout),
		clab.WithTopoPath(topo, varsFile),
		clab.WithNodeFilter(nodeFilter),
		clab.WithDebug(debug),
	)
	if err != nil {
		return err
	}

	err = validateFilter(c.Nodes)
	if err != nil {
		return err
	}

	allConfig := config.PrepareVars(c)

	err = config.RenderAll(allConfig)
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

		err = config.Send(cs, action)
		if err != nil {
			log.Warnf("%s: %s", cs.TargetNode.ShortName, err)
		}
	}
	wg.Add(len(configFilter))
	for _, node := range configFilter {
		// On debug this will not be executed concurrently
		if log.IsLevelEnabled(log.DebugLevel) {
			deploy(node)
		} else {
			go deploy(node)
		}
	}
	wg.Wait()

	return nil
}

func validateFilter(nodes map[string]nodes.Node) error {
	if len(configFilter) == 0 {
		for n := range nodes {
			configFilter = append(configFilter, n)
		}
		return nil
	}
	mis := []string{}
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

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringSliceVarP(&config.TemplatePaths, "template-path", "p", []string{},
		"comma separated list of paths to search for templates")
	_ = configCmd.MarkFlagDirname("template-path")
	configCmd.Flags().StringSliceVarP(&config.TemplateNames, "template-list", "l", []string{},
		"comma separated list of template names to render")
	configCmd.Flags().StringSliceVarP(&configFilter, "filter", "f", []string{},
		"comma separated list of nodes to include")
	configCmd.Flags().SortFlags = false

	configCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")

	configCmd.AddCommand(configSendCmd)
	configSendCmd.Flags().AddFlagSet(configCmd.Flags())

	configCmd.AddCommand(configCompareCmd)
	configCompareCmd.Flags().AddFlagSet(configCmd.Flags())
}
