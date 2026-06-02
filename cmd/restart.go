package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func restartCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "restart",
		Short: "Restart one or more nodes in a deployed lab (seamless dataplane)",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return restartFn(cmd, o)
		},
	}

	c.Flags().StringSliceVarP(
		&o.NodeLifecycle.Nodes,
		"node",
		"n",
		o.NodeLifecycle.Nodes,
		"node(s) to restart (repeatable or comma-separated). If omitted, restart all nodes",
	)

	return c, nil
}

func restartFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	return c.RestartNodes(cmd.Context(), o.NodeLifecycle.Nodes)
}
