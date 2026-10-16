// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// destroyTestBehavior configures the fake runtime the destroy tests register under the
// "destroytest" name. The behavior is read by every instance NewContainerLab creates for
// the duration of one test.
type destroyTestBehavior struct {
	// containers is returned by the first ListContainers call.
	containers []clabruntime.GenericContainer
	// failCreateNet makes CreateNet return an error, which fails makeCopyForDestroy.
	failCreateNet bool
	// listErrAfterFirst makes every ListContainers call after the first return an
	// error, which fails the inner destroy after the copy was built.
	listErrAfterFirst bool
	// listEmptyAfterFirst makes every ListContainers call after the first return no
	// containers, so the inner destroy completes without touching nodes.
	listEmptyAfterFirst bool
}

var destroyTestRuntimeBehavior atomic.Pointer[destroyTestBehavior]

// destroyTestListCalls counts ListContainers calls across every fake runtime instance
// the destroy tests create: the first call is the outer Destroy inventory, the later
// ones belong to the CLab copy makeCopyForDestroy builds.
var destroyTestListCalls atomic.Int32

var registerDestroyTestRuntimeOnce sync.Once

// registerDestroyTestRuntime registers a minimal fake container runtime the Destroy
// unit tests can drive without a container engine.
func registerDestroyTestRuntime() {
	registerDestroyTestRuntimeOnce.Do(func() {
		clabruntime.Register("destroytest", func() clabruntime.ContainerRuntime {
			return &fakeDestroyRuntime{}
		})
	})
}

// setDestroyTestBehavior installs the behavior for one test and resets the call
// bookkeeping the previous test left behind.
func setDestroyTestBehavior(t *testing.T, behavior *destroyTestBehavior) {
	t.Helper()

	destroyTestListCalls.Store(0)
	destroyTestRuntimeBehavior.Store(behavior)

	t.Cleanup(func() {
		destroyTestRuntimeBehavior.Store(nil)
	})
}

type fakeDestroyRuntime struct {
	clabruntime.ContainerRuntime
	mgmt   *clabtypes.MgmtNet
	config *clabruntime.RuntimeConfig
}

func (f *fakeDestroyRuntime) Init(opts ...clabruntime.RuntimeOption) error {
	for _, opt := range opts {
		opt(f)
	}

	return nil
}

func (f *fakeDestroyRuntime) WithConfig(cfg *clabruntime.RuntimeConfig) { f.config = cfg }

func (f *fakeDestroyRuntime) WithMgmtNet(mgmt *clabtypes.MgmtNet) { f.mgmt = mgmt }

func (f *fakeDestroyRuntime) Mgmt() *clabtypes.MgmtNet { return f.mgmt }

func (*fakeDestroyRuntime) GetName() string { return "destroytest" }

func (f *fakeDestroyRuntime) Config() clabruntime.RuntimeConfig {
	if f.config == nil {
		return clabruntime.RuntimeConfig{}
	}

	return *f.config
}

func (*fakeDestroyRuntime) GetRuntimeSocket() (string, error) { return "", nil }

func (f *fakeDestroyRuntime) ListContainers(
	_ context.Context,
	_ []*clabtypes.GenericFilter,
) ([]clabruntime.GenericContainer, error) {
	behavior := destroyTestRuntimeBehavior.Load()
	if behavior == nil {
		return nil, nil
	}

	if destroyTestListCalls.Add(1) > 1 {
		switch {
		case behavior.listErrAfterFirst:
			return nil, errors.New("synthetic runtime failure")
		case behavior.listEmptyAfterFirst:
			return nil, nil
		}
	}

	return behavior.containers, nil
}

func (*fakeDestroyRuntime) CreateNet(
	context.Context,
	...clabruntime.NetworkCreateOptions,
) error {
	behavior := destroyTestRuntimeBehavior.Load()
	if behavior != nil && behavior.failCreateNet {
		return errors.New("synthetic network creation failure")
	}

	return nil
}

func (*fakeDestroyRuntime) DeleteNet(context.Context) error { return nil }

func (*fakeDestroyRuntime) DeleteContainer(context.Context, string) error { return nil }

func (*fakeDestroyRuntime) GetContainerStatus(
	context.Context,
	string,
) clabruntime.ContainerStatus {
	return clabruntime.NotFound
}

// writeDestroyTestTopology writes a minimal topology file and returns its path.
func writeDestroyTestTopology(t *testing.T, labName string) string {
	t.Helper()

	topoDir := t.TempDir()
	topo := filepath.Join(topoDir, labName+".clab.yml")

	const definition = `name: %s
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
`

	if err := os.WriteFile(topo, []byte(fmt.Sprintf(definition, labName)), 0o644); err != nil {
		t.Fatal(err)
	}

	return topo
}

// TestDestroyCleanupReportsDestroyFailure reproduces issue 3467: with --cleanup a failed
// destroy must not be reported as success and must not delete the lab directory of a lab
// whose destroy failed.
func TestDestroyCleanupReportsDestroyFailure(t *testing.T) {
	registerDestroyTestRuntime()

	for _, tc := range []struct {
		name              string
		failCreateNet     bool
		listErrAfterFirst bool
	}{
		{name: "makeCopyForDestroy fails", failCreateNet: true},
		{name: "destroy fails after the copy was built", listErrAfterFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topo := writeDestroyTestTopology(t, "ovr")
			labDir := t.TempDir()

			behavior := &destroyTestBehavior{
				containers: []clabruntime.GenericContainer{{
					Names: []string{"clab-ovr-n1"},
					Labels: map[string]string{
						clabconstants.TopoFile:   topo,
						clabconstants.NodeLabDir: labDir,
					},
				}},
				failCreateNet:     tc.failCreateNet,
				listErrAfterFirst: tc.listErrAfterFirst,
			}
			setDestroyTestBehavior(t, behavior)

			c, err := NewContainerLab(
				WithTimeout(time.Minute),
				WithTopoPath(topo, nil),
				WithRuntime("destroytest", &clabruntime.RuntimeConfig{}),
				WithSkippedBindsPathsCheck(),
			)
			if err != nil {
				t.Fatal(err)
			}

			err = c.Destroy(
				context.Background(),
				WithDestroyMaxWorkers(1),
				WithDestroyCleanup(),
			)
			if err == nil {
				t.Fatal("destroy reported success although the destroy failed")
			}

			if _, statErr := os.Stat(labDir); statErr != nil {
				t.Fatalf("lab directory of the failed destroy was removed: %v", statErr)
			}
		})
	}
}

// TestDestroyCleanupRemovesLabDirsOnSuccess checks that --cleanup keeps removing lab
// directories of labs that were destroyed successfully.
func TestDestroyCleanupRemovesLabDirsOnSuccess(t *testing.T) {
	registerDestroyTestRuntime()
	withTestHostsFiles(t, t.TempDir())

	topo := writeDestroyTestTopology(t, "ok")
	labDir := t.TempDir()

	behavior := &destroyTestBehavior{
		containers: []clabruntime.GenericContainer{{
			Names: []string{"clab-ok-n1"},
			Labels: map[string]string{
				clabconstants.TopoFile:   topo,
				clabconstants.NodeLabDir: labDir,
			},
		}},
		listEmptyAfterFirst: true,
	}
	setDestroyTestBehavior(t, behavior)

	c, err := NewContainerLab(
		WithTimeout(time.Minute),
		WithTopoPath(topo, nil),
		WithRuntime("destroytest", &clabruntime.RuntimeConfig{}),
		WithSkippedBindsPathsCheck(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Destroy(
		context.Background(),
		WithDestroyMaxWorkers(1),
		WithDestroyCleanup(),
	); err != nil {
		t.Fatalf("destroy failed: %v", err)
	}

	if _, statErr := os.Stat(labDir); !os.IsNotExist(statErr) {
		t.Fatalf("lab directory of a destroyed lab was not removed: %v", statErr)
	}
}

// TestDestroyLabDirsKeepsFailedLabDirs checks destroyLabDirs directly: only the lab
// directories of successful destroys are removed.
func TestDestroyLabDirsKeepsFailedLabDirs(t *testing.T) {
	t.Parallel()

	removed := t.TempDir()
	kept := t.TempDir()

	c := &CLab{}

	err := c.destroyLabDirs(
		map[string]string{
			"a.clab.yml": removed,
			"b.clab.yml": kept,
		},
		false,
		map[string]bool{kept: true},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(removed); !os.IsNotExist(statErr) {
		t.Fatalf("lab directory of a destroyed lab was not removed: %v", statErr)
	}
	if _, statErr := os.Stat(kept); statErr != nil {
		t.Fatalf("lab directory of the failed destroy was removed: %v", statErr)
	}
}

// mgmtResolveBehavior configures the fake runtime the management network resolution
// tests register under the "mgmtresolvetest" name.
type mgmtResolveBehavior struct {
	// checkable lets the fake implement the MgmtNetworkChecker interface.
	checkable bool
	// networkExists is what MgmtNetworkExists reports when checkable is set.
	networkExists bool
	// failCreateNet makes CreateNet return an error, so a call to it is observable
	// through the test failing instead of silently creating a network.
	failCreateNet bool
}

var mgmtResolveRuntimeBehavior atomic.Pointer[mgmtResolveBehavior]

// mgmtResolveRuntimeStats aggregates the calls every fake instance of one test makes.
type mgmtResolveRuntimeStats struct {
	createNetCalls atomic.Int32
	existsCalls    atomic.Int32
}

var mgmtResolveRuntimeStatsPtr atomic.Pointer[mgmtResolveRuntimeStats]

var registerMgmtResolveRuntimeOnce sync.Once

func registerMgmtResolveRuntime() {
	registerMgmtResolveRuntimeOnce.Do(func() {
		clabruntime.Register("mgmtresolvetest", func() clabruntime.ContainerRuntime {
			behavior := mgmtResolveRuntimeBehavior.Load()
			if behavior != nil && behavior.checkable {
				return &fakeMgmtResolveCheckableRuntime{}
			}

			return &fakeMgmtResolveRuntime{}
		})
	})
}

// setMgmtResolveBehavior installs the behavior for one test and resets the call
// bookkeeping the previous test left behind.
func setMgmtResolveBehavior(t *testing.T, behavior *mgmtResolveBehavior) {
	t.Helper()

	mgmtResolveRuntimeStatsPtr.Store(&mgmtResolveRuntimeStats{})
	mgmtResolveRuntimeBehavior.Store(behavior)

	t.Cleanup(func() {
		mgmtResolveRuntimeBehavior.Store(nil)
	})
}

type fakeMgmtResolveRuntime struct {
	clabruntime.ContainerRuntime
	mgmt   *clabtypes.MgmtNet
	config *clabruntime.RuntimeConfig
}

func (f *fakeMgmtResolveRuntime) Init(opts ...clabruntime.RuntimeOption) error {
	for _, opt := range opts {
		opt(f)
	}

	return nil
}

func (f *fakeMgmtResolveRuntime) WithConfig(cfg *clabruntime.RuntimeConfig) { f.config = cfg }

func (f *fakeMgmtResolveRuntime) WithMgmtNet(mgmt *clabtypes.MgmtNet) { f.mgmt = mgmt }

func (f *fakeMgmtResolveRuntime) Mgmt() *clabtypes.MgmtNet { return f.mgmt }

func (*fakeMgmtResolveRuntime) GetName() string { return "mgmtresolvetest" }

func (f *fakeMgmtResolveRuntime) Config() clabruntime.RuntimeConfig {
	if f.config == nil {
		return clabruntime.RuntimeConfig{}
	}

	return *f.config
}

func (*fakeMgmtResolveRuntime) GetRuntimeSocket() (string, error) { return "", nil }

func (f *fakeMgmtResolveRuntime) CreateNet(
	context.Context,
	...clabruntime.NetworkCreateOptions,
) error {
	stats := mgmtResolveRuntimeStatsPtr.Load()
	if stats != nil {
		stats.createNetCalls.Add(1)
	}

	behavior := mgmtResolveRuntimeBehavior.Load()
	if behavior != nil && behavior.failCreateNet {
		return errors.New("synthetic network creation failure")
	}

	// the real runtimes resolve the bridge of an existing network; mimic it so the
	// test can observe that destroy kept the resolution step
	if f.mgmt != nil && f.mgmt.Bridge == "" {
		f.mgmt.Bridge = "br-fake"
	}

	return nil
}

// fakeMgmtResolveCheckableRuntime is the variant of the fake runtime that implements
// the MgmtNetworkChecker interface.
type fakeMgmtResolveCheckableRuntime struct {
	fakeMgmtResolveRuntime
}

// ForMgmtNet binds a copy of the fake to another management network, so the fake
// supports the multiple management networks of a topology.
func (f *fakeMgmtResolveCheckableRuntime) ForMgmtNet(
	m *clabtypes.MgmtNet,
) clabruntime.ContainerRuntime {
	c := *f
	c.mgmt = m
	return &c
}

func (*fakeMgmtResolveCheckableRuntime) MgmtNetworkExists(context.Context) (bool, error) {
	stats := mgmtResolveRuntimeStatsPtr.Load()
	if stats != nil {
		stats.existsCalls.Add(1)
	}

	behavior := mgmtResolveRuntimeBehavior.Load()
	if behavior == nil {
		return false, nil
	}

	return behavior.networkExists, nil
}

// TestMakeCopyForDestroySkipsMissingMgmtNetworks reproduces issue 3466: destroying a
// lab that was deployed with a management network override must not fail because the
// topology's default network does not exist and its default subnets are taken by the
// overridden network.
func TestMakeCopyForDestroySkipsMissingMgmtNetworks(t *testing.T) {
	registerMgmtResolveRuntime()

	topo := writeDestroyTestTopology(t, "ovr")

	for _, tc := range []struct {
		name          string
		checkable     bool
		networkExists bool
		failCreateNet bool
		wantExists    bool
		wantCreateNet bool
	}{
		{
			name:          "missing network is skipped",
			checkable:     true,
			networkExists: false,
			failCreateNet: true,
			wantExists:    true,
			wantCreateNet: false,
		},
		{
			name:          "existing network is resolved",
			checkable:     true,
			networkExists: true,
			failCreateNet: false,
			wantExists:    true,
			wantCreateNet: true,
		},
		{
			name:          "runtime without an existence check keeps creating",
			checkable:     false,
			failCreateNet: false,
			wantExists:    false,
			wantCreateNet: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setMgmtResolveBehavior(t, &mgmtResolveBehavior{
				checkable:     tc.checkable,
				networkExists: tc.networkExists,
				failCreateNet: tc.failCreateNet,
			})

			c, err := NewContainerLab(
				WithTimeout(time.Minute),
				WithTopoPath(topo, nil),
				WithRuntime("mgmtresolvetest", &clabruntime.RuntimeConfig{}),
				WithSkippedBindsPathsCheck(),
			)
			if err != nil {
				t.Fatal(err)
			}

			cc, err := c.makeCopyForDestroy(
				context.Background(), topo, "", *NewDestroyOptions())
			if err != nil {
				t.Fatalf("makeCopyForDestroy failed: %v", err)
			}

			stats := mgmtResolveRuntimeStatsPtr.Load()
			if got := stats.existsCalls.Load(); got != int32(boolToInt(tc.wantExists)) {
				t.Fatalf("MgmtNetworkExists calls = %d, want %t", got, tc.wantExists)
			}
			if got := stats.createNetCalls.Load(); got != int32(boolToInt(tc.wantCreateNet)) {
				t.Fatalf("CreateNet calls = %d, want %t", got, tc.wantCreateNet)
			}

			if tc.checkable && tc.networkExists && cc.Config.Mgmt.Bridge == "" {
				t.Fatal("existing network did not populate the management bridge name")
			}
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
