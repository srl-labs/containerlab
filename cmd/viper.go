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

// flagBinding represents the mapping between a flag name and how to update the option value.
type flagBinding struct {
	flagName string
	updater  func(*Options)
}

// getFlagBindings returns all flag-to-option bindings in a centralized location.
func getFlagBindings() []flagBinding {
	return []flagBinding{
		// Global options (persistent flags)
		{"debug", func(o *Options) { o.Global.DebugCount = v.GetInt("debug") }},
		{"topo", func(o *Options) { o.Global.TopologyFile = v.GetString("topo") }},
		{"vars", func(o *Options) { o.Global.VarsFile = v.GetString("vars") }},
		{"name", func(o *Options) { o.Global.TopologyName = v.GetString("name") }},
		{"timeout", func(o *Options) { o.Global.Timeout = v.GetDuration("timeout") }},
		{"runtime", func(o *Options) { o.Global.Runtime = v.GetString("runtime") }},
		{"log-level", func(o *Options) { o.Global.LogLevel = v.GetString("log-level") }},

		// Deploy options (local flags for deploy command)
		{"graph", func(o *Options) { o.Deploy.GenerateGraph = v.GetBool("graph") }},
		{"network", func(o *Options) { o.Deploy.ManagementNetworkName = v.GetString("network") }},
		{"reconfigure", func(o *Options) { o.Deploy.Reconfigure = v.GetBool("reconfigure") }},
		{"max-workers", func(o *Options) { o.Deploy.MaxWorkers = v.GetUint("max-workers") }},
		{"skip-post-deploy", func(o *Options) { o.Deploy.SkipPostDeploy = v.GetBool("skip-post-deploy") }},
		{"skip-labdir-acl", func(o *Options) { o.Deploy.SkipLabDirectoryFileACLs = v.GetBool("skip-labdir-acl") }},
		{"export-template", func(o *Options) { o.Deploy.ExportTemplate = v.GetString("export-template") }},
		{"owner", func(o *Options) { o.Deploy.LabOwner = v.GetString("owner") }},

		// Filter options
		{"node-filter", func(o *Options) { o.Filter.NodeFilter = v.GetStringSlice("node-filter") }},

		// Destroy options
		{"cleanup", func(o *Options) { o.Destroy.Cleanup = v.GetBool("cleanup") }},
		{"all", func(o *Options) { o.Destroy.All = v.GetBool("all") }},
		{"keep-mgmt-net", func(o *Options) { o.Destroy.KeepManagementNetwork = v.GetBool("keep-mgmt-net") }},

		// Inspect options
		{"format", func(o *Options) { o.Inspect.Format = v.GetString("format") }},
		{"details", func(o *Options) { o.Inspect.Details = v.GetBool("details") }},
		{"wide", func(o *Options) { o.Inspect.Wide = v.GetBool("wide") }},

		// Graph options
		{"server", func(o *Options) { o.Graph.Server = v.GetString("server") }},
		{"template", func(o *Options) { o.Graph.Template = v.GetString("template") }},
		{"offline", func(o *Options) { o.Graph.Offline = v.GetBool("offline") }},
		{"dot", func(o *Options) { o.Graph.GenerateDotFile = v.GetBool("dot") }},
		{"mermaid", func(o *Options) { o.Graph.GenerateMermaid = v.GetBool("mermaid") }},
		{"drawio", func(o *Options) { o.Graph.GenerateDrawIO = v.GetBool("drawio") }},
	}
}

// updateOptionsFromViper updates the Options struct from viper values
// when environment variables are set and flags are not explicitly provided.
func updateOptionsFromViper(cmd *cobra.Command, o *Options) {
	for _, binding := range getFlagBindings() {
		updateIfNotChanged(cmd, binding.flagName, func() {
			binding.updater(o)
		})
	}
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
