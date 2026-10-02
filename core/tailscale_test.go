// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodestailscale "github.com/srl-labs/containerlab/nodes/tailscale"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func writeTailscaleTopo(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "topo.clab.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTailscaleSidecarEligible(t *testing.T) {
	tests := []struct {
		name string
		cfg  *clabtypes.NodeConfig
		want bool
	}{
		{name: "nil"},
		{name: "linux", cfg: &clabtypes.NodeConfig{Kind: "linux"}, want: true},
		{name: "tailscale kind", cfg: &clabtypes.NodeConfig{Kind: clabnodestailscale.KindName}},
		{
			name: "internal",
			cfg: &clabtypes.NodeConfig{
				Kind:   "linux",
				Labels: map[string]string{clabconstants.InternalNode: "true"},
			},
		},
		{
			name: "components",
			cfg: &clabtypes.NodeConfig{
				Kind:       "nokia_srsim",
				Components: []*clabtypes.Component{{Slot: "A"}},
			},
		},
		{name: "host net", cfg: &clabtypes.NodeConfig{Kind: "linux", NetworkMode: "host"}},
		{name: "none net", cfg: &clabtypes.NodeConfig{Kind: "linux", NetworkMode: "none"}},
		{
			name: "shared netns",
			cfg:  &clabtypes.NodeConfig{Kind: "linux", NetworkMode: "container:other"},
		},
		{name: "root ns", cfg: &clabtypes.NodeConfig{Kind: "host", IsRootNamespaceBased: true}},
		{
			name: "ext-container",
			cfg:  &clabtypes.NodeConfig{Kind: "ext-container", SkipUniquenessCheck: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tailscaleSidecarEligible(tc.cfg); got != tc.want {
				t.Fatalf("eligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInjectTailscaleSidecars(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
    n2:
      kind: linux
      image: alpine:3
    host1:
      kind: host
    none1:
      kind: linux
      image: alpine:3
      network-mode: none
    side:
      kind: linux
      image: alpine:3
      network-mode: container:n1
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := c.Nodes["n1-ts"]; !ok {
		t.Fatal("missing n1-ts sidecar")
	}
	if _, ok := c.Nodes["n2-ts"]; !ok {
		t.Fatal("missing n2-ts sidecar")
	}
	for _, name := range []string{"host1-ts", "none1-ts", "side-ts"} {
		if _, ok := c.Nodes[name]; ok {
			t.Fatalf("unexpected sidecar %s", name)
		}
	}

	sc := c.Nodes["n1-ts"].Config()
	if sc.Kind != clabnodestailscale.KindName {
		t.Fatalf("kind = %q", sc.Kind)
	}
	if sc.NetworkMode != "" {
		t.Fatalf("network-mode = %q, want empty", sc.NetworkMode)
	}
	if sc.Stages == nil || sc.Stages.Create == nil {
		t.Fatal("missing create stage")
	}
	wf := sc.Stages.Create.WaitFor
	if len(wf) != 1 || wf[0].Node != "n1" || wf[0].Stage != clabtypes.WaitForCreate {
		t.Fatalf("wait-for = %+v", wf)
	}
	if sc.Image != clabnodestailscale.DefaultImage {
		t.Fatalf("image = %q", sc.Image)
	}
	if sc.Labels[clabconstants.InternalNode] != "true" {
		t.Fatal("sidecar is not labeled internal")
	}
	if sc.Labels[clabnodestailscale.ParentLabel] != "n1" {
		t.Fatalf("root node = %q", sc.Labels[clabnodestailscale.ParentLabel])
	}
	if sc.Env["TS_AUTHKEY"] != "file:/clab/authkey" {
		t.Fatalf("TS_AUTHKEY = %q", sc.Env["TS_AUTHKEY"])
	}
	if sc.Env["TS_HOSTNAME"] != "clab-mylab-n1" {
		t.Fatalf("TS_HOSTNAME = %q", sc.Env["TS_HOSTNAME"])
	}
	if sc.Env["TS_USERSPACE"] != "false" {
		t.Fatalf("TS_USERSPACE = %q", sc.Env["TS_USERSPACE"])
	}
	if _, ok := sc.Env["TS_EXTRA_ARGS"]; ok {
		t.Fatalf("TS_EXTRA_ARGS = %q", sc.Env["TS_EXTRA_ARGS"])
	}
	if _, ok := c.Config.Topology.Nodes["n1-ts"]; !ok {
		t.Fatal("sidecar missing from topology.nodes")
	}
}

func TestInjectTailscaleSidecarsDisabled(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Nodes["n1-ts"]; ok {
		t.Fatal("sidecar injected without mgmt.tailscale")
	}
}

func TestTailscaleSidecarsAreIndependentRuntimeNodes(t *testing.T) {
	c, err := NewContainerLab(WithTopoPath(writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
`), nil))
	if err != nil {
		t.Fatal(err)
	}
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	c.Runtimes = map[string]clabruntime.ContainerRuntime{c.globalRuntimeName: rt}
	var containers []clabruntime.GenericContainer
	for _, node := range c.Nodes {
		cfg := node.Config()
		containers = append(containers, clabruntime.GenericContainer{
			Names: []string{cfg.LongName}, Labels: cfg.Labels,
		})
	}
	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).Return(containers, nil)
	groups, err := c.runtimeNodeGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"n1", "n1-ts"} {
		group := groups[name]
		if group == nil || group.distributed || len(group.containers) != 1 {
			t.Fatalf("runtime group %q = %+v, want independent container", name, group)
		}
	}
}

func TestTailscaleSidecarsIgnoreNodeDefaults(t *testing.T) {
	c, err := NewContainerLab(WithTopoPath(writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  defaults:
    cmd: sleep infinity
    entrypoint: /bin/sh
    network-mode: none
    ports: [8080:80]
    env:
      TS_USERSPACE: "true"
  nodes:
    n1:
      kind: linux
      image: alpine:3
      network-mode: bridge
`), nil))
	if err != nil {
		t.Fatal(err)
	}
	cfg := c.Nodes["n1-ts"].Config()
	if cfg.Cmd != "" || cfg.Entrypoint != "" || cfg.NetworkMode != "" ||
		len(cfg.PortBindings) != 0 || cfg.Env["TS_USERSPACE"] != "false" {
		t.Fatalf("sidecar inherited node defaults: %+v", cfg)
	}
	resolved := c.resolveNodeConfigFromTopology(c.Config.Topology, "n1-ts")
	if resolved.Cmd != "" || resolved.Entrypoint != "" || resolved.NetworkMode != "" ||
		len(resolved.PortSet) != 0 || len(resolved.Env) != 0 {
		t.Fatalf("apply comparison inherited node defaults: %+v", resolved)
	}
}

func TestInjectTailscaleSidecarsNameCollision(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
    n1-ts:
      kind: linux
      image: alpine:3
`)
	_, err := NewContainerLab(WithTopoPath(path, nil))
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("collision error = %v", err)
	}
}

func TestInjectTailscaleSidecarsMissingAuthKey(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale: {}
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Nodes["n1-ts"]; ok {
		t.Fatal("sidecar injected without auth-key or auth-mode")
	}
	if _, ok := c.Nodes["ts"]; ok {
		t.Fatal("proxy injected without auth-mode")
	}
}

const tailscaleSSOTopo = `
name: mylab
mgmt:
  tailscale:
    auth-mode: sso
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
      ports:
        - 8022:22/ts
        - 80:8080
    n2:
      kind: linux
      image: alpine:3
    hostnet:
      kind: linux
      image: alpine:3
      network-mode: host
      ports:
        - 9022:22/ts
`

func TestTailscaleSSONoInjectedNodes(t *testing.T) {
	c, err := NewContainerLab(WithTopoPath(writeTailscaleTopo(t, tailscaleSSOTopo), nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"n1-ts", clabnodestailscale.ProxyName} {
		if _, ok := c.Nodes[name]; ok {
			t.Fatalf("unexpected node %s in SSO mode", name)
		}
	}
	n1 := c.Nodes["n1"].Config()
	if len(n1.TailscalePorts) != 1 || n1.TailscalePorts[0].Listen != 8022 ||
		n1.TailscalePorts[0].Dest != 22 {
		t.Fatalf("n1 tailscale ports = %#v", n1.TailscalePorts)
	}
	if len(n1.PortBindings) == 0 {
		t.Fatal("docker port 80:8080 dropped")
	}
	if err := c.verifyTailscaleProxy(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyTailscaleProxyDuplicatePorts(t *testing.T) {
	for _, mode := range []string{"auth-mode: sso", "auth-key: tskey-auth-test"} {
		path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    `+mode+`
topology:
  defaults:
    ports:
      - 8022:22/ts
  nodes:
    n1:
      kind: linux
      image: alpine:3
    n2:
      kind: linux
      image: alpine:3
`)
		c, err := NewContainerLab(WithTopoPath(path, nil))
		if err != nil {
			t.Fatal(err)
		}
		err = c.verifyTailscaleProxy()
		if sso := mode == "auth-mode: sso"; sso != (err != nil) {
			t.Fatalf("%s: err = %v", mode, err)
		}
	}
}

func TestVerifyTailscaleProxyReservedName(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-mode: sso
topology:
  nodes:
    ts:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.verifyTailscaleProxy(); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved name error = %v", err)
	}
}

func TestInjectTailscaleKeyModeIgnoresTSPorts(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
      ports:
        - 8022:22/ts
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Nodes["ts"]; ok {
		t.Fatal("proxy injected in key mode")
	}
	if _, ok := c.Nodes["n1-ts"]; !ok {
		t.Fatal("missing n1-ts sidecar")
	}
}

func TestInjectTailscaleSidecarsNodeFilter(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
    n2:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil), WithNodeFilter([]string{"n1"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Nodes["n1-ts"]; !ok {
		t.Fatal("filtered lab missing n1-ts")
	}
	if _, ok := c.Nodes["n2-ts"]; ok {
		t.Fatal("sidecar injected for filtered-out node")
	}
	if _, ok := c.Nodes["n2"]; ok {
		t.Fatal("n2 should have been filtered out")
	}
}

func newTailscaleToolLab(
	t *testing.T,
	topo string,
	opts ...ClabOption,
) (*CLab, *clabmocksmockruntime.MockContainerRuntime) {
	t.Helper()
	c, err := NewContainerLab(
		append([]ClabOption{WithTopoPath(writeTailscaleTopo(t, topo), nil)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	c.Runtimes = map[string]clabruntime.ContainerRuntime{c.globalRuntimeName: rt}
	c.Config.Mgmt.IPAM.Provider = clabtypes.IPAMProviderRuntime
	c.Nodes["n1"].Config().MgmtIPv4Address = "172.20.20.2"

	return c, rt
}

func filterMatch(filters []*clabtypes.GenericFilter, field string) string {
	for _, f := range filters {
		if f.Field == field {
			return f.Match
		}
	}

	return ""
}

func readServeConfig(t *testing.T, c *CLab) string {
	t.Helper()
	b, err := os.ReadFile(
		filepath.Join(c.TopoPaths.NodeDir(clabnodestailscale.ProxyName), "serve.json"),
	)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestSyncTailscaleProxyCreates(t *testing.T) {
	c, rt := newTailscaleToolLab(t, tailscaleSSOTopo)
	ctx := context.Background()

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, f []*clabtypes.GenericFilter) ([]clabruntime.GenericContainer, error) {
			if got := filterMatch(f, clabconstants.ToolType); got != clabnodestailscale.ToolType {
				t.Fatalf("tool-type filter = %q", got)
			}
			return nil, nil
		})
	rt.EXPECT().
		PullImage(gomock.Any(), clabnodestailscale.DefaultImage, clabtypes.PullPolicyIfNotPresent)
	var created *clabtypes.NodeConfig
	rt.EXPECT().CreateContainer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *clabtypes.NodeConfig) (string, error) {
			created = cfg
			return "id", nil
		})
	rt.EXPECT().StartContainer(gomock.Any(), "id", gomock.Any())

	if err := c.syncTailscaleProxy(ctx); err != nil {
		t.Fatal(err)
	}

	if created.LongName != "clab-mylab-ts" {
		t.Fatalf("container name = %q", created.LongName)
	}
	if created.Labels[clabconstants.ToolType] != clabnodestailscale.ToolType ||
		created.Labels[clabconstants.InternalNode] != "true" ||
		created.Labels[clabconstants.Containerlab] != "mylab" {
		t.Fatalf("labels = %v", created.Labels)
	}
	if created.Env["TS_USERSPACE"] != "true" || created.Env["TS_HOSTNAME"] != "clab-mylab" {
		t.Fatalf("env = %v", created.Env)
	}
	if want := `{"TCP":{"8022":{"TCPForward":"172.20.20.2:22"}}}`; readServeConfig(t, c) != want {
		t.Fatalf("serve.json = %s, want %s", readServeConfig(t, c), want)
	}
}

func TestSyncTailscaleProxyExisting(t *testing.T) {
	c, rt := newTailscaleToolLab(t, tailscaleSSOTopo)

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return([]clabruntime.GenericContainer{{Names: []string{"clab-mylab-ts"}}}, nil)

	if err := c.syncTailscaleProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readServeConfig(t, c), "8022") {
		t.Fatalf("serve.json = %s", readServeConfig(t, c))
	}
}

func TestSyncTailscaleProxyRemovesWhenDisabled(t *testing.T) {
	c, rt := newTailscaleToolLab(t, tailscaleSSOTopo)
	c.Config.Mgmt.Tailscale = nil
	labDir := c.TopoPaths.NodeDir(clabnodestailscale.ProxyName)
	if err := os.MkdirAll(labDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(labDir, clabnodestailscale.ServeFile)
	if err := os.WriteFile(marker, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return([]clabruntime.GenericContainer{{
			Names:  []string{"/clab-mylab-ts"},
			Labels: map[string]string{clabconstants.ToolType: clabnodestailscale.ToolType},
		}}, nil)
	rt.EXPECT().Exec(gomock.Any(), "clab-mylab-ts", gomock.Any()).
		Return(&clabexec.ExecResult{}, nil)
	rt.EXPECT().DeleteContainer(gomock.Any(), "clab-mylab-ts")

	if err := c.syncTailscaleProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("serve.json not removed: %v", err)
	}

	// without the marker there is no proxy to look up.
	if err := c.syncTailscaleProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteApplyNodesRunsPreDestroy(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	c := &CLab{
		Nodes:             map[string]clabnodes.Node{"n1-ts": sidecar},
		Runtimes:          map[string]clabruntime.ContainerRuntime{"docker": rt},
		globalRuntimeName: "docker",
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{
		"n1-ts": {name: "n1-ts", containers: []clabruntime.GenericContainer{{
			Names: []string{"clab-mylab-n1-ts"},
		}}},
		"gone": {name: "gone", containers: []clabruntime.GenericContainer{{
			Names: []string{"clab-mylab-gone"},
		}}},
	}, nil)
	plan.recreatedNodeSet["n1-ts"] = struct{}{}
	plan.deletedNodeSet["gone"] = struct{}{}

	gomock.InOrder(
		sidecar.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.Running),
		sidecar.EXPECT().PreDestroy(gomock.Any()),
		rt.EXPECT().DeleteContainer(gomock.Any(), "clab-mylab-gone"),
		rt.EXPECT().DeleteContainer(gomock.Any(), "clab-mylab-n1-ts"),
	)

	if err := c.deleteApplyNodes(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteApplyNodesLogsOutRemovedSidecar(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	otherRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	c := &CLab{
		Nodes:             map[string]clabnodes.Node{},
		Runtimes:          map[string]clabruntime.ContainerRuntime{"docker": rt},
		globalRuntimeName: "docker",
	}
	plan := newApplyPlan(map[string]*runtimeNodeGroup{
		"gone-ts": {containers: []clabruntime.GenericContainer{{
			Names:   []string{"clab-mylab-gone-ts"},
			Labels:  map[string]string{clabconstants.NodeKind: clabnodestailscale.KindName},
			Runtime: otherRuntime,
		}}},
	}, nil)
	plan.deletedNodeSet["gone-ts"] = struct{}{}
	gomock.InOrder(
		otherRuntime.EXPECT().Exec(gomock.Any(), "clab-mylab-gone-ts", gomock.Any()).
			Return(&clabexec.ExecResult{}, nil),
		otherRuntime.EXPECT().DeleteContainer(gomock.Any(), "clab-mylab-gone-ts"),
	)
	if err := c.deleteApplyNodes(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverTailscaleContainersWithoutAuthKey(t *testing.T) {
	for _, filter := range [][]string{nil, {"n1"}} {
		ctrl := gomock.NewController(t)
		rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		c := &CLab{
			Config:   &Config{Name: "mylab"},
			Runtimes: map[string]clabruntime.ContainerRuntime{"docker": rt},
		}
		containers := []clabruntime.GenericContainer{{
			Names:  []string{"clab-mylab-n1"},
			Labels: map[string]string{clabconstants.NodeKind: "linux"},
		}, {
			Names:  []string{"clab-mylab-ts"},
			Labels: map[string]string{clabconstants.ToolType: clabnodestailscale.ToolType},
		}}
		for _, parent := range []string{"n1", "n2"} {
			containers = append(containers, clabruntime.GenericContainer{
				Names: []string{"clab-mylab-" + parent + "-ts"},
				Labels: map[string]string{
					clabconstants.NodeKind:         clabnodestailscale.KindName,
					clabconstants.InternalNode:     "true",
					clabnodestailscale.ParentLabel: parent,
				},
			})
		}
		rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, filters []*clabtypes.GenericFilter) (
				[]clabruntime.GenericContainer, error,
			) {
				if filterMatch(filters, clabconstants.Containerlab) != "mylab" {
					t.Fatal("discovery must be scoped to the lab")
				}
				return containers, nil
			})
		found, err := c.discoverTailscaleContainers(context.Background(), filter)
		if err != nil {
			t.Fatal(err)
		}
		if filter == nil && len(found) != 3 {
			t.Fatalf("found %d Tailscale containers, want 3", len(found))
		}
		if filter != nil && (len(found) != 1 || found[0].Names[0] != "clab-mylab-n1-ts") {
			t.Fatalf("filtered discovery = %+v", found)
		}
	}
}

func TestDeleteContainersDirectLogsOutTailscale(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	c := &CLab{}
	for _, labels := range []map[string]string{
		{clabconstants.NodeKind: clabnodestailscale.KindName},
		{clabconstants.ToolType: clabnodestailscale.ToolType},
	} {
		gomock.InOrder(
			rt.EXPECT().Exec(gomock.Any(), "clab-mylab-ts", gomock.Any()).
				Return(&clabexec.ExecResult{}, nil),
			rt.EXPECT().DeleteContainer(gomock.Any(), "clab-mylab-ts"),
		)
		c.deleteContainersDirect(context.Background(), []clabruntime.GenericContainer{{
			Names: []string{"clab-mylab-ts"}, Labels: labels, Runtime: rt,
		}})
	}
}

func TestPreDestroyNodesSkipsMissingAndIgnoresErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	running := clabmocksmocknodes.NewMockNode(ctrl)
	missing := clabmocksmocknodes.NewMockNode(ctrl)

	running.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.Running)
	running.EXPECT().PreDestroy(gomock.Any()).Return(errors.New("boom"))
	running.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "running"})
	missing.EXPECT().GetContainerStatus(gomock.Any()).Return(clabruntime.NotFound)

	(&CLab{}).preDestroyNodes(context.Background(), []clabnodes.Node{running, missing}, 1)
}

func TestDeleteToolContainersKeepsTailscaleOnFilter(t *testing.T) {
	for name, filter := range map[string][]string{"full": nil, "filtered": {"n1"}} {
		t.Run(name, func(t *testing.T) {
			var opts []ClabOption
			if filter != nil {
				opts = append(opts, WithNodeFilter(filter))
			}
			c, rt := newTailscaleToolLab(t, tailscaleSSOTopo, opts...)

			var tools []string
			rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, f []*clabtypes.GenericFilter) (
					[]clabruntime.GenericContainer, error,
				) {
					tools = append(tools, filterMatch(f, clabconstants.ToolType))
					return nil, nil
				}).
				AnyTimes()

			c.deleteToolContainers(context.Background())

			if got := slices.Contains(tools, clabnodestailscale.ToolType); got != (filter == nil) {
				t.Fatalf("tailscale listed for deletion = %v, tools = %v", got, tools)
			}
		})
	}
}

func TestPlanTailscaleSidecarRecreates(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
    n2:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}

	plan := newApplyPlan(map[string]*runtimeNodeGroup{}, nil)
	plan.recreatedNodeSet["n1"] = struct{}{}
	plan.restartNodeSet["n1-ts"] = struct{}{}
	c.planTailscaleSidecarRecreates(plan)

	if _, ok := plan.recreatedNodeSet["n1-ts"]; !ok {
		t.Fatal("n1-ts not recreated with its parent")
	}
	if _, ok := plan.restartNodeSet["n1-ts"]; ok {
		t.Fatal("n1-ts still planned for restart")
	}
	if _, ok := plan.recreatedNodeSet["n2-ts"]; ok {
		t.Fatal("n2-ts recreated without its parent")
	}
}

func TestInjectTailscaleSidecarsSSHConfigSkipsInternal(t *testing.T) {
	path := writeTailscaleTopo(t, `
name: mylab
mgmt:
  tailscale:
    auth-key: tskey-auth-test
topology:
  nodes:
    n1:
      kind: linux
      image: alpine:3
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &SSHConfigTmpl{Nodes: make([]SSHConfigNodeTmpl, 0, len(c.Nodes))}
	for _, n := range c.Nodes {
		if isInternalNode(n) {
			continue
		}
		tmpl.Nodes = append(tmpl.Nodes, SSHConfigNodeTmpl{Names: []string{n.Config().LongName}})
	}
	if len(tmpl.Nodes) != 1 {
		t.Fatalf("ssh nodes = %d, want 1", len(tmpl.Nodes))
	}
}
