package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// graphCmd represents the graph command
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "generate a topology graph",

	Run: func(cmd *cobra.Command, args []string) {
		c := clab.NewContainerLab(debug)
		err := c.Init()
		if err != nil {
			log.Info(err)
		}

		if err = c.GetTopology(&topo); err != nil {
			log.Fatal(err)
		}

		// Parse topology information
		if err = c.ParseTopology(); err != nil {
			log.Fatal(err)
		}

		if err = c.GenerateGraph(topo); err != nil {
			log.Error(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
}
