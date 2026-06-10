package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootRequirementHelpers(t *testing.T) {
	tests := []struct {
		name               string
		runtime            string
		wantGlobalRoot     bool
		wantCommandSkipped bool
	}{
		{
			name:               "default runtime",
			runtime:            "",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "docker runtime",
			runtime:            "docker",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "podman runtime",
			runtime:            "podman",
			wantGlobalRoot:     true,
			wantCommandSkipped: false,
		},
		{
			name:               "clabernetes runtime",
			runtime:            "clabernetes",
			wantGlobalRoot:     false,
			wantCommandSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := globalRuntimeRequiresRoot(tt.runtime); got != tt.wantGlobalRoot {
				t.Fatalf("globalRuntimeRequiresRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantGlobalRoot)
			}

			if got := commandSkipsRoot(tt.runtime); got != tt.wantCommandSkipped {
				t.Fatalf("commandSkipsRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantCommandSkipped)
			}
		})
	}
}

func TestCheckLabRuntimeCommandSupport(t *testing.T) {
	root := &cobra.Command{Use: "containerlab"}
	graph := &cobra.Command{Use: "graph"}
	tools := &cobra.Command{Use: "tools"}
	sshx := &cobra.Command{Use: "sshx"}
	deploy := &cobra.Command{Use: "deploy"}

	root.AddCommand(graph, tools, deploy)
	tools.AddCommand(sshx)

	tests := []struct {
		name    string
		runtime string
		cmd     *cobra.Command
		wantErr bool
	}{
		{
			name:    "graph with docker runtime",
			runtime: "docker",
			cmd:     graph,
			wantErr: false,
		},
		{
			name:    "deploy with clabernetes runtime",
			runtime: "clabernetes",
			cmd:     deploy,
			wantErr: false,
		},
		{
			name:    "graph with clabernetes runtime",
			runtime: "clabernetes",
			cmd:     graph,
			wantErr: true,
		},
		{
			name:    "tools subcommand with clabernetes runtime",
			runtime: "clabernetes",
			cmd:     sshx,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkLabRuntimeCommandSupport(tt.cmd, tt.runtime)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkLabRuntimeCommandSupport(%q, %q) error = %v, wantErr %v",
					tt.cmd.Name(), tt.runtime, err, tt.wantErr)
			}
		})
	}
}
