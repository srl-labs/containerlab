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

	// Replace hyphens, slashes, and dots with underscores in environment variable names
	// This allows keys like "deploy.graph" to match env var "CLAB_DEPLOY_GRAPH"
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", "/", "_", ".", "_"))

	// Automatically bind environment variables
	v.AutomaticEnv()

	// Bind all flags to viper
	return bindFlags(cmd, v)
}

// bindFlags binds all cobra flags to viper for a command and its subcommands.
// It uses command hierarchy to create namespaced keys for environment variables.
// For example, the --container flag in "tools disable-tx-offload" becomes:
// CLAB_TOOLS_DISABLE_TX_OFFLOAD_CONTAINER
func bindFlags(cmd *cobra.Command, v *viper.Viper) error {
	return bindFlagsWithPath(cmd, v, "")
}

// bindFlagsWithPath recursively binds flags with their command path as prefix.
func bindFlagsWithPath(cmd *cobra.Command, v *viper.Viper, cmdPath string) error {
	// Build the current command path
	currentPath := cmdPath
	isRootCmd := cmd.Name() == "containerlab" || cmd.Name() == ""

	if !isRootCmd {
		if currentPath != "" {
			currentPath = currentPath + "." + cmd.Name()
		} else {
			currentPath = cmd.Name()
		}
	}

	// Bind persistent flags
	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		// For root command persistent flags, bind WITHOUT prefix so they work globally
		// as CLAB_<FLAG> from any command
		if isRootCmd {
			_ = v.BindPFlag(flag.Name, flag)
		}

		// Also bind with command path if we have one (for subcommands)
		if currentPath != "" {
			key := currentPath + "." + flag.Name
			_ = v.BindPFlag(key, flag)
		}
	})

	// Bind local flags with command path prefix
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		// Skip if this flag is a persistent flag (already bound above)
		if cmd.PersistentFlags().Lookup(flag.Name) != nil {
			return
		}

		// Local flags MUST have a command path prefix
		// Don't bind without prefix to avoid ambiguity
		if currentPath != "" {
			key := currentPath + "." + flag.Name
			_ = v.BindPFlag(key, flag)
		}
	})

	// Recursively bind flags for all subcommands
	for _, subCmd := range cmd.Commands() {
		if err := bindFlagsWithPath(subCmd, v, currentPath); err != nil {
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
	// Build command path for this command
	cmdPath := getCommandPath(cmd)

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
		updateFlagFromViper(f, cmdPath)
	}
}

// getCommandPath builds the command path from root to current command.
// For example: "tools.disable-tx-offload"
func getCommandPath(cmd *cobra.Command) string {
	var parts []string
	current := cmd

	for current != nil && current.Name() != "containerlab" && current.Name() != "" {
		parts = append([]string{current.Name()}, parts...)
		current = current.Parent()
	}

	return strings.Join(parts, ".")
}

// updateFlagFromViper updates a single flag's value from viper if:
// - The flag was not explicitly set on the command line
// - Viper has a value for this flag (from env var or config file).
// It checks for the value using the command-namespaced key.
func updateFlagFromViper(f *pflag.Flag, cmdPath string) {
	// Skip if flag was explicitly set via command line
	if f.Changed {
		return
	}

	// Determine the key to check in viper
	// Strategy:
	// 1. First check if flag is bound with full command path (e.g., version.short)
	// 2. If not found AND the flag name itself is a bound key (root persistent flags),
	//    then check that key
	var key string
	var hasValue bool

	if cmdPath != "" {
		// For commands with a path, first try the full command.flag format
		key = cmdPath + "." + f.Name
		hasValue = v.IsSet(key)

		// If not found with command path, check if this flag name is bound at root level
		// This happens for root persistent flags which are bound without prefix
		if !hasValue {
			// Check if the flag name itself is a viper key (indicates root persistent flag)
			// We use AllKeys to check all bound keys
			isRootKey := false
			for _, k := range v.AllKeys() {
				if k == f.Name {
					isRootKey = true
					break
				}
			}

			// Only fall back to unprefixed key if it's actually bound at root
			if isRootKey {
				key = f.Name
				hasValue = v.IsSet(key)
			}
		}
	} else {
		// For commands without a path (shouldn't normally happen)
		key = f.Name
		hasValue = v.IsSet(key)
	}

	// Skip if viper doesn't have a value for this flag
	if !hasValue {
		return
	}

	// Get the value from viper based on the flag's type and set it
	// The flag.Value.Set() method handles type conversion for us
	var val string

	switch f.Value.Type() {
	case "bool":
		val = v.GetString(key)
	case "string":
		val = v.GetString(key)
	case "stringSlice":
		// For slices, join with comma as that's what cobra expects
		slice := v.GetStringSlice(key)
		if len(slice) > 0 {
			val = strings.Join(slice, ",")
		}
	case "int", "int8", "int16", "int32", "int64":
		val = v.GetString(key)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		val = v.GetString(key)
	case "float32", "float64":
		val = v.GetString(key)
	case "duration":
		val = v.GetString(key)
	case "count":
		val = v.GetString(key)
	default:
		// For any other type, try to get it as a string
		val = v.GetString(key)
	}

	// Set the value on the flag (which updates the underlying variable)
	if val != "" {
		_ = f.Value.Set(val)
	}
}
