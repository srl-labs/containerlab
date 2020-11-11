package cmd

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

var labels []string

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "execute a command on one or multiple containers",

	Run: func(cmd *cobra.Command, args []string) {
		if prefix == "" && topo == "" {
			fmt.Println("provide either lab prefix (--prefix) or topology file path (--topo)")
			return
		}
		log.Debugf("raw command: %v", args)
		if len(args) == 0 {
			fmt.Println("provide command to execute")
			return
		}
		c := clab.NewContainerLab(debug)
		err := c.Init(timeout)
		if err != nil {
			log.Fatal(err)
		}
		if prefix == "" {
			if err = c.GetTopology(topo); err != nil {
				log.Fatal(err)
			}
			prefix = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		labels = append(labels, "containerlab=lab-"+prefix)
		containers, err := c.ListContainers(ctx, labels)
		if err != nil {
			log.Fatalf("could not list containers: %v", err)
		}
		if len(containers) == 0 {
			log.Println("no containers found")
			return
		}
		cmds := make([]string, 0, len(args))
		for _, a := range args {
			cmds = append(cmds, strings.Split(a, " ")...)
		}
		for _, cont := range containers {
			stdout, stderr, err := c.Exec(ctx, cont.ID, cmds)
			if err != nil {
				log.Errorf("%s: failed to execute cmd: %v", cont.Names, err)
				continue
			}
			if len(stdout) > 0 {
				log.Infof("%s: stdout:\n%s", cont.Names, string(stdout))
			}
			if len(stderr) > 0 {
				log.Infof("%s: stderr:\n%s", cont.Names, string(stderr))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringSliceVarP(&labels, "label", "", []string{}, "labels to filter container subset")
}
