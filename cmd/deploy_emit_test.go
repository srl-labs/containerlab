package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
)

func TestDeployEmitCRsFlag(t *testing.T) {
	optionsInstance = nil

	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	deploy := findCommand(cmd, "deploy")
	if deploy == nil {
		t.Fatal("deploy command is not registered")
	}
	flag := deploy.Flags().Lookup("emit-crs")
	if flag == nil {
		t.Fatal("deploy command missing emit-crs flag")
	}
	if flag.DefValue != "false" {
		t.Fatalf("emit-crs default = %q, want false", flag.DefValue)
	}

	// redeploy always destroys and recreates the lab, so it has no manifest-only mode.
	redeploy := findCommand(cmd, "redeploy")
	if redeploy == nil {
		t.Fatal("redeploy command is not registered")
	}
	if redeploy.Flags().Lookup("emit-crs") != nil {
		t.Fatal("redeploy command must not expose the emit-crs flag")
	}
}

func TestValidateEmitCRsFlags(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		deploy  DeployOptions
		wantErr string
	}{
		{
			name:    "not requested",
			runtime: clabruntimedocker.RuntimeName,
			deploy:  DeployOptions{DryRun: true, Reconfigure: true},
		},
		{
			name:    "lab runtime",
			runtime: clablabruntime.ClabernetesRuntimeName,
			deploy:  DeployOptions{EmitCRs: true, NoTopologyCR: true},
		},
		{
			name:    "container runtime",
			runtime: clabruntimedocker.RuntimeName,
			deploy:  DeployOptions{EmitCRs: true},
			wantErr: "only supported with the \"c9s\" runtime",
		},
		{
			name:    "dry-run",
			runtime: clablabruntime.ClabernetesRuntimeName,
			deploy:  DeployOptions{EmitCRs: true, DryRun: true},
			wantErr: "cannot be combined with --dry-run",
		},
		{
			name:    "reconfigure",
			runtime: clablabruntime.ClabernetesRuntimeName,
			deploy:  DeployOptions{EmitCRs: true, Reconfigure: true},
			wantErr: "cannot be combined with --reconfigure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploy := tt.deploy
			o := &Options{
				Global: &GlobalOptions{Runtime: tt.runtime},
				Deploy: &deploy,
			}

			err := validateEmitCRsFlags(o)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func testManifests() []clablabruntime.Manifest {
	return []clablabruntime.Manifest{
		{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       "c9s-lab1",
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]any{"name": "c9s-lab1"},
			},
		},
		{
			APIVersion: "c9s.run/v1alpha1",
			Kind:       "Topology",
			Namespace:  "c9s-lab1",
			Name:       "lab1",
			Object: map[string]any{
				"apiVersion": "c9s.run/v1alpha1",
				"kind":       "Topology",
				"metadata":   map[string]any{"name": "lab1", "namespace": "c9s-lab1"},
				"spec": map[string]any{
					"definition": map[string]any{"containerlab": "name: lab1\n"},
				},
			},
		},
	}
}

func TestPrintLabRuntimeManifestsYAMLStream(t *testing.T) {
	var out bytes.Buffer
	if err := printLabRuntimeManifests(&out, testManifests(), clabconstants.FormatTable); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	documents := strings.Split(strings.TrimSpace(got), "---\n")
	// A leading separator produces an empty first element; two documents follow.
	if len(documents) != 3 || documents[0] != "" {
		t.Fatalf("expected two YAML documents separated by ---, got:\n%s", got)
	}
	if !strings.Contains(documents[1], "kind: Namespace") ||
		!strings.Contains(documents[2], "kind: Topology") {
		t.Fatalf("documents are out of order or incomplete:\n%s", got)
	}
	if !strings.Contains(documents[2], "containerlab: |") &&
		!strings.Contains(documents[2], "containerlab: 'name: lab1") &&
		!strings.Contains(documents[2], `containerlab: "name: lab1\n"`) {
		t.Fatalf("topology definition is not rendered as a YAML string:\n%s", documents[2])
	}
}

func TestPrintLabRuntimeManifestsJSONList(t *testing.T) {
	var out bytes.Buffer
	if err := printLabRuntimeManifests(&out, testManifests(), clabconstants.FormatJSON); err != nil {
		t.Fatal(err)
	}

	var list struct {
		APIVersion string           `json:"apiVersion"`
		Kind       string           `json:"kind"`
		Items      []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if list.APIVersion != "v1" || list.Kind != "List" || len(list.Items) != 2 {
		t.Fatalf("unexpected list envelope: %+v", list)
	}
	if list.Items[0]["kind"] != "Namespace" || list.Items[1]["kind"] != "Topology" {
		t.Fatalf("unexpected list items: %+v", list.Items)
	}
}
