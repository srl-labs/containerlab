package cmd

import (
	"github.com/spf13/cobra"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func redeployCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   "redeploy",
		Short: "destroy and redeploy a lab",
		Long: "destroy a lab and deploy it again based on the topology definition file\n" +
			"reference: https://containerlab.dev/cmd/redeploy/",
		Aliases: []string{"rdep"},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return redeployFn(cobraCmd, o)
		},
	}

	// Add destroy flags
	c.Flags().BoolVarP(
		&o.Destroy.Cleanup,
		"cleanup",
		"c",
		o.Destroy.Cleanup,
		"delete lab directory",
	)
	c.Flags().BoolVarP(
		&o.Global.GracefulShutdown,
		"graceful",
		"",
		o.Global.GracefulShutdown,
		"attempt to stop containers before removing",
	)
	c.Flags().BoolVarP(
		&o.Destroy.All,
		"all",
		"a",
		o.Destroy.All,
		"destroy all containerlab labs",
	)
	c.Flags().UintVarP(
		&o.Deploy.MaxWorkers,
		"max-workers",
		"",
		o.Deploy.MaxWorkers,
		"limit the maximum number of workers creating/deleting nodes",
	)
	c.Flags().BoolVarP(
		&o.Destroy.KeepManagementNetwork,
		"keep-mgmt-net",
		"",
		o.Destroy.KeepManagementNetwork,
		"do not remove the management network",
	)

	// Add deploy flags
	c.Flags().BoolVarP(
		&o.Deploy.GenerateGraph,
		"graph",
		"g",
		o.Deploy.GenerateGraph,
		"generate topology graph",
	)
	c.Flags().StringVarP(
		&o.Deploy.ManagementNetworkName,
		"network",
		"",
		o.Deploy.ManagementNetworkName,
		"management network name",
	)
	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv4Subnet,
		"ipv4-subnet",
		"4",
		o.Deploy.ManagementIPv4Subnet,
		"management network IPv4 subnet range",
	)
	c.Flags().IPNetVarP(
		&o.Deploy.ManagementIPv6Subnet,
		"ipv6-subnet",
		"6",
		o.Deploy.ManagementIPv6Subnet,
		"management network IPv6 subnet range",
	)
	c.Flags().StringVarP(
		&o.Inspect.Format,
		"format",
		"f",
		o.Inspect.Format, "output format. One of [table, json]",
	)
	c.Flags().BoolVarP(
		&o.Deploy.SkipPostDeploy,
		"skip-post-deploy",
		"",
		o.Deploy.SkipPostDeploy,
		"skip post deploy action",
	)
	c.Flags().StringVarP(
		&o.Deploy.ExportTemplate,
		"export-template",
		"",
		o.Deploy.ExportTemplate,
		"template file for topology data export",
	)
	c.Flags().BoolVarP(
		&o.Deploy.SkipLabDirectoryFileACLs,
		"skip-labdir-acl",
		"",
		o.Deploy.SkipLabDirectoryFileACLs,
		"skip the lab directory extended ACLs provisioning",
	)

	return c, nil
}

func redeployFn(cobraCmd *cobra.Command, o *Options) error {
	// First destroy the lab
	err := destroyFn(cobraCmd, o)
	if err != nil {
		return err
	}

	// Then deploy it again
	return deployFn(cobraCmd, o)
}
