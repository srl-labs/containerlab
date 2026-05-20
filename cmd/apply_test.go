package cmd

import "testing"

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
