// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

// setEnvWithCleanup sets an environment variable and returns a cleanup function
// to restore the original value.
func setEnvWithCleanup(key, value string) func() {
	original := os.Getenv(key)
	os.Setenv(key, value)

	return func() {
		if original != "" {
			os.Setenv(key, original)
		} else {
			os.Unsetenv(key)
		}
	}
}

func TestViperEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		flagName string
		check    func(*Options) bool
	}{
		{
			name:     "CLAB_LOG_LEVEL sets log level",
			envKey:   "CLAB_LOG_LEVEL",
			envValue: "debug",
			flagName: "log-level",
			check: func(o *Options) bool {
				return o.Global.LogLevel == "debug"
			},
		},
		{
			name:     "CLAB_RUNTIME sets runtime",
			envKey:   "CLAB_RUNTIME",
			envValue: "docker",
			flagName: "runtime",
			check: func(o *Options) bool {
				return o.Global.Runtime == "docker"
			},
		},
		{
			name:     "CLAB_TIMEOUT sets timeout",
			envKey:   "CLAB_TIMEOUT",
			envValue: "300s",
			flagName: "timeout",
			check: func(o *Options) bool {
				return o.Global.Timeout == 300*time.Second
			},
		},
		{
			name:     "CLAB_NAME sets topology name",
			envKey:   "CLAB_NAME",
			envValue: "test-lab",
			flagName: "name",
			check: func(o *Options) bool {
				return o.Global.TopologyName == "test-lab"
			},
		},
		{
			name:     "CLAB_TOPO sets topology file",
			envKey:   "CLAB_TOPO",
			envValue: "/path/to/topo.yml",
			flagName: "topo",
			check: func(o *Options) bool {
				return o.Global.TopologyFile == "/path/to/topo.yml"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer setEnvWithCleanup(tt.envKey, tt.envValue)()

			// Reset options instance to get fresh defaults
			optionsInstance = nil

			// Create command
			cmd, err := Entrypoint()
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			// Execute prerun to trigger viper update
			o := GetOptions()

			err = preRunFn(cmd, o)
			if err != nil {
				t.Fatalf("PreRun failed: %v", err)
			}

			// Check if the value was set correctly
			if !tt.check(o) {
				t.Errorf("Environment variable %s did not set the expected value", tt.envKey)
			}
		})
	}
}

func TestViperEnvKeyReplacer(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(*Options) bool
	}{
		{
			name:     "Hyphen in flag name replaced with underscore",
			envKey:   "CLAB_LOG_LEVEL",
			envValue: "error",
			check: func(o *Options) bool {
				return o.Global.LogLevel == "error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer setEnvWithCleanup(tt.envKey, tt.envValue)()

			// Reset options instance to get fresh defaults
			optionsInstance = nil

			// Create command
			cmd, err := Entrypoint()
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			// Execute prerun to trigger viper update
			o := GetOptions()

			err = preRunFn(cmd, o)
			if err != nil {
				t.Fatalf("PreRun failed: %v", err)
			}

			// Check if the value was set correctly
			if !tt.check(o) {
				t.Errorf("Environment variable %s did not set the expected value", tt.envKey)
			}
		})
	}
}

func TestViperFlagTakesPrecedence(t *testing.T) {
	defer setEnvWithCleanup("CLAB_LOG_LEVEL", "debug")()

	// Reset options instance to get fresh defaults
	optionsInstance = nil

	// Create command
	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Set flag explicitly
	cmd.SetArgs([]string{"version", "--log-level=error"})

	// Execute command
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Get options
	o := GetOptions()

	// Check that flag value took precedence over env var
	if !cmp.Equal(o.Global.LogLevel, "error") {
		t.Errorf("Expected log level to be 'error' (from flag), got '%s'", o.Global.LogLevel)
	}
}

func TestViperSubcommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(*Options) bool
	}{
		{
			name:     "CLAB_DEPLOY_GRAPH sets graph generation flag",
			envKey:   "CLAB_DEPLOY_GRAPH",
			envValue: "true",
			check: func(o *Options) bool {
				return o.Deploy.GenerateGraph == true
			},
		},
		{
			name:     "CLAB_DEPLOY_RECONFIGURE sets reconfigure flag",
			envKey:   "CLAB_DEPLOY_RECONFIGURE",
			envValue: "true",
			check: func(o *Options) bool {
				return o.Deploy.Reconfigure == true
			},
		},
		{
			name:     "CLAB_DEPLOY_MAX_WORKERS sets max workers",
			envKey:   "CLAB_DEPLOY_MAX_WORKERS",
			envValue: "10",
			check: func(o *Options) bool {
				return o.Deploy.MaxWorkers == 10
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer setEnvWithCleanup(tt.envKey, tt.envValue)()
			// Also set a topology name to avoid the file search
			defer setEnvWithCleanup("CLAB_NAME", "test-lab")()

			// Reset options instance to get fresh defaults
			optionsInstance = nil

			// Create command
			cmd, err := Entrypoint()
			if err != nil {
				t.Fatalf("Failed to create command: %v", err)
			}

			// Find the deploy subcommand
			deployCmd := findCommand(cmd, "deploy")
			if deployCmd == nil {
				t.Fatal("Deploy command not found")
			}

			// Execute prerun on deploy command (use root command's prerun to avoid deploy-specific
			// checks)
			o := GetOptions()

			err = preRunFn(deployCmd, o)
			if err != nil {
				t.Fatalf("PreRun failed: %v", err)
			}

			// Check if the value was set correctly
			if !tt.check(o) {
				t.Errorf("Environment variable %s did not set the expected value", tt.envKey)
			}
		})
	}
}

// findCommand finds a subcommand by name.
func findCommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == name {
			return subCmd
		}
	}

	return nil
}

func TestViperNestedCommandFlags(t *testing.T) {
	// Test for nested commands like "tools disable-tx-offload"
	// Environment variable should be CLAB_TOOLS_DISABLE_TX_OFFLOAD_CONTAINER
	defer setEnvWithCleanup("CLAB_TOOLS_DISABLE_TX_OFFLOAD_CONTAINER", "test-container")()
	// Also set a topology name to avoid the file search
	defer setEnvWithCleanup("CLAB_NAME", "test-lab")()

	// Reset options instance to get fresh defaults
	optionsInstance = nil

	// Create command
	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Find the tools command
	toolsCmd := findCommand(cmd, "tools")
	if toolsCmd == nil {
		t.Fatal("Tools command not found")
	}

	// Find the disable-tx-offload subcommand
	disableTxCmd := findCommand(toolsCmd, "disable-tx-offload")
	if disableTxCmd == nil {
		t.Fatal("disable-tx-offload command not found")
	}

	// Execute prerun on the command
	o := GetOptions()

	err = preRunFn(disableTxCmd, o)
	if err != nil {
		t.Fatalf("PreRun failed: %v", err)
	}

	// Check if the value was set correctly
	if o.ToolsTxOffload.ContainerName != "test-container" {
		t.Errorf(
			"Expected container name to be 'test-container', got '%s'",
			o.ToolsTxOffload.ContainerName,
		)
	}
}
