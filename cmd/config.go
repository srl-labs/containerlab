package cmd

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/clab/config"
	"golang.org/x/crypto/ssh"
)

// path to additional templates
var templatePath string

// Only print config locally, dont send to the node
var printLines int

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

		config.DebugCount = debugCount

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
		log.Debugf("Topology definition: %+v", c.Config)
		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			return err
		}

		// config map per node. each node gets a couple of config snippets []string
		allConfig := make(map[string][]config.ConfigSnippet)

		renderErr := 0

		for _, node := range c.Nodes {
			err = config.LoadTemplate(node.Kind, templatePath)
			if err != nil {
				return err
			}

			res, err := config.RenderNode(node)
			if err != nil {
				log.Errorln(err)
				renderErr += 1
				continue
			}
			allConfig[node.LongName] = append(allConfig[node.LongName], res...)

		}

		for lIdx, link := range c.Links {

			res, err := config.RenderLink(link)
			if err != nil {
				log.Errorf("%d. %s\n", lIdx, err)
				renderErr += 1
				continue
			}
			for _, rr := range res {
				allConfig[rr.TargetNode.LongName] = append(allConfig[rr.TargetNode.LongName], rr)
			}

		}

		if renderErr > 0 {
			return fmt.Errorf("%d render warnings", renderErr)
		}

		if printLines > 0 {
			// Debug log all config to be deployed
			for _, v := range allConfig {
				for _, r := range v {
					r.Print(printLines)
				}
			}
			return nil
		}

		var wg sync.WaitGroup
		wg.Add(len(allConfig))
		for _, cs_ := range allConfig {
			deploy1 := func(cs []config.ConfigSnippet) {
				defer wg.Done()

				var transport config.Transport

				ct, ok := cs[0].TargetNode.Labels["config.transport"]
				if !ok {
					ct = "ssh"
				}

				if ct == "ssh" {
					transport, _ = newSSHTransport(cs[0].TargetNode)
					if err != nil {
						log.Errorf("%s: %s", kind, err)
					}
				} else if ct == "grpc" {
					// newGRPCTransport
				} else {
					log.Errorf("Unknown transport: %s", ct)
					return
				}

				err := config.WriteConfig(transport, cs)
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

func newSSHTransport(node *clab.Node) (*config.SshTransport, error) {
	switch node.Kind {
	case "vr-sros", "srl":
		c := &config.SshTransport{}
		c.SshConfig = &ssh.ClientConfig{}
		config.SshConfigWithUserNamePassword(
			c.SshConfig,
			clab.DefaultCredentials[node.Kind][0],
			clab.DefaultCredentials[node.Kind][1])

		switch node.Kind {
		case "vr-sros":
			c.K = &config.VrSrosSshKind{}
		case "srl":
			c.K = &config.SrlSshKind{}
		}
		return c, nil
	}
	return nil, fmt.Errorf("no tranport implemented for kind: %s", kind)
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVarP(&templatePath, "template-path", "p", "", "directory with templates used to render config")
	configCmd.MarkFlagDirname("template-path")
	configCmd.Flags().StringSliceVarP(&config.TemplateOverride, "template-list", "l", []string{}, "comma separated list of template names to render")
	configCmd.Flags().IntVarP(&printLines, "check", "c", 0, "render dry-run & print n lines of config")
}
