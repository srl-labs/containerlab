package mgmt

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestAllocateIPs(t *testing.T) {
	m := &clabtypes.MgmtNet{
		IPAM:       clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab},
		IPv4Subnet: "192.0.2.0/24",
		IPv4Range:  "192.0.2.128/30",
		IPv4Gw:     "192.0.2.254",
		IPv6Subnet: "2001:db8::/64",
		IPv6Gw:     "2001:db8::ff",
	}
	options := clabtypes.AllocationOptions{
		Reserved: []netip.Addr{netip.MustParseAddr("192.0.2.129")},
	}
	a, b := &clabtypes.NodeConfig{ShortName: "a"}, &clabtypes.NodeConfig{ShortName: "b"}
	if err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{b, a},
		options,
	); err != nil {
		t.Fatal(err)
	}
	for _, n := range []*clabtypes.NodeConfig{a, b} {
		if !netip.MustParsePrefix(m.IPv4Range).Contains(netip.MustParseAddr(n.MgmtIPv4Address)) ||
			n.MgmtIPv4Address == "192.0.2.129" {
			t.Fatalf("invalid allocation: %+v", n)
		}
		if n.MgmtIPv4PrefixLength != 24 || n.MgmtIPv4Gateway != m.IPv4Gw ||
			n.MgmtIPv6PrefixLength != 64 || n.MgmtIPv6Gateway != m.IPv6Gw {
			t.Fatalf("missing management IP configuration: %+v", n)
		}
	}
	if a.MgmtIPv4Address == b.MgmtIPv4Address {
		t.Fatal("collision")
	}
}

func TestAllocateIPsReservationsAndExhaustion(t *testing.T) {
	m := &clabtypes.MgmtNet{
		IPAM:       clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab},
		IPv4Subnet: "192.0.2.0/29",
	}
	nodes := []*clabtypes.NodeConfig{
		{ShortName: "static", MgmtIPv4Address: "192.0.2.2"},
		{ShortName: "host", NetworkMode: "host"},
		{ShortName: "root", IsRootNamespaceBased: true},
		{ShortName: "external", SkipUniquenessCheck: true},
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		nodes = append(nodes, &clabtypes.NodeConfig{ShortName: name})
	}
	if err := AllocateManagementIPs(
		context.Background(),
		m,
		nodes,
		clabtypes.AllocationOptions{},
	); err != nil {
		t.Fatal(err)
	}
	if nodes[0].MgmtIPv4Address != "192.0.2.2" ||
		nodes[1].MgmtIPv4Address != "" ||
		nodes[2].MgmtIPv4Address != "" ||
		nodes[3].MgmtIPv4Address != "" {
		t.Fatal("explicit or ineligible address changed")
	}
	if nodes[0].MgmtIPv4PrefixLength != 29 || nodes[0].MgmtIPv4Gateway != "192.0.2.1" {
		t.Fatalf("static address missing management IP configuration: %+v", nodes[0])
	}
	if err := AllocateManagementIPs(context.Background(),
		m,
		append(nodes, &clabtypes.NodeConfig{ShortName: "overflow"}),
		clabtypes.AllocationOptions{},
	); err == nil {
		t.Fatal("expected exhaustion")
	}
	if err := AllocateManagementIPs(context.Background(),
		m,
		[]*clabtypes.NodeConfig{{ShortName: "a", MgmtIPv4Address: "192.0.2.1"}},
		clabtypes.AllocationOptions{},
	); err == nil {
		t.Fatal("accepted gateway")
	}
	if err := AllocateManagementIPs(
		context.Background(),
		&clabtypes.MgmtNet{
			IPAM:       clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab},
			IPv4Subnet: "192.0.2.0/24",
			IPv4Range:  "192.0.2.128/26",
		},
		[]*clabtypes.NodeConfig{{ShortName: "a", MgmtIPv4Address: "192.0.2.10"}},
		clabtypes.AllocationOptions{},
	); err == nil || !strings.Contains(err.Error(), "192.0.2.10") ||
		!strings.Contains(err.Error(), "192.0.2.128/26") {
		t.Fatalf("static address outside IP range: %v", err)
	}
	if err := AllocateManagementIPs(context.Background(),
		m,
		[]*clabtypes.NodeConfig{
			{ShortName: "a", MgmtIPv4Address: "192.0.2.2"},
			{ShortName: "b", MgmtIPv4Address: "192.0.2.2"},
		},
		clabtypes.AllocationOptions{},
	); err == nil {
		t.Fatal("accepted duplicate")
	}
}

func TestManagementIPAMProvider(t *testing.T) {
	for _, provider := range []clabtypes.IPAMProvider{
		"",
		clabtypes.IPAMProviderContainerlab,
		clabtypes.IPAMProviderRuntime,
	} {
		t.Run(string(provider), func(t *testing.T) {
			m := &clabtypes.MgmtNet{
				IPv4Subnet: "192.0.2.0/24",
				IPAM:       clabtypes.MgmtIPAM{Provider: provider},
			}
			n := &clabtypes.NodeConfig{ShortName: "node"}
			if err := AllocateManagementIPs(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{n},
				clabtypes.AllocationOptions{},
			); err != nil {
				t.Fatal(err)
			}
			if (n.MgmtIPv4Address == "") != (provider == clabtypes.IPAMProviderRuntime) {
				t.Fatalf("provider %q allocated %q", provider, n.MgmtIPv4Address)
			}
		})
	}
}

func TestAllocationPreservesExistingAddresses(t *testing.T) {
	m := &clabtypes.MgmtNet{
		Driver:      "macvlan",
		MacvlanMode: "private",
		IPv4Subnet:  "192.0.2.0/24",
		IPv4Range:   "192.0.2.128/30",
		IPAM:        clabtypes.MgmtIPAM{DAD: new(false)},
	}
	retained := &clabtypes.NodeConfig{ShortName: "retained"}
	added := &clabtypes.NodeConfig{ShortName: "added"}
	existing := netip.MustParseAddr("192.0.2.129")
	err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{retained, added},
		clabtypes.AllocationOptions{
			Existing: []clabtypes.ExistingAddress{{NodeName: "retained", Address: existing}},
		},
	)
	if err != nil {
		t.Fatalf("allocation: %v", err)
	}
	if retained.MgmtIPv4Address != existing.String() || added.MgmtIPv4Address == existing.String() {
		t.Fatal("existing allocation not preserved")
	}
}

func TestAllocationReportsConflictingExistingAddressOwners(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.10")
	err := AllocateManagementIPs(
		context.Background(),
		&clabtypes.MgmtNet{
			IPAM:       clabtypes.MgmtIPAM{DAD: new(false)},
			IPv4Subnet: "192.0.2.0/24",
		},
		nil,
		clabtypes.AllocationOptions{Existing: []clabtypes.ExistingAddress{
			{NodeName: "running", ContainerID: "container-running", Address: address},
			{NodeName: "stopped", ContainerID: "container-stopped", Address: address},
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "running") ||
		!strings.Contains(err.Error(), "stopped") ||
		!strings.Contains(err.Error(), "container-stopped") ||
		!strings.Contains(err.Error(), address.String()) {
		t.Fatalf("missing existing address owner context: %v", err)
	}
}

func TestPreferredAllocationPreferences(t *testing.T) {
	for _, tc := range []struct {
		name, preferred, explicit string
	}{
		{name: "reuse", preferred: "192.0.2.5"},
		{name: "out of pool", preferred: "198.51.100.5"},
		{name: "invalid", preferred: "invalid"},
		{name: "reserved gateway", preferred: "192.0.2.1"},
		{name: "explicit wins", preferred: "192.0.2.5", explicit: "192.0.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &clabtypes.MgmtNet{
				IPv4Subnet: "192.0.2.0/29",
				IPAM:       clabtypes.MgmtIPAM{DAD: new(false)},
			}
			n := &clabtypes.NodeConfig{ShortName: "node", MgmtIPv4Address: tc.explicit}
			err := AllocateManagementIPs(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{n},
				clabtypes.AllocationOptions{
					Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv4: tc.preferred}},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.explicit != "":
				if n.MgmtIPv4Address != tc.explicit {
					t.Fatal("preference replaced explicit address")
				}
			case tc.name == "reuse":
				if n.MgmtIPv4Address != tc.preferred {
					t.Fatal("preferred address was not reused")
				}
			default:
				if n.MgmtIPv4Address == tc.preferred || n.MgmtIPv4Address == "" {
					t.Fatalf("invalid fallback %s", n.MgmtIPv4Address)
				}
			}
		})
	}
}

func TestPreferredIPv6AndRuntimeOwnership(t *testing.T) {
	m := &clabtypes.MgmtNet{
		IPv6Subnet: "2001:db8::/125",
		IPAM:       clabtypes.MgmtIPAM{DAD: new(false)},
	}
	n := &clabtypes.NodeConfig{ShortName: "node"}
	live := netip.MustParseAddr("2001:db8::3")
	err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		clabtypes.AllocationOptions{
			Existing:  []clabtypes.ExistingAddress{{NodeName: "node", Address: live}},
			Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv6: "2001:db8::5"}},
		},
	)
	if err != nil || n.MgmtIPv6Address != live.String() {
		t.Fatalf("runtime ownership did not win: %v, %+v", err, n)
	}
	n.MgmtIPv6Address = ""
	err = AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		clabtypes.AllocationOptions{
			Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv6: "2001:db8::5"}},
		},
	)
	if err != nil || n.MgmtIPv6Address != "2001:db8::5" {
		t.Fatalf("IPv6 preference was not reused: %v", err)
	}
}

func TestPreferredAddressReservedBeforeNewNodes(t *testing.T) {
	m := &clabtypes.MgmtNet{IPv4Subnet: "192.0.2.0/29", IPAM: clabtypes.MgmtIPAM{DAD: new(false)}}
	newNode := &clabtypes.NodeConfig{ShortName: "a-new"}
	if err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{newNode},
		clabtypes.AllocationOptions{},
	); err != nil {
		t.Fatal(err)
	}
	preferred := newNode.MgmtIPv4Address
	newNode.MgmtIPv4Address = ""
	retained := &clabtypes.NodeConfig{ShortName: "z-retained"}
	err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{newNode, retained},
		clabtypes.AllocationOptions{
			Preferred: map[string]clabtypes.NodeAddresses{"z-retained": {IPv4: preferred}},
		},
	)
	if err != nil || retained.MgmtIPv4Address != preferred || newNode.MgmtIPv4Address == preferred {
		t.Fatalf("new node stole preferred address: %v", err)
	}
}
