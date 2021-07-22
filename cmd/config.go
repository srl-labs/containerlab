package cmd

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/config"
	"github.com/srl-labs/containerlab/clab/config/transport"
	"github.com/srl-labs/containerlab/nodes"
)

// Dryrun and print config
var check string

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:          "config",
	Short:        "configure a lab",
	Long:         "configure a lab based on templates and variables from the topology definition file\nreference: https://containerlab.srlinux.dev/cmd/config/",
	Aliases:      []string{"conf"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		transport.DebugCount = debugCount
		config.DebugCount = debugCount

		c, err := clab.NewContainerLab(
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
		)
		if err != nil {
			return err
		}

		// Config map per node. Each node gets a config.NodeConfig
		allConfig, err := config.RenderAll(c.Nodes, c.Links)
		if err != nil {
			return err
		}

		if check != "" {

			pv := check == "all" || check == "vars"
			pt := check == "all" || check == "template"

			if !(pv || pt) {
				if c, ok := allConfig[check]; ok {
					c.Print(true, true)
					return nil
				}
				log.Warnf("Invalid command line option for check. Options: 'template'(default), 'vars', 'all' or a valid node name")
				pt = true
			}
			for _, c := range allConfig {
				c.Print(pv, pt)
			}
			return nil
		}

		var wg sync.WaitGroup
		wg.Add(len(allConfig))
		for _, cs_ := range allConfig {
			deploy1 := func(cs *config.NodeConfig) {
				defer wg.Done()

				var tx transport.Transport
				var err error

				ct, ok := cs.TargetNode.Labels["config.transport"]
				if !ok {
					ct = "ssh"
				}

				if ct == "ssh" {
					tx, err = transport.NewSSHTransport(
						cs.TargetNode,
						transport.WithUserNamePassword(
							nodes.DefaultCredentials[cs.TargetNode.Kind][0],
							nodes.DefaultCredentials[cs.TargetNode.Kind][1]),
						transport.HostKeyCallback(),
					)
					if err != nil {
						log.Errorf("%s: %s", kind, err)
					}
				} else if ct == "grpc" {
					// NewGRPCTransport
				} else {
					log.Errorf("Unknown transport: %s", ct)
					return
				}

				err = transport.Write(tx, cs.TargetNode.LongName, cs.Data, cs.Info)
				if err != nil {
					log.Errorf("%s\n", err)
				}
			}

			// On debug this will not be executed concurrently
			if log.IsLevelEnabled(log.DebugLevel) {
				deploy1(cs_)
			} else {
				go deploy1(cs_)
			}
		}
		wg.Wait()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringSliceVarP(&config.TemplatePaths, "template-path", "p", []string{}, "comma separated list of paths to search for templates")
	_ = configCmd.MarkFlagDirname("template-path")
	configCmd.Flags().StringSliceVarP(&config.TemplateNames, "template-list", "l", []string{}, "comma separated list of template names to render")
	configCmd.Flags().StringVarP(&check, "check", "c", "", "render templates in dry-run mode & print either 'template', 'vars', 'all' or a specific node")
	configCmd.Flags().Lookup("check").NoOptDefVal = "template"
}
