package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	clabcore "github.com/srl-labs/containerlab/core"
)

func TestApplyCommandRegistered(t *testing.T) {
	optionsInstance = nil

	cmd, err := Entrypoint()
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	apply := findCommand(cmd, "apply")
	if apply == nil {
		t.Fatal("apply command is not registered")
	}

	for _, flagName := range []string{"dry-run", "max-workers", "skip-post-deploy", "export-template"} {
		if apply.Flags().Lookup(flagName) == nil {
			t.Fatalf("apply command missing %q flag", flagName)
		}
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
