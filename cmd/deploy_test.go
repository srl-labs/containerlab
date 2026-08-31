package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

func TestApplyIsDeployAlias(t *testing.T) {
	optionsInstance = nil

	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	if apply := findCommand(cmd, "apply"); apply != nil {
		t.Fatal("apply must not be a standalone command")
	}

	deploy := findCommand(cmd, "deploy")
	if deploy == nil {
		t.Fatal("deploy command is not registered")
	}

	if !deploy.HasAlias("apply") {
		t.Fatal("deploy command is missing the apply alias")
	}

	for _, flagName := range []string{
		"dry-run",
		"max-workers",
		"skip-post-deploy",
		"export-template",
		"image-pull-secret",
	} {
		if deploy.Flags().Lookup(flagName) == nil {
			t.Fatalf("deploy command missing %q flag", flagName)
		}
	}
}

func TestDeployImagePullSecretFlagDefault(t *testing.T) {
	optionsInstance = nil

	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	deploy := findCommand(cmd, "deploy")
	if deploy == nil {
		t.Fatal("deploy command is not registered")
	}

	flag := deploy.Flags().Lookup("image-pull-secret")
	if flag == nil {
		t.Fatal("deploy command missing image-pull-secret flag")
	}
	if flag.DefValue != "" {
		t.Fatalf("image-pull-secret default = %q, want no implicit pull secret", flag.DefValue)
	}

	redeploy := findCommand(cmd, "redeploy")
	if redeploy == nil {
		t.Fatal("redeploy command is not registered")
	}
	if redeploy.Flags().Lookup("image-pull-secret") == nil {
		t.Fatal("redeploy command missing image-pull-secret flag")
	}
}

func TestPostDeployVersionDisplaySkipsLabRuntimes(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		want    bool
	}{
		{name: "default runtime", want: true},
		{name: "docker runtime", runtime: "docker", want: true},
		{name: "clabernetes runtime", runtime: clablabruntime.ClabernetesRuntimeName, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDisplayPostDeployVersion(tt.runtime); got != tt.want {
				t.Fatalf(
					"shouldDisplayPostDeployVersion(%q) = %t, want %t",
					tt.runtime,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestPrintApplyResultUsesInfoAndItemRows(t *testing.T) {
	output := captureApplyOutput(t, func() {
		printApplyResult(&clabcore.ApplyResult{
			AddedNodes:       []string{"l3"},
			RecreatedNodes:   []string{"xrd1"},
			DeletedEndpoints: []string{"l1:eth2", "l2:eth2"},
		})
	})

	for _, want := range []string{
		"INFO",
		"Apply summary",
		"Action",
		"Details",
		"added nodes",
		"l3",
		"recreated nodes",
		"xrd1",
		"deleted endpoints",
		"l1:eth2",
		"l2:eth2",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}

	if strings.Contains(output, "added nodes:") {
		t.Fatalf("expected table output, got old label format:\n%s", output)
	}
	if strings.Contains(output, "l1:eth2, l2:eth2") {
		t.Fatalf("expected one row per table item, got joined details:\n%s", output)
	}
}

func captureApplyOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	log.SetOutput(w)
	log.SetTimeFormat(time.TimeOnly)
	defer func() {
		os.Stdout = oldStdout
		log.SetOutput(os.Stderr)
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	return string(output)
}
