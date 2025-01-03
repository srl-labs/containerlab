package cmd

import (
	"net"

	"github.com/spf13/cobra"
)

// redeployCmd represents the redeploy command.
var redeployCmd = &cobra.Command{
	Use:          "redeploy",
	Short:        "destroy and redeploy a lab",
	Long:         "destroy a lab and deploy it again based on the topology definition file\nreference: https://containerlab.dev/cmd/redeploy/",
	Aliases:      []string{"rdep"},
	PreRunE:      sudoCheck,
	SilenceUsage: true,
	RunE:         redeployFn,
}

func init() {
	rootCmd.AddCommand(redeployCmd) // Add to rootCmd

	// Add destroy flags
	redeployCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", false, "delete lab directory")
	redeployCmd.Flags().BoolVarP(&graceful, "graceful", "", false,
		"attempt to stop containers before removing")
	redeployCmd.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	redeployCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers creating/deleting nodes")
	redeployCmd.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")
	redeployCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")

	// Add deploy flags
	redeployCmd.Flags().BoolVarP(&graph, "graph", "g", false, "generate topology graph")
	redeployCmd.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	redeployCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	redeployCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	redeployCmd.Flags().StringVarP(&deployFormat, "format", "f", "table", "output format. One of [table, json]")
	redeployCmd.Flags().BoolVarP(&skipPostDeploy, "skip-post-deploy", "", false, "skip post deploy action")
	redeployCmd.Flags().StringVarP(&exportTemplate, "export-template", "", "",
		"template file for topology data export")
	redeployCmd.Flags().BoolVarP(&skipLabDirFileACLs, "skip-labdir-acl", "", false,
		"skip the lab directory extended ACLs provisioning")
}

func redeployFn(cmd *cobra.Command, args []string) error {
	// First destroy the lab
	err := destroyFn(destroyCmd, args)
	if err != nil {
		return err
	}

	// Then deploy it again
	return deployFn(deployCmd, args)
}
