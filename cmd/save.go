package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

var saveCommand = map[string][]string{
	"srl":  {"sr_cli", "-d", "tools", "system", "configuration", "generate-checkpoint"},
	"ceos": {"Cli", "-p", "15", "-c", "copy running flash:conf-saved.conf"},
	"crpd": {"cli", "show", "conf"},
}

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "save containers configuration",
	Long: `save performs a configuration save. The exact command that is used to save the config depends on the node kind.
Refer to the https://containerlab.srlinux.dev/cmd/save/ documentation to see the exact command used per node's kind`,
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && topo == "" {
			return fmt.Errorf("provide topology file path  with --topo flag")
		}
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithTopoFile(topo),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		containers, err := c.ListContainers(ctx, []string{"containerlab=" + c.Config.Name})
		if err != nil {
			return fmt.Errorf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			return fmt.Errorf("no containers found")
		}

		var wg sync.WaitGroup
		wg.Add(len(containers))
		for _, cont := range containers {
			go func(cont types.Container) {
				defer wg.Done()
				kind := cont.Labels["kind"]
				// skip saving process for linux containers
				if kind == "linux" {
					return
				}
				stdout, stderr, err := c.Exec(ctx, cont.ID, saveCommand[kind])
				if err != nil {
					log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)

				}
				if len(stderr) > 0 {
					log.Infof("%s errors: %s", strings.TrimLeft(cont.Names[0], "/"), string(stderr))
				}
				switch {
				// for srl kinds print the full stdout
				case kind == "srl":
					if len(stdout) > 0 {
						confPath := cont.Labels["clab-node-dir"] + "/config/checkpoint/checkpoint-0.json"
						log.Infof("saved SR Linux configuration from %s node to %s\noutput:\n%s", strings.TrimLeft(cont.Names[0], "/"), confPath, string(stdout))
					}

				case kind == "crpd":
					// path by which to save a config
					confPath := cont.Labels["clab-node-dir"] + "/config/conf-saved.conf"
					err := ioutil.WriteFile(confPath, stdout, 0777)
					if err != nil {
						log.Errorf("failed to write config by %s path from %s container: %v", confPath, strings.TrimLeft(cont.Names[0], "/"), err)
					}
					log.Infof("saved cRPD configuration from %s node to %s", strings.TrimLeft(cont.Names[0], "/"), confPath)

				case kind == "ceos":
					// path by which a config was saved
					confPath := cont.Labels["clab-node-dir"] + "/flash/conf-saved.conf"
					log.Infof("saved cEOS configuration from %s node to %s", strings.TrimLeft(cont.Names[0], "/"), confPath)
				}
			}(cont)
		}
		wg.Wait()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
