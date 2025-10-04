// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
// This iterates through all flags that were bound to viper and updates their
// values from environment variables if the flag was not explicitly set.
func updateOptionsFromViper(cmd *cobra.Command, _ *Options) {
	// Collect all flags that should be checked (avoid duplicates)
	flagMap := make(map[string]*pflag.Flag)
	
	// Helper to add flags to map
	addFlags := func(fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			if _, exists := flagMap[f.Name]; !exists {
				flagMap[f.Name] = f
			}
		})
	}
	
	// Add this command's local flags
	addFlags(cmd.Flags())
	
	// Add this command's persistent flags
	addFlags(cmd.PersistentFlags())
	
	// Add inherited persistent flags from all parent commands
	parent := cmd.Parent()
	for parent != nil {
		addFlags(parent.PersistentFlags())
		parent = parent.Parent()
	}
	
	// Now update all collected flags from viper
	for _, f := range flagMap {
		updateFlagFromViper(f)
	}
}

// updateFlagFromViper updates a single flag's value from viper if:
// - The flag was not explicitly set on the command line
// - Viper has a value for this flag (from env var or config file)
func updateFlagFromViper(f *pflag.Flag) {
	// Skip if flag was explicitly set via command line
	if f.Changed {
		return
	}

	// Skip if viper doesn't have a value for this flag
	if !v.IsSet(f.Name) {
		return
	}

	// Get the value from viper based on the flag's type and set it
	// The flag.Value.Set() method handles type conversion for us
	var val string
	
	switch f.Value.Type() {
	case "bool":
		val = v.GetString(f.Name)
	case "string":
		val = v.GetString(f.Name)
	case "stringSlice":
		// For slices, join with comma as that's what cobra expects
		slice := v.GetStringSlice(f.Name)
		if len(slice) > 0 {
			val = strings.Join(slice, ",")
		}
	case "int", "int8", "int16", "int32", "int64":
		val = v.GetString(f.Name)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		val = v.GetString(f.Name)
	case "float32", "float64":
		val = v.GetString(f.Name)
	case "duration":
		val = v.GetString(f.Name)
	case "count":
		val = v.GetString(f.Name)
	default:
		// For any other type, try to get it as a string
		val = v.GetString(f.Name)
	}

	// Set the value on the flag (which updates the underlying variable)
	if val != "" {
		_ = f.Value.Set(val)
	}
}
