package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func logsCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "logs",
		Short: "show the logs of a lab node",
		Long: "show the logs of a lab node's container, or of its device container with a lab " +
			"runtime\nreference: https://containerlab.dev/cmd/logs/",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if commandSkipsRoot(o.Global.Runtime) {
				return nil
			}

			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return logsFn(cmd, o)
		},
	}

	c.Flags().StringVarP(
		&o.Logs.Node,
		"node",
		"n",
		o.Logs.Node,
		"node to show the logs of",
	)
	c.Flags().Int64Var(
		&o.Logs.Tail,
		"tail",
		o.Logs.Tail,
		"number of lines to show from the end of the log; -1 shows the whole log",
	)
	c.Flags().BoolVarP(
		&o.Logs.Follow,
		"follow",
		"f",
		o.Logs.Follow,
		"keep streaming new log lines",
	)
	c.Flags().BoolVar(
		&o.Logs.Timestamps,
		"timestamps",
		o.Logs.Timestamps,
		"prefix every line with its timestamp",
	)

	c.Example = `# Show the whole log of node srl1
containerlab logs -t srl.clab.yml -n srl1

# Follow the last 100 lines with timestamps
containerlab logs --name srl -n srl1 --tail 100 --follow --timestamps`

	return c, nil
}

func logsFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}
	if o.Logs.Node == "" {
		return fmt.Errorf("provide the node to show the logs of (--node)")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	opts := clabcore.NodeLogOptions{
		NodeName:   o.Logs.Node,
		Follow:     o.Logs.Follow,
		Timestamps: o.Logs.Timestamps,
	}
	if o.Logs.Tail >= 0 {
		tail := o.Logs.Tail
		opts.Tail = &tail
	}

	return c.NodeLogs(cmd.Context(), opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}
