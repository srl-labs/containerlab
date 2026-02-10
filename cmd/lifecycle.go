package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func stopCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "stop",
		Short: "Stop one or more nodes in a deployed lab (seamless dataplane)",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return stopFn(cmd, o)
		},
	}

	c.Flags().StringSliceVarP(
		&o.NodeLifecycle.Nodes,
		"node",
		"n",
		o.NodeLifecycle.Nodes,
		"node(s) to stop (repeatable or comma-separated)",
	)

	return c, nil
}

func stopFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}
	if len(o.NodeLifecycle.Nodes) == 0 {
		return fmt.Errorf("provide at least one node name via --node/-n")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	return c.StopNodes(cmd.Context(), o.NodeLifecycle.Nodes)
}

func startCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "start",
		Short: "Start one or more nodes in a deployed lab (seamless dataplane)",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return startFn(cmd, o)
		},
	}

	c.Flags().StringSliceVarP(
		&o.NodeLifecycle.Nodes,
		"node",
		"n",
		o.NodeLifecycle.Nodes,
		"node(s) to start (repeatable or comma-separated)",
	)

	return c, nil
}

func startFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}
	if len(o.NodeLifecycle.Nodes) == 0 {
		return fmt.Errorf("provide at least one node name via --node/-n")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	return c.StartNodes(cmd.Context(), o.NodeLifecycle.Nodes)
}

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
		"node(s) to restart (repeatable or comma-separated)",
	)

	return c, nil
}

func restartFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}
	if len(o.NodeLifecycle.Nodes) == 0 {
		return fmt.Errorf("provide at least one node name via --node/-n")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	return c.RestartNodes(cmd.Context(), o.NodeLifecycle.Nodes)
}
