// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	claberrors "github.com/srl-labs/containerlab/errors"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
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
	clablabruntime.LabRuntime
}

func TestWithKeepMgmtNet_noopsForLabRuntime(t *testing.T) {
	t.Parallel()

	c := &CLab{
		LabRuntime:        noopLabRuntime{},
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	if err := WithKeepMgmtNet()(c); err != nil {
		t.Fatalf("WithKeepMgmtNet returned error for lab runtime: %v", err)
	}
}

func TestAddMgmtNetworksFromContainers(t *testing.T) {
	m := &clabtypes.MgmtNet{Network: "clab"}
	c := &CLab{Config: &Config{Mgmt: m, MgmtNetworks: clabtypes.MgmtNetworks{m}}}

	ctr := func(network, bridge string) clabruntime.GenericContainer {
		return clabruntime.GenericContainer{
			NetworkName: network,
			Labels:      map[string]string{clabconstants.NodeMgmtNetBr: bridge},
		}
	}
	c.addMgmtNetworksFromContainers([]clabruntime.GenericContainer{
		ctr("clab", "br-clab"),
		ctr("oob", "br-oob"),
		ctr("oob", "br-oob"),
		ctr("host", ""),
		ctr("unknown", ""),
	})

	got := c.Config.MgmtNetworks
	if len(got) != 2 || got[1].Network != "oob" || got[1].Bridge != "br-oob" ||
		got[1].ExternalAccess == nil || !*got[1].ExternalAccess {
		t.Fatalf("MgmtNetworks = %+v", got)
	}
}

func TestMgmtNetworksForCleanupIncludesStateRecordedNetworks(t *testing.T) {
	t.Parallel()

	paths := &clabtypes.TopoPaths{}
	if err := paths.SetLabDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	main := &clabtypes.MgmtNet{Network: "clab-lab-main"}
	oob := &clabtypes.MgmtNet{Network: "clab-lab-oob"}

	seed := &CLab{
		TopoPaths: paths,
		Config:    &Config{Mgmt: main, MgmtNetworks: clabtypes.MgmtNetworks{main, oob}},
	}
	if err := seed.WriteState(); err != nil {
		t.Fatal(err)
	}

	// The topology dropped the oob network; destroy must still delete it.
	c := &CLab{
		TopoPaths: paths,
		Config:    &Config{Mgmt: main, MgmtNetworks: clabtypes.MgmtNetworks{main}},
	}
	networks := c.mgmtNetworksForCleanup()
	if len(networks) != 2 || networks[0] != main || networks[1].Network != "clab-lab-oob" ||
		networks[1].ExternalAccess == nil {
		t.Fatalf("mgmtNetworksForCleanup = %+v", networks)
	}

	// Without a state file only the topology-defined networks are returned.
	c.Config.MgmtNetworks = clabtypes.MgmtNetworks{main, oob}
	if err := os.Remove(paths.StateFile()); err != nil {
		t.Fatal(err)
	}
	networks = c.mgmtNetworksForCleanup()
	if len(networks) != 2 || networks[0] != main || networks[1] != oob {
		t.Fatalf("mgmtNetworksForCleanup without state = %+v", networks)
	}
}

func TestWithDestroyKeepLinks(t *testing.T) {
	t.Parallel()

	opts := NewDestroyOptions()
	WithDestroyKeepLinks()(opts)

	if !opts.keepLinks {
		t.Fatal("WithDestroyKeepLinks did not enable link preservation")
	}
}
