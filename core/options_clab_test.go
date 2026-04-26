// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"path/filepath"
	"testing"
)

func TestWithTopologyVarsFile_setsAbsPathWithoutTopology(t *testing.T) {
	t.Parallel()

	varsPath := filepath.Join("test_data", "topo1.yml")

	c, err := NewContainerLab(WithTopologyVarsFile(varsPath))
	if err != nil {
		t.Fatal(err)
	}

	got := c.TopoPaths.VarsFilenameAbsPath()
	if got == "" {
		t.Fatal("expected non-empty VarsFilenameAbsPath")
	}

	if !filepath.IsAbs(got) {
		t.Fatalf("VarsFilenameAbsPath = %q, want absolute path", got)
	}

	if c.TopoPaths.TopologyFileIsSet() {
		t.Fatal("topology file should not be set when only vars are provided")
	}
}
