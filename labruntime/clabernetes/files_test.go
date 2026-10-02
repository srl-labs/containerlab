package clabernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

func TestStageNodeFileReferences(t *testing.T) {
	t.Parallel()
	const content = "first line\nsecond line\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	for _, tt := range []struct {
		name, startupConfig, license, wantPath string
	}{
		{
			"remote startup", server.URL + "/router.partial.cfg", "",
			"/clabernetes/r1/startup-config/router.partial.cfg",
		},
		{"remote license", "", server.URL + "/license.txt", "/clabernetes/r1/license/license.txt"},
		{"embedded license", "", content, "/clabernetes/r1/license/embedded.lic"},
		{"embedded startup", content, "", inlineStartupConfigMountPath},
		{"license variable", "", "__clabNodeName__.lic", "r1.lic"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			topologyDir := t.TempDir()
			writeFile(t, filepath.Join(topologyDir, "r1.lic"), content, 0o644)
			config := &clabRuntimeConfig{
				Name: "lab1",
				Topology: &clabtypes.Topology{Nodes: map[string]*clabtypes.NodeDefinition{
					"r1": {
						Kind:          "nokia_srsim",
						StartupConfig: tt.startupConfig,
						License:       tt.license,
					},
				}},
			}
			definition, err := yaml.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			rendered, staged, _, err := stageTopologyLocalFiles(
				context.Background(),
				clablabruntime.DeployRequest{
					Name: "lab1", TopologyFile: filepath.Join(topologyDir, "lab.clab.yml"),
					TopologyDefinition: definition,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(rendered, config); err != nil {
				t.Fatal(err)
			}
			gotPath := config.Topology.Nodes["r1"].License
			if tt.startupConfig != "" {
				gotPath = config.Topology.Nodes["r1"].StartupConfig
			}
			if gotPath != tt.wantPath {
				t.Fatalf("rendered path = %q, want %q", gotPath, tt.wantPath)
			}
			if len(staged) != 1 || len(staged[0].mounts) != 1 ||
				staged[0].mounts[0].filePath != gotPath {
				t.Fatalf("staged mounts do not match rendered reference: %+v", staged)
			}
			gotContent, _ := staged[0].content("file")
			if string(gotContent) != content {
				t.Fatalf("staged content = %q, want %q", gotContent, content)
			}
		})
	}
}

func TestStageConfigMapFileRejectsOversizedEmbeddedContent(t *testing.T) {
	t.Parallel()
	err := stageConfigMapFile(
		map[string]*stagedConfigMap{},
		"lab",
		"inline",
		"r1",
		inlineStartupConfigMountPath,
		fileModeRead,
		[]byte(strings.Repeat("x", maxConfigMapFileBytes+1)),
	)
	if err == nil || !strings.Contains(err.Error(), "ConfigMap file limit") {
		t.Fatalf("oversized inline content error = %v", err)
	}
}

func TestResolveNodeFileReferenceHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("canceled download contacted server")
	}))
	defer server.Close()
	_, _, err := resolveNodeFileReference(
		ctx,
		server.URL+"/startup.cfg",
		"startup-config",
		"r1",
		t.TempDir(),
		"",
	)
	if err == nil {
		t.Fatal("canceled download succeeded")
	}
}
