package cmd

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/config"
)

// path to additional templates
var templatePath string

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:          "config",
	Short:        "configure a lab",
	Long:         "configure a lab based using templates and variables from the topology definition file\nreference: https://containerlab.srlinux.dev/cmd/config/",
	Aliases:      []string{"conf"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		if err = topoSet(); err != nil {
			return err
		}

		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)

		//ctx, cancel := context.WithCancel(context.Background())
		//defer cancel()

		setFlags(c.Config)
		log.Debugf("lab Conf: %+v", c.Config)
		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			return err
		}

		// config map per node. each node gets a couple of config snippets []string
		allConfig := make(map[string][]*config.ConfigSnippet)

		renderErr := 0

		for _, node := range c.Nodes {
			kind := node.Labels["clab-node-kind"]
			err = config.LoadTemplate(kind, templatePath)
			if err != nil {
				return err
			}

			res, err := config.RenderNode(node)
			if err != nil {
				log.Errorln(err)
				renderErr += 1
				continue
			}
			allConfig[node.LongName] = append(allConfig[node.LongName], res)
		}

		for lIdx, link := range c.Links {

			resA, resB, err := config.RenderLink(link)
			if err != nil {
				log.Errorf("%d. %s\n", lIdx, err)
				renderErr += 1
				continue
			}
			allConfig[link.A.Node.LongName] = append(allConfig[link.A.Node.LongName], resA)
			allConfig[link.B.Node.LongName] = append(allConfig[link.B.Node.LongName], resB)

		}

		if renderErr > 0 {
			return fmt.Errorf("%d render warnings", renderErr)
		}

		// Debug log all config to be deployed
		for _, v := range allConfig {
			for _, r := range v {
				log.Infof("%s\n%s", r, r.Config)

			}
		}

		var wg sync.WaitGroup
		wg.Add(len(allConfig))
		for _, cs := range allConfig {
			go func(configSnippets []*config.ConfigSnippet) {
				defer wg.Done()

				err := config.SendConfig(configSnippets)
				if err != nil {
					log.Errorf("%s\n", err)
				}

			}(cs)
		}
		wg.Wait()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVarP(&templatePath, "templates", "", "", "specify template path")
	configCmd.MarkFlagDirname("templates")
}
