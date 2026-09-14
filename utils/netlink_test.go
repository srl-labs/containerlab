package utils

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestNetlinkFamily(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ip     netip.Addr
		family int
	}{
		{name: "IPv4", ip: netip.MustParseAddr("192.0.2.1"), family: netlink.FAMILY_V4},
		{name: "IPv6", ip: netip.MustParseAddr("2001:db8::1"), family: netlink.FAMILY_V6},
		{name: "invalid", family: netlink.FAMILY_ALL},
		{name: "mapped IPv4", ip: netip.MustParseAddr("::ffff:192.0.2.1"), family: netlink.FAMILY_V6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if family := NetlinkFamily(tc.ip); family != tc.family {
				t.Fatalf("NetlinkFamily(%s) = %d; want %d", tc.ip, family, tc.family)
			}
		})
	}
}

func TestSanitizeInterfaceName(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"sanitize-test-original": {
			input: "eth0",
			want:  "eth0",
		},
		"sanitize-test-xrd": {
			input: "Gi0-0-0-0",
			want:  "Gi0-0-0-0",
		},
		"sanitize-test-c8000": {
			input: "Hu0_0_0_1",
			want:  "Hu0_0_0_1",
		},
		"sanitize-test-asa": {
			input: "GigabitEthernet 0/0",
			want:  "GigabitEthernet-0-0",
		},
		"sanitize-test-junos": {
			input: "ge-0/0/0",
			want:  "ge-0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := SanitizeInterfaceName(tt.input)
			if got != tt.want {
				t.Errorf("got wrong sanitized interface name %q, want %q", got, tt.want)
			}
		})
	}
}

type hostNetlink struct {
	link                   netlink.Link
	addresses              []netlink.Addr
	routes                 []netlink.Route
	failures               map[string]error
	calls                  map[string]int
	addressRace, routeRace bool
	onCreate               func(netlink.Link)
}

func newHostNetlink() *hostNetlink {
	return &hostNetlink{failures: map[string]error{}, calls: map[string]int{}}
}

func (f *hostNetlink) call(operation string) error {
	f.calls[operation]++
	return f.failures[operation]
}
func (f *hostNetlink) LinkByName(string) (netlink.Link, error) {
	if err := f.call("lookup"); err != nil {
		return nil, err
	}
	if f.link == nil {
		return nil, netlink.LinkNotFoundError{}
	}
	return f.link, nil
}
func (f *hostNetlink) LinkAdd(link netlink.Link) error {
	if err := f.call("create"); err != nil {
		return err
	}
	if f.onCreate != nil {
		f.onCreate(link)
	}
	link.Attrs().Index = 2
	f.link = link
	return nil
}
func (f *hostNetlink) LinkSetAlias(link netlink.Link, alias string) error {
	if err := f.call("alias"); err != nil {
		return err
	}
	link.Attrs().Alias = alias
	return nil
}
func (f *hostNetlink) LinkSetUp(link netlink.Link) error {
	if err := f.call("up"); err != nil {
		return err
	}
	link.Attrs().Flags |= net.FlagUp
	return nil
}
func (f *hostNetlink) LinkDel(netlink.Link) error {
	if err := f.call("delete"); err != nil {
		return err
	}
	f.link, f.addresses, f.routes = nil, nil, nil
	return nil
}
func (f *hostNetlink) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	if err := f.call("list-addresses"); err != nil {
		return nil, err
	}
	var addresses []netlink.Addr
	for _, address := range f.addresses {
		if (address.IP.To4() != nil) == (family == netlink.FAMILY_V4) && (link == nil || address.LinkIndex == link.Attrs().Index) {
			addresses = append(addresses, address)
		}
	}
	return addresses, nil
}
func (f *hostNetlink) AddrAdd(link netlink.Link, address *netlink.Addr) error {
	err := f.call("address")
	if err == nil || f.addressRace {
		copy := *address
		copy.LinkIndex = link.Attrs().Index
		f.addresses = append(f.addresses, copy)
	}
	return err
}
func (f *hostNetlink) RouteList(netlink.Link, int) ([]netlink.Route, error) {
	return f.routes, f.call("list-routes")
}
func (f *hostNetlink) RouteAdd(route *netlink.Route) error {
	err := f.call("route")
	if err == nil || f.routeRace {
		f.routes = append(f.routes, *route)
	}
	return err
}

func hostParent() netlink.Link {
	return &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent", Index: 1, MTU: 1400}}
}

func ensureTestHost(ctx context.Context, host MacvlanHost, ipv6 bool) error {
	ip, route := "192.0.2.129", "192.0.2.128/26"
	if ipv6 {
		ip, route = "2001:db8::129", "2001:db8::/64"
	}
	return host.Ensure(ctx, "network-id", hostParent(), netip.MustParseAddr(ip), netip.MustParsePrefix(route))
}

func TestMacvlanHostLifecycle(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		name := "IPv4"
		if ipv6 {
			name = "IPv6"
		}
		t.Run(name, func(t *testing.T) {
			f := newHostNetlink()
			var createdMAC net.HardwareAddr
			f.onCreate = func(link netlink.Link) {
				mac := link.Attrs().HardwareAddr
				if len(mac) != 6 || mac[0]&3 != 2 {
					t.Fatalf("creation must supply a local unicast MAC: %v", mac)
				}
				if createdMAC != nil && !bytes.Equal(createdMAC, mac) {
					t.Fatalf("MAC changed on recreation: %v -> %v", createdMAC, mac)
				}
				createdMAC = append(net.HardwareAddr(nil), mac...)
			}
			probes := 0
			host := MacvlanHost{Links: f, Probe: func(_ context.Context, attrs *netlink.LinkAttrs, ip netip.Addr) error {
				probes++
				if len(f.addresses) != 0 || len(f.routes) != 0 {
					t.Fatal("address or route committed before probing")
				}
				if !bytes.Equal(attrs.HardwareAddr, createdMAC) || ip.Is6() != ipv6 {
					t.Fatal("probe uses incorrect interface or address")
				}
				return nil
			}}
			for range 2 {
				if err := ensureTestHost(context.Background(), host, ipv6); err != nil {
					t.Fatal(err)
				}
			}
			if f.calls["create"] != 1 || f.calls["address"] != 1 || f.calls["route"] != 1 || probes != 1 {
				t.Fatalf("reuse recreated resources: %+v, probes %d", f.calls, probes)
			}
			address, route := f.addresses[0], f.routes[0]
			bits, width := address.Mask.Size()
			if bits != width || f.link.Attrs().MTU != 1400 || route.LinkIndex != 2 || !route.Src.Equal(address.IP) {
				t.Fatalf("incorrect host configuration: %v %v", address, route)
			}
			wantScope := netlink.SCOPE_LINK
			if ipv6 {
				wantScope = netlink.SCOPE_UNIVERSE
				if address.Flags&unix.IFA_F_NOPREFIXROUTE == 0 {
					t.Fatal("IPv6 address can install an implicit prefix route")
				}
			}
			if route.Scope != wantScope || route.Table != unix.RT_TABLE_MAIN {
				t.Fatalf("incorrect route scope/table: %v", route)
			}
			f.routes = nil
			f.link.Attrs().Flags = 0
			if err := ensureTestHost(context.Background(), host, ipv6); err != nil {
				t.Fatal(err)
			}
			if len(f.routes) != 1 || f.link.Attrs().Flags&net.FlagUp == 0 || probes != 1 {
				t.Fatal("partial interface was not repaired in place")
			}
			if err := host.Remove("network-id"); err != nil {
				t.Fatal(err)
			}
			if err := host.Remove("network-id"); err != nil {
				t.Fatal(err)
			}
			if f.calls["delete"] != 1 {
				t.Fatal("absent interface was deleted again")
			}
			if err := ensureTestHost(context.Background(), host, ipv6); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMacvlanHostSetupFailures(t *testing.T) {
	for _, operation := range []string{"lookup", "create", "alias", "up", "list-addresses", "address", "list-routes", "route"} {
		t.Run(operation, func(t *testing.T) {
			f := newHostNetlink()
			f.failures[operation] = unix.EPERM
			host := MacvlanHost{Links: f}
			if err := ensureTestHost(context.Background(), host, false); !errors.Is(err, unix.EPERM) {
				t.Fatalf("lost operation error: %v", err)
			}
			wantDeletes := 0
			if operation == "alias" {
				wantDeletes = 1
			}
			if f.calls["delete"] != wantDeletes {
				t.Fatalf("unsafe rollback after %s: %+v", operation, f.calls)
			}
			delete(f.failures, operation)
			if err := ensureTestHost(context.Background(), host, false); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
		})
	}
}

func TestMacvlanHostConcurrentReservation(t *testing.T) {
	for _, operation := range []string{"address", "route"} {
		for _, installed := range []bool{false, true} {
			t.Run(operation+"/installed="+strconv.FormatBool(installed), func(t *testing.T) {
				f := newHostNetlink()
				f.failures[operation] = unix.EEXIST
				f.addressRace, f.routeRace = installed, installed
				err := ensureTestHost(context.Background(), MacvlanHost{Links: f}, false)
				if installed && err != nil {
					t.Fatalf("matching concurrent reservation rejected: %v", err)
				}
				if !installed && !errors.Is(err, unix.EEXIST) {
					t.Fatalf("unverified reservation accepted: %v", err)
				}
				if f.calls["delete"] != 0 {
					t.Fatal("concurrent interface removed")
				}
			})
		}
	}
}

func TestMacvlanHostRefusesForeignState(t *testing.T) {
	for _, scenario := range []string{"alias", "type", "parent", "mode", "host-address", "extra-address", "route"} {
		t.Run(scenario, func(t *testing.T) {
			f := newHostNetlink()
			host := MacvlanHost{Links: f}
			if err := ensureTestHost(context.Background(), host, false); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "alias":
				f.link.Attrs().Alias = "foreign"
			case "type":
				f.link = &netlink.Dummy{LinkAttrs: *f.link.Attrs()}
			case "parent":
				f.link.Attrs().ParentIndex = 99
			case "mode":
				f.link.(*netlink.Macvlan).Mode = netlink.MACVLAN_MODE_PRIVATE
			case "host-address":
				address := f.addresses[0]
				address.LinkIndex = 1
				f.addresses = append(f.addresses, address)
			case "extra-address":
				address, err := netlink.ParseAddr("192.0.2.140/32")
				if err != nil {
					t.Fatal(err)
				}
				address.LinkIndex = 2
				f.addresses = append(f.addresses, *address)
			case "route":
				f.routes[0].LinkIndex = 99
			}
			if err := ensureTestHost(context.Background(), host, false); err == nil {
				t.Fatal("foreign configuration accepted")
			}
			if f.calls["create"] != 1 || f.calls["address"] != 1 || f.calls["route"] != 1 || f.calls["delete"] != 0 {
				t.Fatalf("foreign state modified: %+v", f.calls)
			}
			if scenario == "alias" || scenario == "type" {
				if err := host.Remove("network-id"); err == nil || f.calls["delete"] != 0 {
					t.Fatal("foreign interface removed")
				}
			}
		})
	}
}

func TestMacvlanHostIPv6Readiness(t *testing.T) {
	for _, state := range []string{"ready", "tentative", "duplicate", "missing", "read-error"} {
		t.Run(state, func(t *testing.T) {
			f := newHostNetlink()
			address, err := netlink.ParseAddr("2001:db8::129/128")
			if err != nil {
				t.Fatal(err)
			}
			address.LinkIndex = 2
			linkLocal, err := netlink.ParseAddr("fe80::1/64")
			if err != nil {
				t.Fatal(err)
			}
			linkLocal.LinkIndex = 2
			f.addresses = []netlink.Addr{*linkLocal, *address}
			switch state {
			case "tentative":
				f.addresses[1].Flags = unix.IFA_F_TENTATIVE
			case "duplicate":
				f.addresses[1].Flags = unix.IFA_F_DADFAILED
			case "missing":
				f.addresses = f.addresses[:1]
			case "read-error":
				f.failures["list-addresses"] = unix.EPERM
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if state == "tentative" || state == "missing" {
				cancel()
			}
			err = (MacvlanHost{Links: f}).waitAddressReady(ctx, &netlink.Macvlan{LinkAttrs: netlink.LinkAttrs{Index: 2}}, netip.MustParseAddr("2001:db8::129"))
			switch state {
			case "ready":
				if err != nil {
					t.Fatal(err)
				}
			case "tentative", "missing":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("waiting ignored cancellation: %v", err)
				}
			case "duplicate":
				if err == nil || !strings.Contains(err.Error(), "duplicate address detection failed") {
					t.Fatalf("duplicate accepted: %v", err)
				}
			case "read-error":
				if !errors.Is(err, unix.EPERM) {
					t.Fatalf("read error lost: %v", err)
				}
			}
		})
	}
}

func TestMacvlanHostProbeFailure(t *testing.T) {
	f := newHostNetlink()
	failure := errors.New("address is occupied")
	host := MacvlanHost{Links: f, Probe: func(context.Context, *netlink.LinkAttrs, netip.Addr) error { return failure }}
	if err := ensureTestHost(context.Background(), host, false); !errors.Is(err, failure) {
		t.Fatalf("probe failure lost: %v", err)
	}
	if len(f.addresses) != 0 || len(f.routes) != 0 || f.calls["delete"] != 0 {
		t.Fatal("probe failure modified addresses or deleted shared interface")
	}
}

func TestMacvlanHostRemoveErrors(t *testing.T) {
	for _, operation := range []string{"lookup", "delete"} {
		f := newHostNetlink()
		host := MacvlanHost{Links: f}
		if err := ensureTestHost(context.Background(), host, false); err != nil {
			t.Fatal(err)
		}
		f.failures[operation] = unix.EPERM
		if err := host.Remove("network-id"); !errors.Is(err, unix.EPERM) {
			t.Fatalf("%s error lost: %v", operation, err)
		}
	}
	f := newHostNetlink()
	host := MacvlanHost{Links: f}
	if err := host.Remove(""); err == nil || f.calls["lookup"] != 0 {
		t.Fatal("empty ID reached netlink")
	}
	if err := host.Ensure(context.Background(), "", hostParent(), netip.MustParseAddr("192.0.2.129"), netip.MustParsePrefix("192.0.2.128/26")); err == nil {
		t.Fatal("empty network ID accepted")
	}
	if err := ensureTestHost(context.Background(), host, false); err != nil {
		t.Fatal(err)
	}
	f.failures["delete"] = unix.ENODEV
	if err := host.Remove("network-id"); err != nil {
		t.Fatalf("concurrent removal rejected: %v", err)
	}
}
