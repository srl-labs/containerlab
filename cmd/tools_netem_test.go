package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestNetemNodeFlagRetainsLegacyBinding(t *testing.T) {
	o := &Options{
		Global:     &GlobalOptions{},
		ToolsNetem: &ToolsNetemOptions{},
	}

	cmd, err := netemCmd(o)
	if err != nil {
		t.Fatalf("netemCmd() error = %v", err)
	}

	for _, name := range []string{"set", "show", "reset"} {
		t.Run(name, func(t *testing.T) {
			subcommand := netemSubcommand(t, cmd, name)
			flag := subcommand.Flags().Lookup("node")
			if flag == nil {
				t.Fatal("missing --node flag")
			}
			if flag.Shorthand != "n" {
				t.Fatalf("--node shorthand = %q, want %q", flag.Shorthand, "n")
			}
			if subcommand.Flags().Lookup("container") != nil {
				t.Fatal("unexpected --container flag")
			}

			want := "clab-test-sros-1"
			if err := subcommand.Flags().Set("node", want); err != nil {
				t.Fatalf("set --node: %v", err)
			}
			if o.ToolsNetem.ContainerName != want {
				t.Fatalf("ContainerName = %q, want %q", o.ToolsNetem.ContainerName, want)
			}
		})
	}
}

func TestResolveNetemNodeFromTopology(t *testing.T) {
	topologyPath := filepath.Join(t.TempDir(), "netem.clab.yml")
	topology := []byte(`name: netem-test
topology:
  nodes:
    sros:
      kind: host
`)
	if err := os.WriteFile(topologyPath, topology, 0o600); err != nil {
		t.Fatalf("write topology: %v", err)
	}

	o := &Options{
		Global: &GlobalOptions{
			TopologyFile: topologyPath,
			Runtime:      "docker",
			Timeout:      time.Second,
		},
		Filter:     &FilterOptions{},
		Deploy:     &DeployOptions{},
		Destroy:    &DestroyOptions{},
		ToolsNetem: &ToolsNetemOptions{ContainerName: "sros"},
	}

	node, err := resolveNetemNode(context.Background(), o)
	if err != nil {
		t.Fatalf("resolveNetemNode() error = %v", err)
	}

	target, err := node.TargetFor("eth1")
	if err != nil {
		t.Fatalf("TargetFor() error = %v", err)
	}
	if target.NSPath == "" {
		t.Fatal("TargetFor() returned an empty namespace path")
	}
	if target.Iface != "eth1" {
		t.Fatalf("TargetFor() interface = %q, want %q", target.Iface, "eth1")
	}
	if target.DisplayName != "sros" {
		t.Fatalf("TargetFor() display name = %q, want %q", target.DisplayName, "sros")
	}
}

func netemSubcommand(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, subcommand := range cmd.Commands() {
		if subcommand.Name() == name {
			return subcommand
		}
	}

	t.Fatalf("missing %q subcommand", name)
	return nil
}
