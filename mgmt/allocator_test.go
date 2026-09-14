package mgmt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func allocateManagementIPsForTest(
	ctx context.Context,
	m *clabtypes.MgmtNet,
	nodes []*clabtypes.NodeConfig,
	check func(context.Context, *clabtypes.MgmtNet, netip.Addr) error,
	options ...clabtypes.AllocationOptions,
) error {
	var option clabtypes.AllocationOptions
	if len(options) != 0 {
		option = options[0]
	}
	drafts, err := allocateManagementIPs(ctx, m, nodes, check, option)
	if err != nil {
		return err
	}
	for i := range nodes {
		nodes[i].MgmtIPv4Address = drafts[i].MgmtIPv4Address
		nodes[i].MgmtIPv6Address = drafts[i].MgmtIPv6Address
	}
	return nil
}

func TestAllocateIPs(t *testing.T) {
	m := &clabtypes.MgmtNet{
		IPAM:       clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab},
		IPv4Subnet: "192.0.2.0/24",
		IPv4Range:  "192.0.2.128/30",
		IPv4Gw:     "192.0.2.254",
		IPv6Subnet: "2001:db8::/64",
		IPv6Gw:     "2001:db8::ff",
		MacvlanAux: "192.0.2.129/30",
	}
	a, b := &clabtypes.NodeConfig{ShortName: "a"}, &clabtypes.NodeConfig{ShortName: "b"}
	if err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{b, a},
		clabtypes.AllocationOptions{},
	); err != nil {
		t.Fatal(err)
	}
	a2, b2 := &clabtypes.NodeConfig{ShortName: "a"}, &clabtypes.NodeConfig{ShortName: "b"}
	if err := AllocateManagementIPs(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{a2, b2},
		clabtypes.AllocationOptions{},
	); err != nil {
		t.Fatal(err)
	}
	if a.MgmtIPv4Address != a2.MgmtIPv4Address || b.MgmtIPv6Address != b2.MgmtIPv6Address {
		t.Fatal("allocation depends on input order")
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
	if err := AllocateManagementIPs(context.Background(), m, nodes, clabtypes.AllocationOptions{}); err != nil {
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
	for _, provider := range []clabtypes.IPAMProvider{"", clabtypes.IPAMProviderContainerlab, clabtypes.IPAMProviderRuntime} {
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

func TestAllocationDADBeforeCommit(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			m := &clabtypes.MgmtNet{
				Driver:      "macvlan",
				MacvlanMode: "private",
				IPv4Subnet:  "192.0.2.0/24",
				IPv6Subnet:  "2001:db8::/64",
			}
			n := &clabtypes.NodeConfig{ShortName: "node"}
			if explicit {
				n.MgmtIPv6Address = "2001:db8::2"
			}
			conflict := errors.New("probe failed")
			calls := 0
			err := allocateManagementIPsForTest(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{n},
				func(_ context.Context, _ *clabtypes.MgmtNet, ip netip.Addr) error {
					calls++
					if n.MgmtIPv4Address != "" {
						t.Fatal("committed before DAD completed")
					}
					if ip.Is6() {
						return conflict
					}
					return nil
				},
			)
			if !errors.Is(err, conflict) || calls != 2 {
				t.Fatalf("DAD result: %v, calls %d", err, calls)
			}
			if n.MgmtIPv4Address != "" || (!explicit && n.MgmtIPv6Address != "") {
				t.Fatal("failed DAD changed node addresses")
			}
		})
	}
}

func TestAllocationDADProviderAndDisable(t *testing.T) {
	for _, provider := range []clabtypes.IPAMProvider{clabtypes.IPAMProviderContainerlab, clabtypes.IPAMProviderRuntime} {
		for _, dad := range []*bool{nil, new(true), new(false)} {
			m := &clabtypes.MgmtNet{Driver: "macvlan", MacvlanMode: "private",
				IPv4Subnet: "192.0.2.0/24",
				IPAM:       clabtypes.MgmtIPAM{Provider: provider, DAD: dad},
			}
			calls := 0
			err := allocateManagementIPsForTest(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{{ShortName: "node"}},
				func(context.Context, *clabtypes.MgmtNet, netip.Addr) error { calls++; return nil },
			)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if provider == clabtypes.IPAMProviderContainerlab && (dad == nil || *dad) {
				want = 1
			}
			if calls != want {
				t.Fatalf("provider %s dad %v: %d checks, want %d", provider, dad, calls, want)
			}
		}
	}
}

func TestAllocationRejectsHostAddress(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		m := &clabtypes.MgmtNet{Driver: "bridge",
			IPv4Subnet: "127.0.0.0/8",
			IPv4Range:  "127.0.0.1/32",
			IPv4Gw:     "127.0.0.2",
		}
		n := &clabtypes.NodeConfig{ShortName: "node"}
		if explicit {
			n.MgmtIPv4Address = "127.0.0.1"
		}
		err := AllocateManagementIPs(
			context.Background(), m, []*clabtypes.NodeConfig{n}, clabtypes.AllocationOptions{})
		if explicit {
			if err != nil || n.MgmtIPv4Address != "127.0.0.1" {
				t.Fatalf("static address changed or failed: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "duplicate address") {
			t.Fatalf("missed local conflict: %v", err)
		}
		if !explicit && n.MgmtIPv4Address != "" {
			t.Fatal("committed conflicting address")
		}
	}
}

func TestAllocationPreservesExistingAddresses(t *testing.T) {
	m := &clabtypes.MgmtNet{
		Driver:      "macvlan",
		MacvlanMode: "private",
		IPv4Subnet:  "192.0.2.0/24",
		IPv4Range:   "192.0.2.128/30",
	}
	retained := &clabtypes.NodeConfig{ShortName: "retained"}
	added := &clabtypes.NodeConfig{ShortName: "added"}
	existing := netip.MustParseAddr("192.0.2.129")
	calls := 0
	err := allocateManagementIPsForTest(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{retained, added},
		func(_ context.Context, _ *clabtypes.MgmtNet, ip netip.Addr) error {
			calls++
			if ip == existing {
				t.Fatal("probed already owned address")
			}
			return nil
		},
		clabtypes.AllocationOptions{Existing: []clabtypes.ExistingAddress{{NodeName: "retained", Address: existing}}},
	)
	if err != nil || calls != 1 {
		t.Fatalf("allocation: %v, DAD calls: %d", err, calls)
	}
	if retained.MgmtIPv4Address != existing.String() || added.MgmtIPv4Address == existing.String() {
		t.Fatal("existing allocation not preserved")
	}
}

func TestAllocationDADPrefersFreeAddress(t *testing.T) {
	for _, subnet := range []string{"192.0.2.0/29", "2001:db8::/125"} {
		t.Run(subnet, func(t *testing.T) {
			prefix := netip.MustParsePrefix(subnet)
			m := &clabtypes.MgmtNet{Driver: "macvlan", MacvlanMode: "private"}
			if prefix.Addr().Is4() {
				m.IPv4Subnet = subnet
			} else {
				m.IPv6Subnet = subnet
			}
			n := &clabtypes.NodeConfig{ShortName: "node"}
			seen := map[netip.Addr]bool{}
			var accepted netip.Addr
			err := allocateManagementIPsForTest(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{n},
				func(_ context.Context, _ *clabtypes.MgmtNet, ip netip.Addr) error {
					if seen[ip] {
						t.Fatalf("retried occupied candidate %s", ip)
					}
					seen[ip] = true
					if len(seen) <= 2 {
						return fmt.Errorf("peer owns %s: %w", ip, ErrDuplicateAddress)
					}
					accepted = ip
					return nil
				},
			)
			if err != nil || len(seen) != 3 {
				t.Fatalf("allocation error %v; checked %d candidates", err, len(seen))
			}
			got := n.MgmtIPv6Address
			if prefix.Addr().Is4() {
				got = n.MgmtIPv4Address
			}
			if got != accepted.String() {
				t.Fatalf("assigned %q instead of free candidate %s", got, accepted)
			}
		})
	}
}

func TestAllocationDADExhaustionAndStaticConflict(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		m := &clabtypes.MgmtNet{
			Driver:      "macvlan",
			MacvlanMode: "private",
			IPv4Subnet:  "192.0.2.0/29",
		}
		n := &clabtypes.NodeConfig{ShortName: "node"}
		if explicit {
			n.MgmtIPv4Address = "192.0.2.2"
		}
		calls := 0
		err := allocateManagementIPsForTest(
			context.Background(),
			m,
			[]*clabtypes.NodeConfig{n},
			func(context.Context, *clabtypes.MgmtNet, netip.Addr) error {
				calls++
				return ErrDuplicateAddress
			},
		)
		if explicit && err != nil {
			t.Fatalf("static conflict should warn: %v", err)
		}
		if !explicit && !errors.Is(err, ErrDuplicateAddress) {
			t.Fatalf("expected conflict, got %v", err)
		}
		if explicit {
			if calls != 1 || n.MgmtIPv4Address != "192.0.2.2" {
				t.Fatal("changed or retried explicit address")
			}
		} else if calls != 5 || n.MgmtIPv4Address != "" || !strings.Contains(err.Error(), "exhausted") {
			t.Fatalf("expected exhausted pool without committing: %v; calls %d", err, calls)
		}
	}

	m := &clabtypes.MgmtNet{
		Driver:     "macvlan",
		IPv4Subnet: "192.0.2.0/29",
	}
	n := &clabtypes.NodeConfig{ShortName: "node", MgmtIPv4Address: "192.0.2.2"}
	calls := 0
	if err := allocateManagementIPsForTest(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		func(context.Context, *clabtypes.MgmtNet, netip.Addr) error {
			calls++
			return nil
		},
		clabtypes.AllocationOptions{Reserved: []netip.Addr{netip.MustParseAddr("192.0.2.2")}},
	); err != nil {
		t.Fatalf("runtime static conflict should warn: %v", err)
	}
	if calls != 0 || n.MgmtIPv4Address != "192.0.2.2" {
		t.Fatal("runtime conflict changed or probed explicit address")
	}
}

func TestAllocationDADCancellationDuringRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &clabtypes.MgmtNet{Driver: "macvlan", MacvlanMode: "private", IPv6Subnet: "2001:db8::/64"}
	n := &clabtypes.NodeConfig{ShortName: "node"}
	calls := 0
	err := allocateManagementIPsForTest(
		ctx,
		m,
		[]*clabtypes.NodeConfig{n},
		func(context.Context, *clabtypes.MgmtNet, netip.Addr) error {
			calls++
			cancel()
			return ErrDuplicateAddress
		},
	)
	if !errors.Is(err, context.Canceled) || calls != 1 || n.MgmtIPv6Address != "" {
		t.Fatalf("cancellation: %v; calls %d", err, calls)
	}
}

func TestAllocationUsesLocalDADForNonMacvlan(t *testing.T) {
	for _, driver := range []string{"", "bridge"} {
		m := &clabtypes.MgmtNet{
			Driver:     driver,
			IPv4Subnet: "192.0.2.0/24",
			IPAM:       clabtypes.MgmtIPAM{DAD: new(true)},
		}
		n := &clabtypes.NodeConfig{ShortName: "node"}
		calls := 0
		err := allocateManagementIPsForTest(
			context.Background(),
			m,
			[]*clabtypes.NodeConfig{n},
			func(context.Context, *clabtypes.MgmtNet, netip.Addr) error {
				calls++
				return nil
			},
		)
		if err != nil || n.MgmtIPv4Address == "" || calls != 1 {
			t.Fatalf("allocation failed: %v; DAD calls: %d", err, calls)
		}
	}
}

func TestAllocationSkipsWholeReservedSubnet(t *testing.T) {
	m := &clabtypes.MgmtNet{IPv6Subnet: "2001:db8::/32"}
	n := &clabtypes.NodeConfig{ShortName: "node"}
	checks := 0
	var blocked netip.Prefix
	err := allocateManagementIPsForTest(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		func(_ context.Context, _ *clabtypes.MgmtNet, ip netip.Addr) error {
			checks++
			if checks == 1 {
				blocked = netip.PrefixFrom(ip, 33).Masked()
				return &occupiedPrefix{prefix: blocked}
			}
			if blocked.Contains(ip) {
				t.Fatal("scanned another candidate inside reserved /33")
			}
			return nil
		},
	)
	if err != nil || checks != 2 || n.MgmtIPv6Address == "" {
		t.Fatalf("subnet skip: %v, checks %d", err, checks)
	}
}

func TestPreferredAllocationPreferences(t *testing.T) {
	for _, tc := range []struct {
		name, preferred, explicit string
		conflict                  bool
	}{
		{name: "reuse", preferred: "192.0.2.5"},
		{name: "DAD overrides preference", preferred: "192.0.2.5", conflict: true},
		{name: "out of pool", preferred: "198.51.100.5"},
		{name: "invalid", preferred: "invalid"},
		{name: "reserved gateway", preferred: "192.0.2.1"},
		{name: "explicit wins", preferred: "192.0.2.5", explicit: "192.0.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &clabtypes.MgmtNet{IPv4Subnet: "192.0.2.0/29"}
			n := &clabtypes.NodeConfig{ShortName: "node", MgmtIPv4Address: tc.explicit}
			var checked []netip.Addr
			err := allocateManagementIPsForTest(
				context.Background(),
				m,
				[]*clabtypes.NodeConfig{n},
				func(_ context.Context, _ *clabtypes.MgmtNet, ip netip.Addr) error {
					checked = append(checked, ip)
					if tc.conflict && ip.String() == tc.preferred {
						return ErrDuplicateAddress
					}
					return nil
				},
				clabtypes.AllocationOptions{Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv4: tc.preferred}}},
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
				if n.MgmtIPv4Address != tc.preferred || checked[0].String() != tc.preferred {
					t.Fatal("preferred address was not used and checked")
				}
			default:
				if n.MgmtIPv4Address == tc.preferred || n.MgmtIPv4Address == "" {
					t.Fatalf("invalid fallback %s", n.MgmtIPv4Address)
				}
			}
			if tc.conflict && (len(checked) != 2 || checked[0].String() != tc.preferred) {
				t.Fatalf("DAD did not try preferred address first: %v", checked)
			}
		})
	}
}

func TestPreferredIPv6AndRuntimeOwnership(t *testing.T) {
	m := &clabtypes.MgmtNet{IPv6Subnet: "2001:db8::/125"}
	n := &clabtypes.NodeConfig{ShortName: "node"}
	live := netip.MustParseAddr("2001:db8::3")
	checks := 0
	err := allocateManagementIPsForTest(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		func(context.Context, *clabtypes.MgmtNet, netip.Addr) error { checks++; return nil },
		clabtypes.AllocationOptions{
			Existing:  []clabtypes.ExistingAddress{{NodeName: "node", Address: live}},
			Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv6: "2001:db8::5"}},
		},
	)
	if err != nil || checks != 0 || n.MgmtIPv6Address != live.String() {
		t.Fatalf("runtime ownership did not win: %v, %+v", err, n)
	}
	n.MgmtIPv6Address = ""
	err = allocateManagementIPsForTest(
		context.Background(),
		m,
		[]*clabtypes.NodeConfig{n},
		func(context.Context, *clabtypes.MgmtNet, netip.Addr) error { checks++; return nil },
		clabtypes.AllocationOptions{Preferred: map[string]clabtypes.NodeAddresses{"node": {IPv6: "2001:db8::5"}}},
	)
	if err != nil || checks != 1 || n.MgmtIPv6Address != "2001:db8::5" {
		t.Fatalf("IPv6 preference not checked/reused: %v", err)
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
		clabtypes.AllocationOptions{Preferred: map[string]clabtypes.NodeAddresses{"z-retained": {IPv4: preferred}}},
	)
	if err != nil || retained.MgmtIPv4Address != preferred || newNode.MgmtIPv4Address == preferred {
		t.Fatalf("new node stole preferred address: %v", err)
	}
}

func TestStaticAndPreferredAddressesShareDADBatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	m := &clabtypes.MgmtNet{Driver: "macvlan", IPv4Subnet: "192.0.2.0/24"}
	nodes := []*clabtypes.NodeConfig{
		{ShortName: "static", MgmtIPv4Address: "192.0.2.2"},
		{ShortName: "preferred"},
	}
	var calls atomic.Int32
	gate := make(chan struct{})
	err := allocateManagementIPsForTest(
		ctx,
		m,
		nodes,
		func(ctx context.Context, _ *clabtypes.MgmtNet, _ netip.Addr) error {
			if calls.Add(1) == 2 {
				close(gate)
			}
			select {
			case <-gate:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		clabtypes.AllocationOptions{Preferred: map[string]clabtypes.NodeAddresses{
			"preferred": {IPv4: "192.0.2.3"},
		}},
	)
	if err != nil || calls.Load() != 2 || nodes[1].MgmtIPv4Address != "192.0.2.3" {
		t.Fatalf("shared DAD batch failed: %v, calls=%d, preferred=%s",
			err, calls.Load(), nodes[1].MgmtIPv4Address)
	}
}

func TestMacvlanAllocationConcurrentChecks(t *testing.T) {
	for _, mode := range []string{"generated", "preferred", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m := &clabtypes.MgmtNet{Driver: "macvlan", IPv4Subnet: "198.18.0.0/16"}
			nodes := make([]*clabtypes.NodeConfig, 130)
			preferred := make(map[string]clabtypes.NodeAddresses)
			for i := range nodes {
				name := fmt.Sprintf("node-%03d", i)
				ip := fmt.Sprintf("198.18.1.%d", i+1)
				nodes[i] = &clabtypes.NodeConfig{ShortName: name}
				if mode == "explicit" {
					nodes[i].MgmtIPv4Address = ip
				}
				if mode == "preferred" {
					preferred[name] = clabtypes.NodeAddresses{IPv4: ip}
				}
			}
			var active, peak, calls atomic.Int32
			gate := make(chan struct{})
			err := allocateManagementIPsForTest(ctx, m, nodes, func(ctx context.Context, _ *clabtypes.MgmtNet, _ netip.Addr) error {
				count := active.Add(1)
				defer active.Add(-1)
				for old := peak.Load(); count > old; old = peak.Load() {
					if peak.CompareAndSwap(old, count) {
						break
					}
				}
				if calls.Add(1) == int32(len(nodes)) {
					close(gate)
				}
				select {
				case <-gate:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}, clabtypes.AllocationOptions{Preferred: preferred})
			if err != nil {
				t.Fatal(err)
			}
			if peak.Load() != int32(len(nodes)) || active.Load() != 0 || calls.Load() != 130 {
				t.Fatalf("concurrency peak=%d active=%d calls=%d", peak.Load(), active.Load(), calls.Load())
			}
			used := make(map[string]bool)
			for _, n := range nodes {
				if n.MgmtIPv4Address == "" || used[n.MgmtIPv4Address] {
					t.Fatal("missing or duplicate allocation")
				}
				used[n.MgmtIPv4Address] = true
			}
		})
	}
}

func TestStaticManagementAddressDADWarning(t *testing.T) {
	for _, driver := range []string{"bridge", "macvlan"} {
		for _, dad := range []*bool{nil, new(true), new(false)} {
			t.Run(fmt.Sprintf("%s/%v", driver, dad), func(t *testing.T) {
				var output bytes.Buffer
				previous := log.Default()
				log.SetDefault(log.New(&output))
				t.Cleanup(func() { log.SetDefault(previous) })
				m := &clabtypes.MgmtNet{Driver: driver, MacvlanMode: "bridge",
					IPv4Subnet: "192.0.2.0/24", IPv6Subnet: "2001:db8::/64",
					IPAM: clabtypes.MgmtIPAM{DAD: dad}}
				n := &clabtypes.NodeConfig{ShortName: "static", MgmtIPv4Address: "192.0.2.5", MgmtIPv6Address: "2001:db8::5"}
				var calls atomic.Int32
				err := allocateManagementIPsForTest(context.Background(), m, []*clabtypes.NodeConfig{n},
					func(context.Context, *clabtypes.MgmtNet, netip.Addr) error {
						calls.Add(1)
						return ErrDuplicateAddress
					})
				if err != nil || n.MgmtIPv4Address != "192.0.2.5" || n.MgmtIPv6Address != "2001:db8::5" {
					t.Fatalf("static addresses changed or failed: %+v, %v", n, err)
				}
				if !m.IPAM.DADEnabled() {
					if calls.Load() != 0 || output.Len() != 0 {
						t.Fatal("disabled DAD checked or warned")
					}
					return
				}
				if calls.Load() != 2 || strings.Count(output.String(), "Duplicate static management address") != 2 ||
					!strings.Contains(output.String(), "static") || !strings.Contains(output.String(), "192.0.2.5") ||
					!strings.Contains(output.String(), "2001:db8::5") {
					t.Fatalf("missing dual-stack checks or warnings: calls=%d, log=%s", calls.Load(), output.String())
				}
			})
		}
	}
}
