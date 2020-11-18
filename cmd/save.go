package cmd

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.srlinux.dev/cmd/save/ documentation to see the exact command used per node's kind`,
	Run: func(cmd *cobra.Command, args []string) {
		if name == "" && topo == "" {
			fmt.Println("provide either lab name (--name) or topology file path (--topo)")
			return
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)
		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		containers, err := c.ListContainers(ctx, []string{"containerlab=lab-" + name, "kind=srl"})
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		var saveCmd []string
		for _, cont := range containers {
			if cont.State != "running" {
				continue
			}
			log.Debugf("container: %+v", cont)
			if k, ok := cont.Labels["kind"]; ok {
				switch k {
				case "srl":
					saveCmd = []string{"sr_cli", "-d", "tools", "system", "configuration", "generate-checkpoint"}
				case "ceos":
					//TODO
				default:
					continue
				}
			}
			stdout, stderr, err := c.Exec(ctx, cont.ID, saveCmd)
			if err != nil {
				log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)
				continue
			}
			if len(stdout) > 0 {
				log.Infof("%s output: %s", strings.TrimLeft(cont.Names[0], "/"), string(stdout))
			}
			if len(stderr) > 0 {
				log.Infof("%s errors: %s", strings.TrimLeft(cont.Names[0], "/"), string(stderr))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
