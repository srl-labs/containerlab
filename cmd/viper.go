// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envPrefix = "CLAB"
)

var v *viper.Viper //nolint:gochecknoglobals

// initViper initializes viper for environment variable and config file support.
func initViper(cmd *cobra.Command) error {
	v = viper.New()

	// Set the environment variable prefix
	v.SetEnvPrefix(envPrefix)

	// Replace hyphens and slashes with underscores in environment variable names
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", "/", "_"))

	// Automatically bind environment variables
	v.AutomaticEnv()

	// Bind all flags to viper
	return bindFlags(cmd, v)
}

// bindFlags binds all cobra flags to viper for a command and its subcommands.
func bindFlags(cmd *cobra.Command, v *viper.Viper) error {
	// Bind persistent flags
	if err := v.BindPFlags(cmd.PersistentFlags()); err != nil {
		return err
	}

	// Bind local flags
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// Recursively bind flags for all subcommands
	for _, subCmd := range cmd.Commands() {
		if err := bindFlags(subCmd, v); err != nil {
			return err
		}
	}

	return nil
}

// updateOptionsFromViper updates the Options struct from viper values
// when environment variables are set and flags are not explicitly provided.
func updateOptionsFromViper(cmd *cobra.Command, o *Options) {
	// Update Global options (persistent flags)
	updateIfNotChanged(cmd, "debug", func() { o.Global.DebugCount = v.GetInt("debug") })
	updateIfNotChanged(cmd, "topo", func() { o.Global.TopologyFile = v.GetString("topo") })
	updateIfNotChanged(cmd, "vars", func() { o.Global.VarsFile = v.GetString("vars") })
	updateIfNotChanged(cmd, "name", func() { o.Global.TopologyName = v.GetString("name") })
	updateIfNotChanged(cmd, "timeout", func() { o.Global.Timeout = v.GetDuration("timeout") })
	updateIfNotChanged(cmd, "runtime", func() { o.Global.Runtime = v.GetString("runtime") })
	updateIfNotChanged(cmd, "log-level", func() { o.Global.LogLevel = v.GetString("log-level") })

	// Update Deploy options (local flags for deploy command)
	updateIfNotChanged(cmd, "graph", func() { o.Deploy.GenerateGraph = v.GetBool("graph") })
	updateIfNotChanged(cmd, "network", func() { o.Deploy.ManagementNetworkName = v.GetString("network") })
	updateIfNotChanged(cmd, "reconfigure", func() { o.Deploy.Reconfigure = v.GetBool("reconfigure") })
	updateIfNotChanged(cmd, "max-workers", func() { o.Deploy.MaxWorkers = v.GetUint("max-workers") })
	updateIfNotChanged(cmd, "skip-post-deploy", func() { o.Deploy.SkipPostDeploy = v.GetBool("skip-post-deploy") })
	updateIfNotChanged(cmd, "skip-labdir-acl", func() { o.Deploy.SkipLabDirectoryFileACLs = v.GetBool("skip-labdir-acl") })
	updateIfNotChanged(cmd, "export-template", func() { o.Deploy.ExportTemplate = v.GetString("export-template") })
	updateIfNotChanged(cmd, "owner", func() { o.Deploy.LabOwner = v.GetString("owner") })

	// Update Filter options
	updateIfNotChanged(cmd, "node-filter", func() { o.Filter.NodeFilter = v.GetStringSlice("node-filter") })

	// Update Destroy options
	updateIfNotChanged(cmd, "cleanup", func() { o.Destroy.Cleanup = v.GetBool("cleanup") })
	updateIfNotChanged(cmd, "all", func() { o.Destroy.All = v.GetBool("all") })
	updateIfNotChanged(cmd, "keep-mgmt-net", func() { o.Destroy.KeepManagementNetwork = v.GetBool("keep-mgmt-net") })

	// Update Inspect options
	updateIfNotChanged(cmd, "format", func() { o.Inspect.Format = v.GetString("format") })
	updateIfNotChanged(cmd, "details", func() { o.Inspect.Details = v.GetBool("details") })
	updateIfNotChanged(cmd, "wide", func() { o.Inspect.Wide = v.GetBool("wide") })

	// Update Graph options
	updateIfNotChanged(cmd, "server", func() { o.Graph.Server = v.GetString("server") })
	updateIfNotChanged(cmd, "template", func() { o.Graph.Template = v.GetString("template") })
	updateIfNotChanged(cmd, "offline", func() { o.Graph.Offline = v.GetBool("offline") })
	updateIfNotChanged(cmd, "dot", func() { o.Graph.GenerateDotFile = v.GetBool("dot") })
	updateIfNotChanged(cmd, "mermaid", func() { o.Graph.GenerateMermaid = v.GetBool("mermaid") })
	updateIfNotChanged(cmd, "drawio", func() { o.Graph.GenerateDrawIO = v.GetBool("drawio") })
}

// updateIfNotChanged updates the option value using the provided function
// if the flag was not explicitly set on the command line and viper has a value for it.
func updateIfNotChanged(cmd *cobra.Command, flagName string, updateFn func()) {
	flag := cmd.Flag(flagName)
	if flag == nil {
		// Try persistent flags if local flag not found
		flag = cmd.PersistentFlags().Lookup(flagName)
	}
	if flag != nil && !flag.Changed && v.IsSet(flagName) {
		updateFn()
	}
}
