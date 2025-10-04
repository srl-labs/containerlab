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
	// Update Global options
	if v.IsSet("debug") && !cmd.Flag("debug").Changed {
		o.Global.DebugCount = v.GetInt("debug")
	}
	if v.IsSet("topo") && !cmd.Flag("topo").Changed {
		o.Global.TopologyFile = v.GetString("topo")
	}
	if v.IsSet("vars") && !cmd.Flag("vars").Changed {
		o.Global.VarsFile = v.GetString("vars")
	}
	if v.IsSet("name") && !cmd.Flag("name").Changed {
		o.Global.TopologyName = v.GetString("name")
	}
	if v.IsSet("timeout") && !cmd.Flag("timeout").Changed {
		o.Global.Timeout = v.GetDuration("timeout")
	}
	if v.IsSet("runtime") && !cmd.Flag("runtime").Changed {
		o.Global.Runtime = v.GetString("runtime")
	}
	if v.IsSet("log-level") && !cmd.Flag("log-level").Changed {
		o.Global.LogLevel = v.GetString("log-level")
	}
}
