// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestInitMacvlanManagementNetwork(t *testing.T) {
	for _, subnet := range []string{"", "192.0.2.0/24"} {
		c := &CLab{Config: &Config{Mgmt: &clabtypes.MgmtNet{
			Driver: "macvlan", MacvlanParent: "eth0", IPv4Subnet: subnet,
		}}}
		err := c.initMgmtNetwork()
		if err != nil {
			t.Fatalf("initMgmtNetwork(%q) error = %v", subnet, err)
		}
		if c.Config.Mgmt.IPv4Subnet != subnet || c.Config.Mgmt.IPv6Subnet != "" {
			t.Fatalf("bridge subnet defaults applied to macvlan: %+v", c.Config.Mgmt)
		}
	}
}

// Brief-notation mgmt-net links are converted to LinkTypeMgmtNet during YAML
// unmarshalling, so the macvlan guard must reject them.
func TestInitMgmtNetworkRejectsMgmtNetLinkWithMacvlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topo.clab.yml")
	err := os.WriteFile(path, []byte(`
name: mgmtnet-brief
mgmt:
  network: mgmtnet-brief-mgmt
  driver: macvlan
  macvlan-parent: eth0
topology:
  nodes:
    n1:
      kind: linux
      image: alpine
  links:
    - endpoints: ["mgmt-net:eth1", "n1:eth0"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewContainerLab(WithTopoPath(path, nil))
	if err == nil ||
		!strings.Contains(err.Error(), "mgmt-net links require a bridge management network") {
		t.Fatalf("macvlan guard accepted brief mgmt-net link: %v", err)
	}
}

func TestCreateNetworkPassesStaticManagementAddresses(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	m := &clabtypes.MgmtNet{
		IPAM: clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderRuntime},
	}
	cfg := &clabtypes.NodeConfig{
		MgmtIPv4Address: "192.0.2.10",
		MgmtIPv6Address: "2001:db8::10",
		Labels:          map[string]string{},
	}
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	c := &CLab{
		Config:            &Config{Mgmt: m},
		globalRuntimeName: "test",
		Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
		Nodes:             map[string]clabnodes.Node{"node": node},
	}
	rt.EXPECT().CreateNet(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, options ...clabruntime.NetworkCreateOptions) error {
			want := []netip.Addr{
				netip.MustParseAddr(cfg.MgmtIPv4Address),
				netip.MustParseAddr(cfg.MgmtIPv6Address),
			}
			if len(options) != 1 || !slices.Equal(options[0].StaticAddresses, want) {
				t.Fatalf("static addresses = %v, want %v", options, want)
			}
			return nil
		},
	)
	rt.EXPECT().Mgmt().Return(m)
	if err := c.CreateNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSyncMgmtHostRoutesReturnsRuntimeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	failure := errors.New("host route synchronization failed")
	c := &CLab{
		Config:            &Config{Mgmt: &clabtypes.MgmtNet{Driver: clabtypes.MgmtDriverMacvlan}},
		globalRuntimeName: "test",
		Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
	}
	rt.EXPECT().SyncMgmtHostRoutes(gomock.Any()).Return(failure)
	if err := c.SyncMgmtHostRoutes(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("runtime error lost: %v", err)
	}
}

func TestSkipMgmtNetwork(t *testing.T) {
	withNodes := func(nodes map[string]*clabtypes.NodeDefinition) *clabtypes.Topology {
		topo := clabtypes.NewTopology()
		for n, d := range nodes {
			topo.Nodes[n] = d
		}

		return topo
	}

	tests := []struct {
		name string
		topo *clabtypes.Topology
		mgmt clabtypes.MgmtNet
		want bool
	}{
		{
			name: "every node explicitly none",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "none"},
			}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "none inherited from defaults",
			topo: func() *clabtypes.Topology {
				topo := withNodes(map[string]*clabtypes.NodeDefinition{"n1": {}, "n2": {}})
				topo.Defaults.NetworkMode = "none"
				return topo
			}(),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "none inherited from kind",
			topo: func() *clabtypes.Topology {
				topo := withNodes(map[string]*clabtypes.NodeDefinition{
					"n1": {Kind: "linux"},
					"n2": {Kind: "linux"},
				})
				topo.Kinds["linux"] = &clabtypes.NodeDefinition{NetworkMode: "none"}
				return topo
			}(),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: true,
		},
		{
			name: "flag unset preserves old behavior even when all-none",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "none"},
			}),
			want: false,
		},
		{
			name: "mixed: one node uses mgmt",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{
				"n1": {NetworkMode: "none"},
				"n2": {NetworkMode: "container:foo"},
			}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
		{
			name: "no NetworkMode anywhere (default mgmt attachment)",
			topo: withNodes(map[string]*clabtypes.NodeDefinition{"n1": {}}),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
		{
			name: "empty topology is not 'unused'",
			topo: withNodes(nil),
			mgmt: clabtypes.MgmtNet{SkipWhenUnused: true},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &CLab{
				Config: &Config{
					Mgmt:     &tc.mgmt,
					Topology: tc.topo,
				},
			}

			if got := c.skipMgmtNetwork(); got != tc.want {
				t.Errorf("skipMgmtNetwork() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseTopologyDoesNotAllocateManagementIPs(t *testing.T) {
	c, err := NewContainerLab(WithTopoPath("test_data/topo12.yml", nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range c.Nodes {
		if node.Config().MgmtIPv4Address != "" || node.Config().MgmtIPv6Address != "" {
			t.Fatalf("parsing allocated an address for %s", node.Config().ShortName)
		}
	}
}

func TestPrepareManagementNetworkAllocatesAfterResolution(t *testing.T) {
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	m := &clabtypes.MgmtNet{IPv4Subnet: "auto"}
	cfg := &clabtypes.NodeConfig{ShortName: "node", Labels: map[string]string{}}
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	c := &CLab{
		Config:            &Config{Mgmt: m},
		globalRuntimeName: "test",
		Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
		Nodes:             map[string]clabnodes.Node{"node": node},
	}
	rt.EXPECT().
		CreateNet(gomock.Any()).
		DoAndReturn(func(context.Context, ...clabruntime.NetworkCreateOptions) error {
			if cfg.MgmtIPv4Address != "" {
				t.Fatal("allocated before resolving network")
			}
			m.IPv4Subnet = "192.0.2.0/29"
			m.IPv4Gw = "192.0.2.6"
			return nil
		})
	rt.EXPECT().Mgmt().Return(m)
	rt.EXPECT().NetworkAddresses(gomock.Any(), gomock.Any()).Return(nil, nil)
	if _, err := c.prepareLabManagementNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	ip, err := netip.ParseAddr(cfg.MgmtIPv4Address)
	if err != nil || !netip.MustParsePrefix(m.IPv4Subnet).Contains(ip) ||
		cfg.MgmtIPv4Address == m.IPv4Gw {
		t.Fatalf("invalid resolved allocation: %s", cfg.MgmtIPv4Address)
	}
	if cfg.MgmtIPv4PrefixLength != 29 || cfg.MgmtIPv4Gateway != m.IPv4Gw {
		t.Fatalf("missing resolved management IP configuration: %+v", cfg)
	}
}

func TestPrepareManagementNetworkDelegatesRuntimeIPAM(t *testing.T) {
	for _, address := range []string{"", "192.0.2.123"} {
		t.Run(address, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
			m := &clabtypes.MgmtNet{
				IPv4Subnet: "auto",
				IPAM:       clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderRuntime},
			}
			cfg := &clabtypes.NodeConfig{
				ShortName:       "node",
				MgmtIPv4Address: address,
				Labels:          map[string]string{},
			}
			node := clabmocksmocknodes.NewMockNode(ctrl)
			node.EXPECT().Config().Return(cfg).AnyTimes()
			c := &CLab{
				Config: &Config{Mgmt: m}, globalRuntimeName: "test",
				Runtimes: map[string]clabruntime.ContainerRuntime{"test": rt},
				Nodes:    map[string]clabnodes.Node{"node": node},
			}
			if address != "" {
				rt.EXPECT().CreateNet(gomock.Any(), gomock.Any()).Return(nil)
			} else {
				rt.EXPECT().CreateNet(gomock.Any()).Return(nil)
			}
			rt.EXPECT().Mgmt().Return(m)
			if _, err := c.prepareLabManagementNetwork(context.Background()); err != nil {
				t.Fatal(err)
			}
			if cfg.MgmtIPv4Address != address || cfg.MgmtIPv6Address != "" {
				t.Fatalf("runtime addresses changed: %+v", cfg)
			}
		})
	}
}

func TestPrepareManagementNetworkLoadsPreferredAddress(t *testing.T) {
	paths := &clabtypes.TopoPaths{}
	if err := paths.SetLabDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		paths.StateFile(),
		[]byte("ipam:\n  node:\n    ipv4: 192.0.2.5\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	ctrl := gomock.NewController(t)
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	dad := false
	m := &clabtypes.MgmtNet{
		IPv4Subnet: "192.0.2.0/29",
		IPAM:       clabtypes.MgmtIPAM{DAD: &dad},
	}
	cfg := &clabtypes.NodeConfig{ShortName: "node", Labels: map[string]string{}}
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	c := &CLab{
		TopoPaths: paths,
		Config:    &Config{Mgmt: m}, globalRuntimeName: "test",
		Runtimes: map[string]clabruntime.ContainerRuntime{"test": rt},
		Nodes:    map[string]clabnodes.Node{"node": node},
	}
	rt.EXPECT().CreateNet(gomock.Any()).Return(nil)
	rt.EXPECT().Mgmt().Return(m)
	rt.EXPECT().NetworkAddresses(gomock.Any(), gomock.Any()).Return(nil, nil)
	if _, err := c.prepareLabManagementNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg.MgmtIPv4Address != "192.0.2.5" {
		t.Fatalf("preferred address was not reused: %s", cfg.MgmtIPv4Address)
	}
}

func TestPrepareManagementNetworkRuntimeReservations(t *testing.T) {
	for _, scenario := range []string{"foreign-preferred", "own-live", "foreign-conflict", "inspection-error"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
			m := &clabtypes.MgmtNet{
				Network:    "mgmt",
				IPv4Subnet: "192.0.2.0/29",
				IPAM:       clabtypes.MgmtIPAM{DAD: new(false)},
			}
			cfg := &clabtypes.NodeConfig{ShortName: "node", Labels: map[string]string{}}
			node := clabmocksmocknodes.NewMockNode(ctrl)
			node.EXPECT().Config().Return(cfg).AnyTimes()
			paths := &clabtypes.TopoPaths{}
			if err := paths.SetLabDir(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				paths.StateFile(),
				[]byte("ipam:\n  node:\n    ipv4: 192.0.2.5\n"),
				0644,
			); err != nil {
				t.Fatal(err)
			}
			c := &CLab{
				TopoPaths:         paths,
				Config:            &Config{Mgmt: m},
				globalRuntimeName: "test",
				Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
				Nodes:             map[string]clabnodes.Node{"node": node},
			}
			ip := netip.MustParseAddr("192.0.2.5")
			occupied := []clabruntime.NetworkAddress{
				{NetworkName: "mgmt", ContainerID: "foreign-id", Address: ip},
			}
			var existing []clabtypes.ExistingAddress
			if scenario == "own-live" || scenario == "foreign-conflict" {
				existing = []clabtypes.ExistingAddress{
					{NodeName: "node", ContainerID: "own-id", Address: ip},
				}
			}
			if scenario == "own-live" {
				occupied[0].ContainerID = "own-id"
			}
			var snapshotErr error
			if scenario == "inspection-error" {
				snapshotErr = errors.New("cannot inspect network")
			}
			rt.EXPECT().CreateNet(gomock.Any()).Return(nil)
			rt.EXPECT().Mgmt().Return(m)
			rt.EXPECT().
				NetworkAddresses(gomock.Any(), []netip.Prefix{netip.MustParsePrefix(m.IPv4Subnet)}).
				Return(occupied, snapshotErr)
			_, err := c.prepareLabManagementNetwork(context.Background(), existing...)
			if scenario == "foreign-conflict" || scenario == "inspection-error" {
				if err == nil || cfg.MgmtIPv4Address != "" {
					t.Fatalf("conflict/error accepted or committed: %v %+v", err, cfg)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "own-live" && cfg.MgmtIPv4Address != ip.String() {
				t.Fatal("own live allocation lost")
			}
			if scenario == "foreign-preferred" &&
				(cfg.MgmtIPv4Address == "" || cfg.MgmtIPv4Address == ip.String()) {
				t.Fatal("foreign runtime reservation ignored")
			}
		})
	}
}

func writeTopo(t *testing.T, topo string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "topo.clab.yml")
	if err := os.WriteFile(path, []byte(topo), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMgmtNetworkList(t *testing.T) {
	path := writeTopo(t, `
name: multi
mgmt:
  - network: multi-main
    ipv4-subnet: 192.0.2.0/24
  - network: multi-oob
    driver: macvlan
    macvlan-parent: eth0
    ipv4-subnet: 198.51.100.0/24
  - network: multi-auto
    ipam:
      provider: runtime
topology:
  kinds:
    linux:
      mgmt-net: multi-oob
  nodes:
    n1:
      kind: linux
      image: alpine
    n2:
      kind: linux
      image: alpine
      mgmt-net: multi-main
    n3:
      kind: linux
      image: alpine
      mgmt-net: multi-auto
    n4:
      kind: linux
      image: alpine
      network-mode: host
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Config.Mgmt != c.Config.MgmtNetworks[0] || c.Config.Mgmt.Network != "multi-main" {
		t.Fatalf("default network not taken from first entry: %+v", c.Config.Mgmt)
	}
	if auto := c.Config.MgmtNetworks[2]; auto.IPv4Subnet != "" || auto.IPv6Subnet != "" {
		t.Fatalf("default subnets applied to extra network: %+v", auto)
	}

	for name, want := range map[string]string{
		"n1": "multi-oob",
		"n2": "multi-main",
		"n3": "multi-auto",
		"n4": "",
	} {
		if got := c.Nodes[name].Config().MgmtNet; got != want {
			t.Fatalf("node %s mgmt-net = %q; want %q", name, got, want)
		}
	}
}

func TestMgmtNetworkListErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		mgmt string
		want string
	}{
		{
			name: "missing network",
			mgmt: "  - network: a\n  - ipv4-subnet: 192.0.2.0/24\n",
			want: "entry 1 requires a network name",
		},
		{
			name: "duplicate network",
			mgmt: "  - network: x\n  - network: x\n",
			want: `management network "x" is defined more than once`,
		},
		{
			name: "duplicate bridge",
			mgmt: "  - network: a\n    bridge: br0\n  - network: b\n    bridge: br0\n",
			want: `management networks "a" and "b" use the same bridge "br0"`,
		},
		{
			name: "tailscale on two networks",
			mgmt: "  - network: a\n    tailscale:\n      auth-mode: sso\n" +
				"  - network: b\n    tailscale:\n      auth-mode: sso\n",
			want: `tailscale is set on management networks "a" and "b", only one is allowed`,
		},
		{
			name: "invalid entry",
			mgmt: "  - network: a\n  - network: b\n    driver: macvlan\n",
			want: "macvlan",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTopo(t, "name: bad\nmgmt:\n"+tc.mgmt+"topology:\n  nodes: {}\n")
			_, err := NewContainerLab(WithTopoPath(path, nil))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want %q", err, tc.want)
			}
		})
	}
}

func TestNodeUnknownMgmtNetwork(t *testing.T) {
	path := writeTopo(t, `
name: unknown
topology:
  nodes:
    n1:
      kind: linux
      image: alpine
      mgmt-net: oob
`)
	_, err := NewContainerLab(WithTopoPath(path, nil))
	if err == nil || !strings.Contains(err.Error(), `management network "oob" is not defined`) {
		t.Fatalf("error = %v; want unknown management network", err)
	}
}

func TestMgmtNetworkListRequiresNodeSelection(t *testing.T) {
	topo := `
name: required
mgmt:
  - network: main
  - network: oob
topology:
  nodes:
    br:
      kind: bridge
    hostnode:
      kind: linux
      image: alpine
      network-mode: host
    n1:
      kind: linux
      image: alpine
      mgmt-net: oob
%s`
	if _, err := NewContainerLab(
		WithTopoPath(writeTopo(t, fmt.Sprintf(topo, "")), nil),
	); err != nil {
		t.Fatalf("nodes outside the management network must not require mgmt-net: %v", err)
	}

	missing := "    n2:\n      kind: linux\n      image: alpine\n"
	_, err := NewContainerLab(WithTopoPath(writeTopo(t, fmt.Sprintf(topo, missing)), nil))
	if err == nil || !strings.Contains(err.Error(), `node "n2" must set mgmt-net`) {
		t.Fatalf("error = %v; want missing mgmt-net", err)
	}
}

// Runtimes without MgmtNetBinder, such as podman, only support the default network.
func TestMgmtNetworkListUnsupportedRuntime(t *testing.T) {
	c, err := NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}
	c.Config.MgmtNetworks = clabtypes.MgmtNetworks{
		c.Config.Mgmt, {Network: "oob"},
	}
	c.Runtimes["podman"] = clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))

	err = c.initNode(&clabtypes.NodeConfig{
		ShortName: "n1", Kind: "linux", MgmtNet: "oob", Labels: map[string]string{},
	}, "podman")
	if err == nil ||
		!strings.Contains(
			err.Error(),
			`runtime "podman" does not support multiple management networks`,
		) {
		t.Fatalf("error = %v; want unsupported runtime", err)
	}
}

func TestSingleEntryMgmtNetworkListDefaultsNodes(t *testing.T) {
	path := writeTopo(t, `
name: single-list
mgmt:
  - network: single-list-main
topology:
  nodes:
    n1:
      kind: linux
      image: alpine
`)
	c, err := NewContainerLab(WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Nodes["n1"].Config().MgmtNet; got != "single-list-main" {
		t.Fatalf("mgmt-net = %q; want single-list-main", got)
	}
}

// Each network is created through a runtime bound to it and containerlab IPAM allocates from the
// network the node is attached to.
func TestPrepareManagementNetworkPerNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	noDAD := false
	clabIPAM := clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab, DAD: &noDAD}
	main := &clabtypes.MgmtNet{
		Network: "main", Bridge: "br-main", IPv4Subnet: "192.0.2.0/29", IPAM: clabIPAM,
	}
	auto := &clabtypes.MgmtNet{
		Network: "auto", Bridge: "br-auto",
		IPAM: clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderRuntime},
	}
	mv := &clabtypes.MgmtNet{
		Network: "mv", Driver: clabtypes.MgmtDriverMacvlan, MacvlanParent: "eth0",
		IPv4Subnet: "198.51.100.0/29", IPAM: clabIPAM,
	}
	spare := &clabtypes.MgmtNet{Network: "spare", IPAM: clabIPAM}

	base := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	binder := clabmocksmockruntime.NewMockMgmtNetBinder(ctrl)
	rt := struct {
		*clabmocksmockruntime.MockContainerRuntime
		*clabmocksmockruntime.MockMgmtNetBinder
	}{base, binder}
	autoRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	mvRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	spareRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	binder.EXPECT().ForMgmtNet(auto).Return(autoRt).AnyTimes()
	binder.EXPECT().ForMgmtNet(mv).Return(mvRt).AnyTimes()
	binder.EXPECT().ForMgmtNet(spare).Return(spareRt).AnyTimes()
	for r := range map[*clabmocksmockruntime.MockContainerRuntime]bool{
		base: true, autoRt: true, mvRt: true, spareRt: true,
	} {
		r.EXPECT().CreateNet(gomock.Any()).Return(nil)
	}
	base.EXPECT().Mgmt().Return(main).AnyTimes()
	// reservations are looked up across the whole runtime for main and mv, not for auto
	base.EXPECT().NetworkAddresses(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)

	configs := map[string]*clabtypes.NodeConfig{
		"r1": {ShortName: "r1", MgmtNet: "main"},
		"r2": {ShortName: "r2", MgmtNet: "auto"},
		"r3": {ShortName: "r3", MgmtNet: "mv"},
		"h1": {ShortName: "h1", NetworkMode: "host"},
	}
	nodes := make(map[string]clabnodes.Node, len(configs))
	for name, cfg := range configs {
		cfg.Labels = map[string]string{}
		node := clabmocksmocknodes.NewMockNode(ctrl)
		node.EXPECT().Config().Return(cfg).AnyTimes()
		nodes[name] = node
	}
	c := &CLab{
		Config: &Config{
			Mgmt:         main,
			MgmtNetworks: clabtypes.MgmtNetworks{main, auto, mv, spare},
		},
		globalRuntimeName: "test",
		Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
		Nodes:             nodes,
	}

	if _, err := c.prepareLabManagementNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]struct{ subnet, bridge string }{
		"r1": {"192.0.2.0/29", "br-main"},
		"r2": {"", "br-auto"},
		"r3": {"198.51.100.0/29", ""},
		"h1": {"", "br-main"},
	} {
		cfg := configs[name]
		if want.subnet == "" {
			if cfg.MgmtIPv4Address != "" {
				t.Fatalf("%s: unexpected containerlab allocation %s", name, cfg.MgmtIPv4Address)
			}
		} else {
			ip, err := netip.ParseAddr(cfg.MgmtIPv4Address)
			if err != nil || !netip.MustParsePrefix(want.subnet).Contains(ip) {
				t.Fatalf("%s: address %q outside %s", name, cfg.MgmtIPv4Address, want.subnet)
			}
		}
		if got := cfg.Labels[clabconstants.NodeMgmtNetBr]; got != want.bridge {
			t.Fatalf("%s: bridge label = %q; want %q", name, got, want.bridge)
		}
	}
}

func TestMgmtOverrideFlagsWithMultipleNetworks(t *testing.T) {
	multi := writeTopo(t, `
name: override
mgmt:
  - network: main
  - network: oob
topology:
  nodes: {}
`)
	for name, opt := range map[string]ClabOption{
		"network": WithManagementNetworkName("other"),
		"ipv4":    WithManagementIpv4Subnet("192.0.2.0/24"),
		"ipv6":    WithManagementIpv6Subnet("2001:db8::/64"),
	} {
		_, err := NewContainerLab(WithTopoPath(multi, nil), opt)
		if err == nil ||
			!strings.Contains(err.Error(), "cannot be used with multiple management networks") {
			t.Fatalf("%s: error = %v; want override rejected", name, err)
		}
	}

	single := writeTopo(t, "name: override\ntopology:\n  nodes: {}\n")
	c, err := NewContainerLab(WithTopoPath(single, nil), WithManagementNetworkName("other"))
	if err != nil || c.Config.Mgmt.Network != "other" {
		t.Fatalf("single network override: err = %v, network = %q", err, c.Config.Mgmt.Network)
	}
}

func TestCreateNetworkFillsResolvedSubnetEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := &clabtypes.MgmtNet{Network: "main"}
	cfg := &clabtypes.NodeConfig{
		ShortName: "vm",
		MgmtNet:   "main",
		Labels:    map[string]string{},
		Env:       map[string]string{"DOCKER_NET_V4_ADDR": "", "DOCKER_NET_V6_ADDR": "fd00::/64"},
	}
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	rt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	rt.EXPECT().CreateNet(gomock.Any()).DoAndReturn(
		func(context.Context, ...clabruntime.NetworkCreateOptions) error {
			m.IPv4Subnet, m.IPv6Subnet = "192.0.2.0/24", "2001:db8::/64"
			return nil
		})
	rt.EXPECT().Mgmt().Return(m).AnyTimes()
	c := &CLab{
		Config:            &Config{Mgmt: m, MgmtNetworks: clabtypes.MgmtNetworks{m}},
		globalRuntimeName: "test",
		Runtimes:          map[string]clabruntime.ContainerRuntime{"test": rt},
		Nodes:             map[string]clabnodes.Node{"vm": node},
	}

	if err := c.CreateNetwork(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg.Env["DOCKER_NET_V4_ADDR"] != "192.0.2.0/24" ||
		cfg.Env["DOCKER_NET_V6_ADDR"] != "fd00::/64" {
		t.Fatalf("env = %v; want empty subnet filled and set subnet kept", cfg.Env)
	}
}

func TestInitNodeBindsRuntimeToNodeMgmtNetwork(t *testing.T) {
	c, err := NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}
	oob := &clabtypes.MgmtNet{Network: "oob"}
	c.Config.MgmtNetworks = clabtypes.MgmtNetworks{c.Config.Mgmt, oob}

	ctrl := gomock.NewController(t)
	base := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	binder := clabmocksmockruntime.NewMockMgmtNetBinder(ctrl)
	bound := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	binder.EXPECT().ForMgmtNet(oob).Return(bound)
	c.Runtimes["docker"] = struct {
		*clabmocksmockruntime.MockContainerRuntime
		*clabmocksmockruntime.MockMgmtNetBinder
	}{base, binder}

	for name, network := range map[string]string{"on-oob": "oob", "on-default": ""} {
		cfg := &clabtypes.NodeConfig{
			ShortName: name, Kind: "linux", MgmtNet: network, Labels: map[string]string{},
		}
		if err := c.initNode(cfg, "docker"); err != nil {
			t.Fatal(err)
		}
	}

	if c.Nodes["on-oob"].GetRuntime() != bound {
		t.Fatal("node on oob did not get the runtime bound to oob")
	}
	if c.Nodes["on-default"].GetRuntime() != c.Runtimes["docker"] {
		t.Fatal("node on the default network did not get the base runtime")
	}
}
