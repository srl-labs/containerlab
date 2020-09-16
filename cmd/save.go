package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if prefix == "" && topo == "" {
			fmt.Println("provide either lab prefix (--prefix) or topology file path (--topo)")
			return
		}
		c := clab.NewContainerLab(debug)
		err := c.Init()
		if err != nil {
			log.Fatal(err)
		}
		if prefix == "" {
			if err = c.GetTopology(&topo); err != nil {
				log.Fatal(err)
			}
			prefix = c.Conf.Prefix
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		containers, err := c.ListContainers(ctx, []string{"containerlab=lab-" + prefix, "kind=srl"})
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		var saveCmd []string
		for _, cont := range containers {
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
				log.Infof("%s: stdout: %s", cont.Names, string(stdout))
			}
			if len(stderr) > 0 {
				log.Infof("%s: stderr: %s", cont.Names, string(stderr))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
