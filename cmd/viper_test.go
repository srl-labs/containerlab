// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

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
			// Save original value if exists
			originalVal := os.Getenv(tt.envKey)
			defer func() {
				if originalVal != "" {
					os.Setenv(tt.envKey, originalVal)
				} else {
					os.Unsetenv(tt.envKey)
				}
			}()

			// Set environment variable
			os.Setenv(tt.envKey, tt.envValue)

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

			// Clean up
			os.Unsetenv(tt.envKey)
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
			// Save original value if exists
			originalVal := os.Getenv(tt.envKey)
			defer func() {
				if originalVal != "" {
					os.Setenv(tt.envKey, originalVal)
				} else {
					os.Unsetenv(tt.envKey)
				}
			}()

			// Set environment variable
			os.Setenv(tt.envKey, tt.envValue)

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

			// Clean up
			os.Unsetenv(tt.envKey)
		})
	}
}

func TestViperFlagTakesPrecedence(t *testing.T) {
	// Set environment variable
	os.Setenv("CLAB_LOG_LEVEL", "debug")
	defer os.Unsetenv("CLAB_LOG_LEVEL")

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
