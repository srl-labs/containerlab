package cmd

import (
	"net"

	"github.com/spf13/cobra"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func redeployCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "redeploy",
		Short:        "destroy and redeploy a lab",
		Long:         "destroy a lab and deploy it again based on the topology definition file\nreference: https://containerlab.dev/cmd/redeploy/",
		Aliases:      []string{"rdep"},
		PreRunE:      clabutils.CheckAndGetRootPrivs,
		SilenceUsage: true,
		RunE:         redeployFn,
	}

	// Add destroy flags
	c.Flags().BoolVarP(&cleanup, "cleanup", "c", false, "delete lab directory")
	c.Flags().BoolVarP(&gracefulShutdown, "graceful", "", false,
		"attempt to stop containers before removing")
	c.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	c.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers creating/deleting nodes")
	c.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")

	// Add deploy flags
	c.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	c.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	c.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	c.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	c.Flags().StringVarP(&deployFormat, "format", "f", "table", "output format. One of [table, json]")
	c.Flags().BoolVarP(&skipPostDeploy, "skip-post-deploy", "", false, "skip post deploy action")
	c.Flags().StringVarP(&exportTemplate, "export-template", "", "",
		"template file for topology data export")
	c.Flags().BoolVarP(&skipLabDirFileACLs, "skip-labdir-acl", "", false,
		"skip the lab directory extended ACLs provisioning")

	return c
}

func redeployFn(cobraCmd *cobra.Command, args []string) error {
	// First destroy the lab
	err := destroyFn(cobraCmd, args)
	if err != nil {
		return err
	}

	// Then deploy it again
	return deployFn(cobraCmd, args)
}
