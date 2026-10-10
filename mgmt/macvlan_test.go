package mgmt

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type parentNetlink struct {
	clabutils.MacvlanHostNetlink
	addresses []netlink.Addr
	err       error
	parent    netlink.Link
	lookupErr error
	routes    []netlink.Route
	routeErr  error
}

func (f parentNetlink) AddrList(_ netlink.Link, _ int) ([]netlink.Addr, error) {
	return f.addresses, f.err
}

func (f parentNetlink) RouteList(netlink.Link, int) ([]netlink.Route, error) {
	return f.routes, f.routeErr
}

func TestMacvlanParentSubnet(t *testing.T) {
	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent"}}
	for _, tc := range []struct {
		name          string
		addresses     []string
		family        int
		want, failure string
	}{
		{"IPv4", []string{"192.0.2.10/24", "192.0.2.11/24", "2001:db8::1/64"}, netlink.FAMILY_V4, "192.0.2.0/24", ""},
		{"IPv6", []string{"2001:db8::1/64", "192.0.2.10/24"}, netlink.FAMILY_V6, "2001:db8::/64", ""},
		{"non-global", []string{"127.0.0.1/8", "169.254.1.1/16", "fe80::1/64"}, netlink.FAMILY_V4, "", ""},
		{"empty", nil, netlink.FAMILY_V6, "", ""},
		{"ambiguous", []string{"192.0.2.1/24", "198.51.100.1/24"}, netlink.FAMILY_V4, "", "multiple subnets"},
		{"IPv4 point-to-point", []string{"192.0.2.0/31"}, netlink.FAMILY_V4, "", "at least four addresses"},
		{"IPv6 host", []string{"2001:db8::1/128"}, netlink.FAMILY_V6, "", "at least four addresses"},
		{"smallest IPv4", []string{"192.0.2.1/30"}, netlink.FAMILY_V4, "192.0.2.0/30", ""},
		{"smallest IPv6", []string{"2001:db8::1/126"}, netlink.FAMILY_V6, "2001:db8::/126", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := parentNetlink{}
			for _, cidr := range tc.addresses {
				addr, err := netlink.ParseAddr(cidr)
				if err != nil {
					t.Fatal(err)
				}
				f.addresses = append(f.addresses, *addr)
			}
			got, err := macvlanParentSubnet(f, parent, tc.family)
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("got %v, want %s", err, tc.failure)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got.IsValid() {
					t.Fatalf("unexpected subnet %s", got)
				}
			} else if got.String() != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
	failure := errors.New("read failed")
	if _, err := macvlanParentSubnet(
		parentNetlink{err: failure},
		parent,
		netlink.FAMILY_V4,
	); !errors.Is(
		err,
		failure,
	) {
		t.Fatalf("lost read error: %v", err)
	}
}

func TestResolveMacvlanSubnets(t *testing.T) {
	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent", Flags: net.FlagUp}}
	address, err := netlink.ParseAddr("192.0.2.10/24")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		configure func(*clabtypes.MgmtNet)
		failure   string
	}{
		{"inferred", func(m *clabtypes.MgmtNet) {}, ""},
		{"explicit", func(m *clabtypes.MgmtNet) { m.IPv4Subnet = "198.51.100.0/24" }, ""},
		{"missing gateway family", func(m *clabtypes.MgmtNet) { m.IPv6Gw = "2001:db8::1" }, "no usable IPv6 subnet"},
		{"missing range family", func(m *clabtypes.MgmtNet) { m.IPv6Range = "2001:db8::/80" }, "no usable IPv6 subnet"},
		{"gateway outside subnet", func(m *clabtypes.MgmtNet) { m.IPv4Gw = "198.51.100.1" }, "gateway"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := clabtypes.MgmtNet{Driver: "macvlan", MacvlanParent: "parent"}
			tc.configure(&m)
			before := m
			err := resolveMacvlanSubnets(
				&m,
				parentNetlink{addresses: []netlink.Addr{*address}},
				parent,
			)
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("got %v, want %s", err, tc.failure)
				}
				if !reflect.DeepEqual(m, before) {
					t.Fatal("failed resolution modified configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := before.IPv4Subnet
			if want == "" {
				want = "192.0.2.0/24"
			}
			if m.IPv4Subnet != want || m.IPv6Subnet != "" {
				t.Fatalf("unexpected subnets %q, %q", m.IPv4Subnet, m.IPv6Subnet)
			}
		})
	}
	m := clabtypes.MgmtNet{Driver: "macvlan", MacvlanParent: "parent"}
	if err := resolveMacvlanSubnets(&m, parentNetlink{}, parent); err == nil {
		t.Fatal("accepted parent without usable addresses")
	}

	for _, tc := range []struct {
		name      string
		addresses []string
	}{
		{"usable", []string{"2001:db8::1/64"}},
		{"ambiguous", []string{"2001:db8::1/64", "2001:db9::1/64"}},
	} {
		t.Run("unrequested IPv6 "+tc.name, func(t *testing.T) {
			m := clabtypes.MgmtNet{
				Driver:        "macvlan",
				MacvlanParent: "parent",
				IPv4Subnet:    "192.0.2.0/24",
			}
			var addresses []netlink.Addr
			for _, value := range tc.addresses {
				address, err := netlink.ParseAddr(value)
				if err != nil {
					t.Fatal(err)
				}
				addresses = append(addresses, *address)
			}
			if err := resolveMacvlanSubnets(
				&m,
				parentNetlink{addresses: addresses},
				parent,
			); err != nil {
				t.Fatalf("unrequested IPv6 inference blocked explicit IPv4: %v", err)
			}
			if m.IPv6Subnet != "" {
				t.Fatalf("unexpected inferred IPv6 subnet %q", m.IPv6Subnet)
			}
		})
	}

	for _, tc := range []struct {
		name      string
		addresses []string
	}{
		{"usable", []string{"192.0.2.1/24"}},
		{"ambiguous", []string{"192.0.2.1/24", "198.51.100.1/24"}},
	} {
		t.Run("unrequested IPv4 "+tc.name, func(t *testing.T) {
			m := clabtypes.MgmtNet{
				Driver:        "macvlan",
				MacvlanParent: "parent",
				IPv6Subnet:    "2001:db8::/64",
			}
			var addresses []netlink.Addr
			for _, value := range tc.addresses {
				address, err := netlink.ParseAddr(value)
				if err != nil {
					t.Fatal(err)
				}
				addresses = append(addresses, *address)
			}
			if err := resolveMacvlanSubnets(
				&m,
				parentNetlink{addresses: addresses},
				parent,
			); err != nil {
				t.Fatalf("unrequested IPv4 inference blocked explicit IPv6: %v", err)
			}
			if m.IPv4Subnet != "" {
				t.Fatalf("unexpected inferred IPv4 subnet %q", m.IPv4Subnet)
			}
		})
	}

	m = clabtypes.MgmtNet{
		Driver:        "macvlan",
		MacvlanParent: "parent",
		IPv4Subnet:    "192.0.2.0/24",
		IPv6Range:     "2001:db8::/80",
	}
	ipv6Address, err := netlink.ParseAddr("2001:db8::1/64")
	if err != nil {
		t.Fatal(err)
	}
	if err := resolveMacvlanSubnets(
		&m,
		parentNetlink{addresses: []netlink.Addr{*ipv6Address}},
		parent,
	); err != nil {
		t.Fatalf("requested IPv6 inference failed: %v", err)
	}
	if m.IPv6Subnet != "2001:db8::/64" {
		t.Fatalf("requested IPv6 subnet was not inferred: %q", m.IPv6Subnet)
	}

	m = clabtypes.MgmtNet{
		Driver:        "macvlan",
		MacvlanParent: "parent",
		IPv6Subnet:    "2001:db8::/64",
		IPv4Range:     "192.0.2.0/25",
	}
	ipv4Address, err := netlink.ParseAddr("192.0.2.1/24")
	if err != nil {
		t.Fatal(err)
	}
	if err := resolveMacvlanSubnets(
		&m,
		parentNetlink{addresses: []netlink.Addr{*ipv4Address}},
		parent,
	); err != nil {
		t.Fatalf("requested IPv4 inference failed: %v", err)
	}
	if m.IPv4Subnet != "192.0.2.0/24" {
		t.Fatalf("requested IPv4 subnet was not inferred: %q", m.IPv4Subnet)
	}
}

func TestNewMacvlanHost(t *testing.T) {
	links := &parentNetlink{}
	if host := NewMacvlanHost(links); host.Links != links {
		t.Fatal("incorrect netlink backend")
	}
	if NewMacvlanHost(nil).Links == nil {
		t.Fatal("missing default netlink backend")
	}
}

func (f parentNetlink) LinkByName(string) (netlink.Link, error) {
	return f.parent, f.lookupErr
}

func TestResolveMacvlanAuxAddresses(t *testing.T) {
	newConfig := func() *clabtypes.MgmtNet {
		return &clabtypes.MgmtNet{
			Network:    "test",
			IPv4Subnet: "192.0.2.0/24",
			IPv4Gw:     "192.0.2.1",
			IPv6Subnet: "2001:db8::/64",
			IPv6Gw:     "2001:db8::1",
		}
	}
	first := newConfig()
	firstAddresses, err := ResolveMacvlanAuxAddresses(
		context.Background(),
		first,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !netip.MustParsePrefix(first.IPv4Subnet).Contains(firstAddresses[0]) ||
		!netip.MustParsePrefix(first.IPv6Subnet).Contains(firstAddresses[1]) {
		t.Fatalf("incorrect dual-stack auxiliary addresses: %v", firstAddresses)
	}
	second := newConfig()
	secondAddresses, err := ResolveMacvlanAuxAddresses(context.Background(), second, nil)
	if err != nil {
		t.Fatal(err)
	}
	repeated := newConfig()
	repeatedAddresses, err := ResolveMacvlanAuxAddresses(context.Background(), repeated, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(secondAddresses, repeatedAddresses) {
		t.Fatalf("auxiliary addresses changed: %v, then %v", secondAddresses, repeatedAddresses)
	}
	excluded := newConfig()
	excludedAddresses, err := ResolveMacvlanAuxAddresses(
		context.Background(), excluded, secondAddresses,
	)
	if err != nil {
		t.Fatal(err)
	}
	if excludedAddresses[0] == secondAddresses[0] || excludedAddresses[1] == secondAddresses[1] {
		t.Fatalf("selected an excluded node address: %v", excludedAddresses)
	}
}

func TestRestoreMacvlanAuxAddresses(t *testing.T) {
	m := &clabtypes.MgmtNet{IPv4Subnet: "192.0.2.0/24", IPv6Subnet: "2001:db8::/64"}
	addresses, err := RestoreMacvlanAuxAddresses(m, "192.0.2.2", "2001:db8::2")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(addresses, []netip.Addr{
		netip.MustParseAddr("192.0.2.2"),
		netip.MustParseAddr("2001:db8::2"),
	}) {
		t.Fatalf("addresses not restored: %v", addresses)
	}
	for _, addresses := range [][2]string{
		{"", "2001:db8::2"},
		{"192.0.2.2", ""},
		{"198.51.100.2", "2001:db8::2"},
	} {
		if _, err := RestoreMacvlanAuxAddresses(m, addresses[0], addresses[1]); err == nil {
			t.Fatalf("accepted persisted addresses %q", addresses)
		}
	}
}

func TestPrepareMacvlanParent(t *testing.T) {
	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent"}}
	failure := errors.New("parent unavailable")
	for _, tc := range []struct {
		name    string
		links   parentNetlink
		subnet  string
		wantErr bool
	}{
		{"explicit subnet on down parent", parentNetlink{parent: parent}, "192.0.2.0/24", false},
		{"lookup error", parentNetlink{lookupErr: failure}, "192.0.2.0/24", true},
		{"address read error", parentNetlink{parent: parent, err: failure}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &clabtypes.MgmtNet{
				Driver:        "macvlan",
				MacvlanParent: "parent",
				IPv4Subnet:    tc.subnet,
			}
			got, err := PrepareMacvlanParent(m, tc.links)
			if tc.wantErr {
				if !errors.Is(err, failure) {
					t.Fatalf("lost parent error: %v", err)
				}
			} else if err != nil || got != parent {
				t.Fatalf("parent lookup: %v, %v", got, err)
			}
		})
	}
}

func TestEnsureMacvlanHostErrorContext(t *testing.T) {
	m := &clabtypes.MgmtNet{
		Network: "test", Driver: "macvlan", MacvlanParent: "parent",
		IPv4Subnet: "192.0.2.0/24",
	}
	failure := errors.New("lookup failed")
	host := clabutils.MacvlanHost{Links: parentNetlink{lookupErr: failure}}
	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent"}}
	addresses := []netip.Addr{netip.MustParseAddr("192.0.2.129")}
	if err := EnsureMacvlanHost(
		context.Background(),
		m,
		host,
		parent,
		"network-id",
		addresses,
	); !errors.Is(err, failure) ||
		!strings.Contains(err.Error(), m.Network) {
		t.Fatalf("missing network context or underlying error: %v", err)
	}
}
