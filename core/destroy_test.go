// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"errors"
	"path/filepath"
	"testing"

	claberrors "github.com/srl-labs/containerlab/errors"
	"github.com/srl-labs/containerlab/labruntime"
)

// makeCopyForDestroy must apply WithTopoPath (or WithLabNameOnly) before WithNodeFilter so that
// filterClabNodes runs against a loaded topology. This mirrors the option order used there.
func TestDestroyMakeCopyOptionOrder_nodeFilterAfterTopo(t *testing.T) {
	t.Parallel()

	topo := filepath.Join("test_data", "topo1.yml")

	c, err := NewContainerLab(
		WithTopoPath(topo, nil),
		WithNodeFilter([]string{"node1"}),
		WithSkippedBindsPathsCheck(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := c.Config.Topology.Nodes["node1"]; !ok {
		t.Fatal("expected node1 to remain after filter")
	}

	if _, ok := c.Config.Topology.Nodes["node2"]; ok {
		t.Fatal("expected node2 to be removed by node filter")
	}
}

func TestDestroyMakeCopyOptionOrder_nodeFilterBeforeTopoFails(t *testing.T) {
	t.Parallel()

	topo := filepath.Join("test_data", "topo1.yml")

	_, err := NewContainerLab(
		WithNodeFilter([]string{"node1"}),
		WithTopoPath(topo, nil),
		WithSkippedBindsPathsCheck(),
	)
	if err == nil {
		t.Fatal("expected error when node filter is applied before topology is loaded")
	}

	if !errors.Is(err, claberrors.ErrIncorrectInput) {
		t.Fatalf("expected ErrIncorrectInput, got %v", err)
	}
}

func TestWithLabNameOnly_setsNameWithoutTopologyFile(t *testing.T) {
	t.Parallel()

	c, err := NewContainerLab(WithLabNameOnly("my-lab"))
	if err != nil {
		t.Fatal(err)
	}

	if c.Config.Name != "my-lab" {
		t.Fatalf("Config.Name = %q, want my-lab", c.Config.Name)
	}

	if c.TopoPaths.TopologyFileIsSet() {
		t.Fatal("topology file should not be set for lab-name-only init")
	}
}

type noopLabRuntime struct {
	labruntime.LabRuntime
}

func TestWithKeepMgmtNet_noopsForLabRuntime(t *testing.T) {
	t.Parallel()

	c := &CLab{
		LabRuntime:        noopLabRuntime{},
		globalRuntimeName: labruntime.ClabernetesRuntimeName,
	}

	if err := WithKeepMgmtNet()(c); err != nil {
		t.Fatalf("WithKeepMgmtNet returned error for lab runtime: %v", err)
	}
}
