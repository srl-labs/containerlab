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
			if family := netlinkFamily(tc.ip); family != tc.family {
				t.Fatalf("netlinkFamily(%s) = %d; want %d", tc.ip, family, tc.family)
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

func TestMacvlanHostName(t *testing.T) {
	a := macvlanHostName(strings.Repeat("a", 64))
	b := macvlanHostName(strings.Repeat("b", 64))
	if len(a) > 15 || a == b || a != macvlanHostName(strings.Repeat("a", 64)) {
		t.Fatalf("invalid names: %q, %q", a, b)
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

func (f *hostNetlink) LinkList() ([]netlink.Link, error) {
	if err := f.call("list-links"); err != nil {
		return nil, err
	}
	if f.link == nil {
		return nil, nil
	}
	return []netlink.Link{f.link}, nil
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
		if (address.IP.To4() != nil) == (family == netlink.FAMILY_V4) &&
			(link == nil || address.LinkIndex == link.Attrs().Index) {
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

func (f *hostNetlink) RouteList(_ netlink.Link, family int) ([]netlink.Route, error) {
	var routes []netlink.Route
	for _, route := range f.routes {
		if route.Dst == nil || (route.Dst.IP.To4() != nil) == (family == netlink.FAMILY_V4) {
			routes = append(routes, route)
		}
	}
	return routes, f.call("list-routes")
}

func (f *hostNetlink) RouteAdd(route *netlink.Route) error {
	err := f.call("route")
	if err == nil || f.routeRace {
		f.routes = append(f.routes, *route)
	}
	return err
}

func (f *hostNetlink) RouteDel(route *netlink.Route) error {
	if err := f.call("delete-route"); err != nil {
		return err
	}
	for i := range f.routes {
		if f.routes[i].Dst.String() == route.Dst.String() &&
			f.routes[i].LinkIndex == route.LinkIndex {
			f.routes = append(f.routes[:i], f.routes[i+1:]...)
			return nil
		}
	}
	return unix.ESRCH
}

func hostParent() netlink.Link {
	return &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "parent", Index: 1, MTU: 1400}}
}

func ensureTestHost(ctx context.Context, host MacvlanHost, ipv6 bool) error {
	ip := "192.0.2.129"
	if ipv6 {
		ip = "2001:db8::129"
	}
	return host.Ensure(ctx, "network-id", hostParent(), []netip.Addr{netip.MustParseAddr(ip)})
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
			host := MacvlanHost{Links: f}
			for range 2 {
				if err := ensureTestHost(context.Background(), host, ipv6); err != nil {
					t.Fatal(err)
				}
				source, destination := "192.0.2.129", "192.0.2.140"
				if ipv6 {
					source, destination = "2001:db8::129", "2001:db8::140"
				}
				if err := host.SyncRoutes(
					"network-id",
					[]netip.Addr{netip.MustParseAddr(source)},
					[]netip.Addr{netip.MustParseAddr(destination)},
				); err != nil {
					t.Fatal(err)
				}
			}
			if f.calls["create"] != 1 || f.calls["address"] != 1 || f.calls["route"] != 1 {
				t.Fatalf("reuse recreated resources: %+v", f.calls)
			}
			address, route := f.addresses[0], f.routes[0]
			bits, width := address.Mask.Size()
			if bits != width || f.link.Attrs().MTU != 1400 || route.LinkIndex != 2 ||
				!route.Src.Equal(address.IP) {
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
			source, destination := "192.0.2.129", "192.0.2.140"
			if ipv6 {
				source, destination = "2001:db8::129", "2001:db8::140"
			}
			if err := host.SyncRoutes(
				"network-id",
				[]netip.Addr{netip.MustParseAddr(source)},
				[]netip.Addr{netip.MustParseAddr(destination)},
			); err != nil {
				t.Fatal(err)
			}
			if len(f.routes) != 1 || f.link.Attrs().Flags&net.FlagUp == 0 {
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
	for _, operation := range []string{"lookup", "create", "alias", "up", "list-addresses", "address"} {
		t.Run(operation, func(t *testing.T) {
			f := newHostNetlink()
			f.failures[operation] = unix.EPERM
			host := MacvlanHost{Links: f}
			if err := ensureTestHost(
				context.Background(),
				host,
				false,
			); !errors.Is(
				err,
				unix.EPERM,
			) {
				t.Fatalf("lost operation error: %v", err)
			}
			if f.calls["delete"] != 0 {
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
	for _, operation := range []string{"address"} {
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

func TestMacvlanHostSyncRoutes(t *testing.T) {
	f := newHostNetlink()
	host := MacvlanHost{Links: f}
	sources := []netip.Addr{
		netip.MustParseAddr("192.0.2.129"),
		netip.MustParseAddr("2001:db8::129"),
	}
	if err := host.Ensure(context.Background(), "network-id", hostParent(), sources); err != nil {
		t.Fatal(err)
	}
	destinations := []netip.Addr{
		netip.MustParseAddr("192.0.2.140"),
		netip.MustParseAddr("2001:db8::140"),
	}
	if err := host.SyncRoutes("network-id", sources, destinations); err != nil {
		t.Fatal(err)
	}
	if len(f.routes) != 2 {
		t.Fatalf("routes = %v", f.routes)
	}
	for _, route := range f.routes {
		bits, width := route.Dst.Mask.Size()
		if bits != width || route.LinkIndex != f.link.Attrs().Index ||
			route.Protocol != unix.RTPROT_STATIC {
			t.Fatalf("invalid host route: %v", route)
		}
	}
	_, foreignDestination, err := net.ParseCIDR("192.0.2.150/32")
	if err != nil {
		t.Fatal(err)
	}
	f.routes = append(f.routes, netlink.Route{
		LinkIndex: f.link.Attrs().Index,
		Dst:       foreignDestination,
		Src:       net.ParseIP("192.0.2.130"),
		Scope:     netlink.SCOPE_LINK,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  unix.RTPROT_STATIC,
		Type:      unix.RTN_UNICAST,
	})
	if err := host.SyncRoutes("network-id", sources, destinations[1:]); err != nil {
		t.Fatal(err)
	}
	if len(f.routes) != 1 {
		t.Fatalf("stale route was not pruned: %v", f.routes)
	}
	for _, route := range f.routes {
		if route.Dst.IP.To4() != nil {
			t.Fatalf("stale IPv4 route remains: %v", f.routes)
		}
	}
	if err := host.SyncRoutes(
		"network-id",
		sources[1:],
		[]netip.Addr{destinations[0]},
	); err == nil {
		t.Fatal("missing address-family source accepted")
	}
}

func TestMacvlanHostSyncRoutesListsTableOncePerFamily(t *testing.T) {
	f := newHostNetlink()
	host := MacvlanHost{Links: f}
	source := netip.MustParseAddr("192.0.2.129")
	if err := host.Ensure(
		context.Background(),
		"network-id",
		hostParent(),
		[]netip.Addr{source},
	); err != nil {
		t.Fatal(err)
	}
	destinations := make([]netip.Addr, 32)
	for i := range destinations {
		destinations[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 10)})
	}
	if err := host.SyncRoutes("network-id", []netip.Addr{source}, destinations); err != nil {
		t.Fatal(err)
	}
	if f.calls["list-routes"] != 2 {
		t.Fatalf(
			"listed routing table %d times for %d destinations",
			f.calls["list-routes"],
			len(destinations),
		)
	}
}

func TestMacvlanHostSyncRoutesReconcilesMainTable(t *testing.T) {
	source := netip.MustParseAddr("192.0.2.129")
	destination := netip.MustParseAddr("192.0.2.140")

	t.Run("stale source", func(t *testing.T) {
		f := newHostNetlink()
		host := MacvlanHost{Links: f}
		if err := ensureTestHost(context.Background(), host, false); err != nil {
			t.Fatal(err)
		}
		if err := host.SyncRoutes(
			"network-id",
			[]netip.Addr{source},
			[]netip.Addr{destination},
		); err != nil {
			t.Fatal(err)
		}
		f.routes[0].Src = net.ParseIP("192.0.2.130")
		if err := host.SyncRoutes(
			"network-id",
			[]netip.Addr{source},
			[]netip.Addr{destination},
		); err != nil {
			t.Fatal(err)
		}
		if len(f.routes) != 1 || !f.routes[0].Src.Equal(net.IP(source.AsSlice())) ||
			f.calls["delete-route"] != 1 {
			t.Fatalf("stale source was not replaced: %v, calls=%v", f.routes, f.calls)
		}
	})

	t.Run("policy table", func(t *testing.T) {
		f := newHostNetlink()
		host := MacvlanHost{Links: f}
		if err := ensureTestHost(context.Background(), host, false); err != nil {
			t.Fatal(err)
		}
		f.routes = append(f.routes, netlink.Route{
			LinkIndex: f.link.Attrs().Index,
			Dst: &net.IPNet{
				IP:   net.IP(destination.AsSlice()),
				Mask: net.CIDRMask(destination.BitLen(), destination.BitLen()),
			},
			Src:      net.IP(source.AsSlice()),
			Scope:    netlink.SCOPE_LINK,
			Table:    100,
			Protocol: unix.RTPROT_STATIC,
			Type:     unix.RTN_UNICAST,
		})
		if err := host.SyncRoutes(
			"network-id",
			[]netip.Addr{source},
			[]netip.Addr{destination},
		); err != nil {
			t.Fatal(err)
		}
		if len(f.routes) != 2 || f.routes[0].Table != 100 ||
			f.routes[1].Table != unix.RT_TABLE_MAIN {
			t.Fatalf("policy route suppressed main-table route: %v", f.routes)
		}
	})
}

func TestMacvlanHostRefusesForeignState(t *testing.T) {
	for _, scenario := range []string{"type", "parent", "mode", "host-address", "extra-address"} {
		t.Run(scenario, func(t *testing.T) {
			f := newHostNetlink()
			host := MacvlanHost{Links: f}
			if err := ensureTestHost(context.Background(), host, false); err != nil {
				t.Fatal(err)
			}
			switch scenario {
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
			}
			if err := ensureTestHost(context.Background(), host, false); err == nil {
				t.Fatal("foreign configuration accepted")
			}
			if f.calls["create"] != 1 || f.calls["address"] != 1 || f.calls["route"] != 0 ||
				f.calls["delete"] != 0 {
				t.Fatalf("foreign state modified: %+v", f.calls)
			}
			if scenario == "type" {
				if err := host.Remove("network-id"); err == nil || f.calls["delete"] != 0 {
					t.Fatal("foreign interface removed")
				}
			}
		})
	}
}

func TestMacvlanHostAdoptsUnaliasedIdentity(t *testing.T) {
	f := newHostNetlink()
	host := MacvlanHost{Links: f}
	if err := ensureTestHost(context.Background(), host, false); err != nil {
		t.Fatal(err)
	}
	f.link.Attrs().Alias = ""
	if err := ensureTestHost(context.Background(), host, false); err != nil {
		t.Fatal(err)
	}
	if f.link.Attrs().Alias != macvlanHostAlias("network-id") || f.calls["create"] != 1 {
		t.Fatalf(
			"unaliased interface was not adopted: alias=%q calls=%v",
			f.link.Attrs().Alias,
			f.calls,
		)
	}
	f.link.Attrs().Alias = ""
	if err := host.Remove("network-id"); err != nil || f.calls["delete"] != 1 {
		t.Fatalf("unaliased owned interface was not removed: %v calls=%v", err, f.calls)
	}
}

func TestMacvlanHostIPv6Sysctl(t *testing.T) {
	f := newHostNetlink()
	var names []string
	orig := macvlanIPv6Sysctl
	t.Cleanup(func() { macvlanIPv6Sysctl = orig })
	macvlanIPv6Sysctl = func(name string) error {
		names = append(names, name)
		return nil
	}
	if err := ensureTestHost(context.Background(), MacvlanHost{Links: f}, false); err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != macvlanHostName("network-id") {
		t.Fatalf("sysctl ifaces = %v", names)
	}
}

func TestMacvlanHostChecksumFill(t *testing.T) {
	f := newHostNetlink()
	var ops []string
	host := MacvlanHost{
		Links: f,
		TCPChecksumFill: func(name string, family int, enable bool) error {
			op := "off"
			if enable {
				op = "on"
			}
			ops = append(ops, name+":"+strconv.Itoa(family)+":"+op)
			return nil
		},
	}
	if err := ensureTestHost(context.Background(), host, false); err != nil {
		t.Fatal(err)
	}
	if err := host.Remove("network-id"); err != nil {
		t.Fatal(err)
	}
	want := macvlanHostName("network-id")
	if len(ops) != 3 ||
		ops[0] != want+":"+strconv.Itoa(netlink.FAMILY_V4)+":on" ||
		ops[1] != want+":"+strconv.Itoa(netlink.FAMILY_V4)+":off" ||
		ops[2] != want+":"+strconv.Itoa(netlink.FAMILY_V6)+":off" {
		t.Fatalf("checksum fill ops = %v", ops)
	}
}

func TestMacvlanHostChecksumFillFailure(t *testing.T) {
	wantErr := errors.New("checksum unavailable")
	host := MacvlanHost{
		Links: newHostNetlink(),
		TCPChecksumFill: func(string, int, bool) error {
			return wantErr
		},
	}
	if err := ensureTestHost(context.Background(), host, false); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v; want %v", err, wantErr)
	}
}

func TestMacvlanHostRemoveOrphans(t *testing.T) {
	f := newHostNetlink()
	host := MacvlanHost{Links: f}
	if err := ensureTestHost(context.Background(), host, false); err != nil {
		t.Fatal(err)
	}
	if err := host.RemoveOrphans(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if f.calls["delete"] != 0 {
		t.Fatal("live network interface was deleted")
	}
	if err := host.RemoveOrphans(func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if f.calls["delete"] != 1 || f.link != nil {
		t.Fatalf("orphan was not deleted: calls=%v link=%v", f.calls, f.link)
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
			err = (MacvlanHost{Links: f}).waitAddressReady(
				ctx,
				&netlink.Macvlan{LinkAttrs: netlink.LinkAttrs{Index: 2}},
				netip.MustParseAddr("2001:db8::129"),
			)
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
				if err == nil ||
					!strings.Contains(err.Error(), "duplicate address detection failed") {
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
	if err := host.Ensure(
		context.Background(),
		"",
		hostParent(),
		[]netip.Addr{netip.MustParseAddr("192.0.2.129")},
	); err == nil {
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
