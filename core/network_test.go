// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
		[]byte("nodes:\n  node:\n    ipam:\n      ipv4: 192.0.2.5\n"),
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
				[]byte("nodes:\n  node:\n    ipam:\n      ipv4: 192.0.2.5\n"),
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
