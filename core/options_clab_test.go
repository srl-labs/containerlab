// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"path/filepath"
	"testing"
)

func TestWithTopologyVarsFiles_setsAbsPathWithoutTopology(t *testing.T) {
	t.Parallel()

	// the following isn't an actual vars file, but we just need a valid path to check the abs handling:
	varsPath := filepath.Join("test_data", "topo1.yml")

	c, err := NewContainerLab(WithTopologyVarsFiles([]string{varsPath}))
	if err != nil {
		t.Fatal(err)
	}

	got := c.TopoPaths.VarsFilenamesAbsPath()
	if len(got) != 1 {
		t.Fatalf("expected exactly one VarsFilenameAbsPaths, got %d", len(got))
	}

	if !filepath.IsAbs(got[0]) {
		t.Fatalf("VarsFilenameAbsPath = %q, want absolute path", got[0])
	}

	if c.TopoPaths.TopologyFileIsSet() {
		t.Fatal("topology file should not be set when only vars are provided")
	}
}

func TestWithTopologyVarsFiles_setMultiple(t *testing.T) {
	t.Parallel()

	varsPaths := []string{
		filepath.Join("test_data", "topo1.yml"),
		filepath.Join("test_data", "topo4.yml"),
		filepath.Join("test_data", "topo3.yml"),
	}

	c, err := NewContainerLab(WithTopologyVarsFiles(varsPaths))
	if err != nil {
		t.Fatal(err)
	}

	got := c.TopoPaths.VarsFilenamesAbsPath()
	if len(got) != 3 {
		t.Fatalf("expected exactly 3 VarsFilenameAbsPaths, got %d", len(got))
	}

	if filepath.Base(got[0]) != "topo1.yml" ||
		filepath.Base(got[1]) != "topo4.yml" ||
		filepath.Base(got[2]) != "topo3.yml" {
		t.Fatal("Expected VarsFiles order to be kept")
	}
}
