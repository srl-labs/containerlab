package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

func TestDocsLinkFromVer(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "major and minor only",
			version:  "0.47.0",
			expected: "0.47/",
		},
		{
			name:     "major, minor, and patch version",
			version:  "0.47.2",
			expected: "0.47/#0472",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := docsLinkFromVer(tt.version)
			if diff := cmp.Diff(got, tt.expected); diff != "" {
				t.Fatalf("docsLinkFromVer() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVersionUpdateAlias(t *testing.T) {
	o := GetOptions()
	cmd, err := versionCmd(o)
	if err != nil {
		t.Fatalf("Failed to create version command: %v", err)
	}

	// Find the upgrade subcommand
	var upgradeCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Use == "upgrade" {
			upgradeCmd = subCmd
			break
		}
	}

	if upgradeCmd == nil {
		t.Fatal("upgrade subcommand not found")
	}

	// Check that the upgrade command has the "update" alias
	aliases := upgradeCmd.Aliases
	found := false
	for _, alias := range aliases {
		if alias == "update" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected 'update' alias for upgrade command, but it was not found. Aliases: %v", aliases)
	}
}
