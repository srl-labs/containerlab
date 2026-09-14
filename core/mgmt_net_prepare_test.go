package core

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"testing"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	"go.uber.org/mock/gomock"

	clabtypes "github.com/srl-labs/containerlab/types"
)

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
	rt.EXPECT().CreateNet(gomock.Any()).DoAndReturn(func(context.Context) error {
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
			// Only network labeling should read the node config; allocation is skipped.
			node.EXPECT().Config().Return(cfg).Times(1)
			c := &CLab{
				Config: &Config{Mgmt: m}, globalRuntimeName: "test",
				Runtimes: map[string]clabruntime.ContainerRuntime{"test": rt},
				Nodes:    map[string]clabnodes.Node{"node": node},
			}
			rt.EXPECT().CreateNet(gomock.Any()).Return(nil)
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
	if err := os.WriteFile(paths.StateFile(), []byte("nodes:\n  node:\n    ipam:\n      ipv4: 192.0.2.5\n"), 0644); err != nil {
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
			m := &clabtypes.MgmtNet{Network: "mgmt", IPv4Subnet: "192.0.2.0/29", IPAM: clabtypes.MgmtIPAM{DAD: new(false)}}
			cfg := &clabtypes.NodeConfig{ShortName: "node", Labels: map[string]string{}}
			node := clabmocksmocknodes.NewMockNode(ctrl)
			node.EXPECT().Config().Return(cfg).AnyTimes()
			paths := &clabtypes.TopoPaths{}
			if err := paths.SetLabDir(t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.StateFile(), []byte("nodes:\n  node:\n    ipam:\n      ipv4: 192.0.2.5\n"), 0644); err != nil {
				t.Fatal(err)
			}
			c := &CLab{TopoPaths: paths, Config: &Config{Mgmt: m}, globalRuntimeName: "test", Runtimes: map[string]clabruntime.ContainerRuntime{"test": rt}, Nodes: map[string]clabnodes.Node{"node": node}}
			ip := netip.MustParseAddr("192.0.2.5")
			occupied := []clabruntime.NetworkAddress{{NetworkName: "mgmt", ContainerID: "foreign-id", Address: ip}}
			var existing []clabtypes.ExistingAddress
			if scenario == "own-live" || scenario == "foreign-conflict" {
				existing = []clabtypes.ExistingAddress{{NodeName: "node", ContainerID: "own-id", Address: ip}}
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
			rt.EXPECT().NetworkAddresses(gomock.Any(), []netip.Prefix{netip.MustParsePrefix(m.IPv4Subnet)}).Return(occupied, snapshotErr)
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
			if scenario == "foreign-preferred" && (cfg.MgmtIPv4Address == "" || cfg.MgmtIPv4Address == ip.String()) {
				t.Fatal("foreign runtime reservation ignored")
			}
		})
	}
}
