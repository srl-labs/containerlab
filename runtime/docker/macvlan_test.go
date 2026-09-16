package docker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	networkapi "github.com/docker/docker/api/types/network"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type fakeMacvlanNetlink struct {
	links                                      map[string]netlink.Link
	addrs                                      []netlink.Addr
	routes                                     []netlink.Route
	adds, deletes, ups, addrAdds, routeAdds    int
	lookupErr, addrErr, routeErr, routeListErr error
	aliasErr                                   error
	concurrent                                 bool
	beforeDelete                               func()
}

func newFakeMacvlanNetlink() *fakeMacvlanNetlink {
	return &fakeMacvlanNetlink{links: map[string]netlink.Link{
		"eth0": &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 1, MTU: 1500, Flags: net.FlagUp},
		},
	}}
}

func (f *fakeMacvlanNetlink) LinkByName(name string) (netlink.Link, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if link, ok := f.links[name]; ok {
		return link, nil
	}
	return nil, netlink.LinkNotFoundError{}
}

func (f *fakeMacvlanNetlink) LinkList() ([]netlink.Link, error) {
	links := make([]netlink.Link, 0, len(f.links))
	for _, link := range f.links {
		links = append(links, link)
	}
	return links, nil
}

func (f *fakeMacvlanNetlink) LinkAdd(link netlink.Link) error {
	f.adds++
	link.Attrs().Index = 2
	f.links[link.Attrs().Name] = link
	if f.concurrent {
		// Simulate a competing deploy that has already marked its new link.
		link.Attrs().Alias = testMacvlanHostAlias("network-id")
		return unix.EEXIST
	}
	return nil
}

func (f *fakeMacvlanNetlink) LinkDel(link netlink.Link) error {
	if f.beforeDelete != nil {
		f.beforeDelete()
	}
	f.deletes++
	delete(f.links, link.Attrs().Name)
	f.addrs, f.routes = nil, nil
	return nil
}

func (f *fakeMacvlanNetlink) LinkSetUp(link netlink.Link) error {
	f.ups++
	link.Attrs().Flags |= net.FlagUp
	return nil
}

func (f *fakeMacvlanNetlink) LinkSetAlias(link netlink.Link, alias string) error {
	if f.aliasErr != nil {
		return f.aliasErr
	}
	link.Attrs().Alias = alias
	return nil
}

func (f *fakeMacvlanNetlink) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	var addrs []netlink.Addr
	for _, addr := range f.addrs {
		if (addr.IP.To4() != nil) != (family == netlink.FAMILY_V4) {
			continue
		}
		if link == nil || addr.LinkIndex == link.Attrs().Index {
			addrs = append(addrs, addr)
		}
	}
	return addrs, nil
}

func (f *fakeMacvlanNetlink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	f.addrAdds++
	if f.addrErr != nil {
		return f.addrErr
	}
	copy := *addr
	copy.LinkIndex = link.Attrs().Index
	f.addrs = append(f.addrs, copy)
	if f.concurrent {
		return unix.EEXIST
	}
	return nil
}

func (f *fakeMacvlanNetlink) RouteList(_ netlink.Link, family int) ([]netlink.Route, error) {
	var routes []netlink.Route
	for _, route := range f.routes {
		if route.Dst == nil || (route.Dst.IP.To4() != nil) == (family == netlink.FAMILY_V4) {
			routes = append(routes, route)
		}
	}
	return routes, f.routeListErr
}

func (f *fakeMacvlanNetlink) RouteAdd(route *netlink.Route) error {
	f.routeAdds++
	if f.routeErr != nil {
		return f.routeErr
	}
	f.routes = append(f.routes, *route)
	if f.concurrent {
		return unix.EEXIST
	}
	return nil
}

func (f *fakeMacvlanNetlink) RouteDel(route *netlink.Route) error {
	for i := range f.routes {
		if f.routes[i].Dst.String() == route.Dst.String() &&
			f.routes[i].LinkIndex == route.LinkIndex {
			f.routes = append(f.routes[:i], f.routes[i+1:]...)
			return nil
		}
	}
	return unix.ESRCH
}

const testMacvlanAuxIPv4 = "192.0.2.129"

func testMacvlanHostName(networkID string) string {
	hash := sha256.Sum256([]byte(networkID))
	return fmt.Sprintf("cm-%x", hash[:6])
}

func testMacvlanHostAlias(networkID string) string {
	return "containerlab:macvlan:" + networkID
}

func testMacvlanConfig() *clabtypes.MgmtNet {
	return &clabtypes.MgmtNet{
		// Fake netlink cannot support wire probes.
		IPAM: clabtypes.MgmtIPAM{
			DAD: new(false),
		},
		Network:       "macvlan-test",
		Driver:        "macvlan",
		MacvlanParent: "eth0",
		IPv4Subnet:    "192.0.2.0/24",
		IPv4Gw:        "192.0.2.1",
		IPv4Range:     "192.0.2.128/26",
	}
}

func TestMacvlanHostRepairAndReuse(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		f := newFakeMacvlanNetlink()
		f.concurrent = concurrent
		host := clabutils.MacvlanHost{Links: f}
		ip := netip.MustParseAddr(testMacvlanAuxIPv4)
		nodeIP := netip.MustParseAddr("192.0.2.140")
		for range 2 {
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{ip},
			); err != nil {
				t.Fatal(err)
			}
			if err := host.SyncRoutes(
				"network-id",
				[]netip.Addr{ip},
				[]netip.Addr{nodeIP},
			); err != nil {
				t.Fatal(err)
			}
		}
		if f.adds != 1 || f.addrAdds != 1 || f.routeAdds != 1 {
			t.Fatalf("reuse recreated resources: %+v", f)
		}
		// Reuse must repair missing routes and a down interface even if the IP exists.
		f.routes = nil
		f.links[testMacvlanHostName("network-id")].Attrs().Flags = 0
		if err := host.Ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			[]netip.Addr{ip},
		); err != nil {
			t.Fatal(err)
		}
		if err := host.SyncRoutes(
			"network-id",
			[]netip.Addr{ip},
			[]netip.Addr{nodeIP},
		); err != nil {
			t.Fatal(err)
		}
		if len(f.routes) != 1 || f.ups != 3 || f.adds != 1 {
			t.Fatalf("incomplete repair: %+v", f)
		}
		if len(f.addrs) != 1 || f.addrs[0].String() != "192.0.2.129/32" {
			t.Fatalf("addresses = %v", f.addrs)
		}
		if f.routes[0].Dst.String() != "192.0.2.140/32" {
			t.Fatalf("route = %v", f.routes[0])
		}
		if err := host.Remove("network-id"); err != nil {
			t.Fatal(err)
		}
		if err := host.Remove("network-id"); err != nil {
			t.Fatal(err)
		}
		if f.deletes != 1 {
			t.Fatalf("deletes = %d", f.deletes)
		}
	}
}

func TestMacvlanHostOwnership(t *testing.T) {
	for _, kind := range []string{"foreign macvlan", "wrong type", "wrong parent", "wrong mode"} {
		t.Run(kind, func(t *testing.T) {
			f := newFakeMacvlanNetlink()
			name := testMacvlanHostName("network-id")
			attrs := netlink.LinkAttrs{
				Name:        name,
				Index:       2,
				ParentIndex: 1,
				Alias:       testMacvlanHostAlias("network-id"),
			}
			link := &netlink.Macvlan{LinkAttrs: attrs, Mode: netlink.MACVLAN_MODE_BRIDGE}
			f.links[name] = link
			switch kind {
			case "foreign macvlan":
				link.Alias = "unrelated"
			case "wrong type":
				f.links[name] = &netlink.Dummy{LinkAttrs: attrs}
			case "wrong parent":
				link.ParentIndex = 99
			case "wrong mode":
				link.Mode = netlink.MACVLAN_MODE_PRIVATE
			}
			ip := netip.MustParseAddr(testMacvlanAuxIPv4)
			host := clabutils.MacvlanHost{Links: f}
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{ip},
			); err == nil {
				t.Fatal("accepted incompatible host interface")
			}
			if f.adds+f.addrAdds+f.routeAdds+f.ups+f.deletes != 0 {
				t.Fatal("modified incompatible host interface")
			}
			if kind == "foreign macvlan" || kind == "wrong type" {
				if err := host.Remove("network-id"); err == nil {
					t.Fatal("removed foreign interface")
				}
				if f.deletes != 0 {
					t.Fatal("deleted foreign interface")
				}
			}
		})
	}
}

func TestMacvlanHostFailuresAreRetryable(t *testing.T) {
	for _, operation := range []string{"lookup", "address"} {
		t.Run(operation, func(t *testing.T) {
			f := newFakeMacvlanNetlink()
			switch operation {
			case "lookup":
				f.lookupErr = unix.EPERM
			case "address":
				f.addrErr = unix.EPERM
			case "route":
				f.routeErr = unix.EPERM
			case "route list":
				f.routeListErr = unix.EPERM
			}
			ip := netip.MustParseAddr(testMacvlanAuxIPv4)
			host := clabutils.MacvlanHost{Links: f}
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{ip},
			); !errors.Is(
				err,
				unix.EPERM,
			) {
				t.Fatalf("error = %v", err)
			}
			if f.deletes != 0 {
				t.Fatal("deleted interface on setup failure")
			}
			f.lookupErr, f.addrErr, f.routeErr, f.routeListErr = nil, nil, nil, nil
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{ip},
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMacvlanRouteConflicts(t *testing.T) {
	for _, tc := range []struct {
		route    string
		conflict bool
	}{
		{"0.0.0.0/0", false}, {"192.0.2.0/24", false}, {"198.51.100.140/32", false},
		{"192.0.2.140/32", true},
	} {
		t.Run(tc.route, func(t *testing.T) {
			f := newFakeMacvlanNetlink()
			_, dst, _ := net.ParseCIDR(tc.route)
			f.routes = []netlink.Route{{Dst: dst, LinkIndex: 1, Table: unix.RT_TABLE_MAIN}}
			ip := netip.MustParseAddr(testMacvlanAuxIPv4)
			host := clabutils.MacvlanHost{Links: f}
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{ip},
			); err != nil {
				t.Fatal(err)
			}
			err := host.SyncRoutes(
				"network-id",
				[]netip.Addr{ip},
				[]netip.Addr{netip.MustParseAddr("192.0.2.140")},
			)
			if (err != nil) != tc.conflict {
				t.Fatalf("ensure() = %v, want conflict %v", err, tc.conflict)
			}
			if tc.conflict && f.routeAdds != 0 {
				t.Fatal("attempted to overwrite a conflicting route")
			}
		})
	}
}

func TestMacvlanHostAddressConflicts(t *testing.T) {
	for _, index := range []int{1, 2} {
		f := newFakeMacvlanNetlink()
		host := clabutils.MacvlanHost{Links: f}
		ip := netip.MustParseAddr(testMacvlanAuxIPv4)
		if err := host.Ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			[]netip.Addr{ip},
		); err != nil {
			t.Fatal(err)
		}
		// Reject an auxiliary address already on the parent, or an unexpected
		// address on the owned interface, without changing either address.
		address := "192.0.2.129/24"
		if index == 2 {
			address = "192.0.2.140/32"
		}
		addr, _ := netlink.ParseAddr(address)
		addr.LinkIndex = index
		f.addrs = append(f.addrs, *addr)
		if err := host.Ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			[]netip.Addr{ip},
		); err == nil {
			t.Fatal("accepted conflicting host address")
		}
		if f.deletes != 0 || f.addrAdds != 1 {
			t.Fatal("changed existing host addresses")
		}
	}
}

func TestMacvlanHostAliasFailure(t *testing.T) {
	f := newFakeMacvlanNetlink()
	f.aliasErr = unix.EPERM
	host := clabutils.MacvlanHost{Links: f}
	ip := netip.MustParseAddr(testMacvlanAuxIPv4)
	if err := host.Ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		[]netip.Addr{ip},
	); !errors.Is(
		err,
		unix.EPERM,
	) {
		t.Fatalf("error = %v", err)
	}
	if f.deletes != 0 || f.addrAdds != 0 {
		t.Fatal("deleted unmarked interface on alias failure")
	}
	f.aliasErr = nil
	if err := host.Ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		[]netip.Addr{ip},
	); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMacvlanNetwork(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	rt.mgmt.IPv6Subnet, rt.mgmt.IPv6Gw = "2001:db8::/64", "2001:db8::1"
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	for range 2 {
		if err := rt.CreateNet(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if fake.creates.Load() != 1 || f.adds != 1 || f.addrAdds != 2 || f.routeAdds != 0 {
		t.Fatalf("network or host resources were not reused: %+v", f)
	}
	ipv4 := fake.info.Labels[clabconstants.MacvlanAuxIPv4]
	ipv6 := fake.info.Labels[clabconstants.MacvlanAuxIPv6]
	if ipv4 == "" || ipv6 == "" ||
		fake.info.IPAM.Config[0].AuxAddress["host"] != ipv4 ||
		fake.info.IPAM.Config[1].AuxAddress["host"] != ipv6 {
		t.Fatalf("dual-stack auxiliary configuration was not persisted: %+v", fake.info)
	}
}

func TestCreateMacvlanNetworkAuxExclusions(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	f := newFakeMacvlanNetlink()
	parentAddress, err := netlink.ParseAddr("192.0.2.3/24")
	if err != nil {
		t.Fatal(err)
	}
	parentAddress.LinkIndex = 1
	f.addrs = []netlink.Addr{*parentAddress}
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	staticAddress := netip.MustParseAddr("192.0.2.2")
	if err := rt.CreateNet(context.Background(), clabruntime.NetworkCreateOptions{
		StaticAddresses: []netip.Addr{staticAddress},
	}); err != nil {
		t.Fatal(err)
	}
	if auxiliary := fake.info.Labels[clabconstants.MacvlanAuxIPv4]; auxiliary != "192.0.2.4" {
		t.Fatalf("auxiliary address = %s, want 192.0.2.4", auxiliary)
	}
}

func TestCreateMacvlanNetworkWithoutAux(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	rt.mgmt.MacvlanAux = new(false)
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.adds != 0 || f.addrAdds != 0 {
		t.Fatalf("created auxiliary host resources: %+v", f)
	}
	if fake.info.Labels[clabconstants.MacvlanAuxIPv4] != "" ||
		fake.info.IPAM.Config[0].AuxAddress["host"] != "" {
		t.Fatalf("persisted auxiliary configuration: %+v", fake.info)
	}
}

func TestCreateMacvlanNetworkNonBridgeSkipsAux(t *testing.T) {
	for _, mode := range []string{"private", "vepa", "passthru"} {
		t.Run(mode, func(t *testing.T) {
			rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
			defer cleanup()
			rt.mgmt = testMacvlanConfig()
			rt.mgmt.MacvlanMode = mode
			f := newFakeMacvlanNetlink()
			rt.macvlanHost = clabutils.MacvlanHost{Links: f}
			if err := rt.CreateNet(context.Background()); err != nil {
				t.Fatal(err)
			}
			if fake.info.Options["macvlan_mode"] != mode || f.adds != 0 || f.addrAdds != 0 {
				t.Fatalf("mode %q created auxiliary host resources: %+v", mode, f)
			}
			if fake.info.Labels[clabconstants.MacvlanAuxIPv4] != "" ||
				fake.info.IPAM.Config[0].AuxAddress["host"] != "" {
				t.Fatalf("mode %q persisted auxiliary configuration: %+v", mode, fake.info)
			}
		})
	}
}

func TestSyncMacvlanHostRoutes(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	rt.mgmt.IPv6Subnet, rt.mgmt.IPv6Gw = "2001:db8::/64", "2001:db8::1"
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.info.Containers = map[string]networkapi.EndpointResource{
		"node1": {IPv4Address: "192.0.2.140/24", IPv6Address: "2001:db8::140/64"},
		"node2": {IPv4Address: "192.0.2.141/24", IPv6Address: "2001:db8::141/64"},
	}
	fake.mu.Unlock()
	if err := rt.SyncMgmtHostRoutes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.routes) != 4 {
		t.Fatalf("routes = %v", f.routes)
	}
	fake.mu.Lock()
	delete(fake.info.Containers, "node1")
	fake.mu.Unlock()
	if err := rt.SyncMgmtHostRoutes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.routes) != 2 {
		t.Fatalf("stale routes were not removed: %v", f.routes)
	}
	for _, route := range f.routes {
		bits, width := route.Dst.Mask.Size()
		if bits != width {
			t.Fatalf("route is not exact: %v", route)
		}
	}
}

func TestCreateMacvlanNetworkRuntimeIPAM(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	rt.mgmt.IPAM.Provider = clabtypes.IPAMProviderRuntime
	rt.macvlanHost = clabutils.MacvlanHost{Links: newFakeMacvlanNetlink()}
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	aux := fake.info.Labels[clabconstants.MacvlanAuxIPv4]
	if aux == "" || fake.info.IPAM.Config[0].AuxAddress["host"] != aux {
		t.Fatalf("runtime IPAM did not reserve an auxiliary address: %+v", fake.info)
	}
}

func TestCreateMacvlanNetworkRemovesOrphanHost(t *testing.T) {
	rt, _, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	orphanID := "dead-network-id"
	if err := rt.managementMacvlanHost().Ensure(
		context.Background(),
		orphanID,
		f.links["eth0"],
		[]netip.Addr{netip.MustParseAddr(testMacvlanAuxIPv4)},
	); err != nil {
		t.Fatal(err)
	}
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.links[testMacvlanHostName(orphanID)]; ok {
		t.Fatal("orphan host interface was not removed")
	}
}

func TestReuseMacvlanNetworkAux(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	auxiliary := []netip.Addr{netip.MustParseAddr(testMacvlanAuxIPv4)}
	opts, err := macvlanNetworkOptions(rt.mgmt, auxiliary)
	if err != nil {
		t.Fatal(err)
	}
	fake.info = networkapi.Inspect{
		ID:      "network-id",
		Driver:  "macvlan",
		Options: opts.Options,
		IPAM:    *opts.IPAM,
		Labels:  opts.Labels,
	}
	fake.created = true
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.creates.Load() != 0 || f.addrAdds != 1 ||
		!f.addrs[0].IP.Equal(net.ParseIP(testMacvlanAuxIPv4)) {
		t.Fatalf("existing auxiliary configuration was not reused: %+v", f)
	}
}

func TestReuseExternalMacvlanNetworkWithoutAux(t *testing.T) {
	m := testMacvlanConfig()
	m.MacvlanAux = new(false)
	opts, err := macvlanNetworkOptions(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := networkapi.Inspect{
		Driver:  "macvlan",
		Options: opts.Options,
		IPAM:    *opts.IPAM,
	}
	if err := validateMacvlanNetwork(&n, opts); err != nil {
		t.Fatalf("external network without auxiliary connectivity was rejected: %v", err)
	}
}

func TestMacvlanNetworkReuseValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*networkapi.Inspect)
	}{
		{"driver", func(n *networkapi.Inspect) { n.Driver = "bridge" }},
		{"parent", func(n *networkapi.Inspect) { n.Options["parent"] = "eth1" }},
		{"mode", func(n *networkapi.Inspect) { n.Options["macvlan_mode"] = "private" }},
		{"subnet", func(n *networkapi.Inspect) { n.IPAM.Config[0].Subnet = "198.51.100.0/24" }},
		{"extra subnet", func(n *networkapi.Inspect) {
			n.IPAM.Config = append(n.IPAM.Config, networkapi.IPAMConfig{Subnet: "fd00::/64"})
		}},
		{"gateway", func(n *networkapi.Inspect) { n.IPAM.Config[0].Gateway = "192.0.2.2" }},
		{"pool", func(n *networkapi.Inspect) { n.IPAM.Config[0].IPRange = "192.0.2.0/25" }},
		{"reservation", func(n *networkapi.Inspect) { n.IPAM.Config[0].AuxAddress = nil }},
		{"legacy network", func(n *networkapi.Inspect) { delete(n.Labels, clabconstants.MacvlanAuxIPv4) }},
		{"auxiliary label", func(n *networkapi.Inspect) { n.Labels[clabconstants.MacvlanAuxIPv4] = "192.0.2.130" }},
		{"external ownership", func(n *networkapi.Inspect) { delete(n.Labels, clabconstants.Containerlab) }},
	} {
		for _, concurrent := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
				defer cleanup()
				rt.mgmt = testMacvlanConfig()
				f := newFakeMacvlanNetlink()
				rt.macvlanHost = clabutils.MacvlanHost{Links: f}
				auxiliary := []netip.Addr{netip.MustParseAddr(testMacvlanAuxIPv4)}
				opts, err := macvlanNetworkOptions(rt.mgmt, auxiliary)
				if err != nil {
					t.Fatal(err)
				}
				fake.info = networkapi.Inspect{
					ID:      "network-id",
					Driver:  "macvlan",
					Options: opts.Options,
					IPAM:    *opts.IPAM,
					Labels:  opts.Labels,
				}
				tc.change(&fake.info)
				fake.created, fake.createConflict = !concurrent, concurrent
				if err := rt.CreateNet(context.Background()); err == nil {
					t.Fatal("reused incompatible network")
				}
				if f.adds != 0 {
					t.Fatal("changed host before validating network")
				}
			})
		}
	}
}

func TestMacvlanNetworkConcurrentCreate(t *testing.T) {
	rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
	defer cleanup()
	rt.mgmt = testMacvlanConfig()
	auxiliary := []netip.Addr{netip.MustParseAddr("192.0.2.2")}
	f := newFakeMacvlanNetlink()
	rt.macvlanHost = clabutils.MacvlanHost{Links: f}
	opts, err := macvlanNetworkOptions(rt.mgmt, auxiliary)
	if err != nil {
		t.Fatal(err)
	}
	fake.info = networkapi.Inspect{
		ID:      "network-id",
		Driver:  "macvlan",
		Options: opts.Options,
		IPAM:    *opts.IPAM,
		Labels:  opts.Labels,
	}
	fake.createConflict = true
	if err := rt.CreateNet(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.adds != 1 {
		t.Fatal("did not configure host after concurrent create")
	}
}

func TestDeleteMacvlanNetwork(t *testing.T) {
	for _, tc := range []struct {
		name               string
		remove, hostDelete int
		wantErr            bool
	}{
		{"owned", 1, 1, false}, {"external", 0, 0, false}, {"shared", 0, 0, false},
		{"keep", 0, 0, false}, {"remove fails", 1, 0, true}, {"missing", 0, 1, false},
		{"concurrent removal", 1, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
			defer cleanup()
			// Deliberately omit mgmt.driver and aux: cleanup uses inspected identity.
			fake.created = true
			fake.info = networkapi.Inspect{
				ID:     "network-id",
				Driver: "macvlan",
				Labels: map[string]string{clabconstants.Containerlab: ""},
			}
			f := newFakeMacvlanNetlink()
			rt.macvlanHost = clabutils.MacvlanHost{Links: f}
			host := rt.managementMacvlanHost()
			if err := host.Ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				[]netip.Addr{netip.MustParseAddr("192.0.2.129")},
			); err != nil {
				t.Fatal(err)
			}
			f.beforeDelete = func() {
				fake.mu.Lock()
				defer fake.mu.Unlock()
				if fake.created {
					t.Error("host removed before Docker network")
				}
			}
			switch tc.name {
			case "external":
				fake.info.Labels = nil
			case "shared":
				fake.info.Containers = map[string]networkapi.EndpointResource{
					"other-lab": {Name: "other-lab"},
				}
			case "keep":
				rt.config.KeepMgmtNet = true
			case "remove fails":
				fake.removeStatus = http.StatusConflict
			case "concurrent removal":
				fake.removeStatus = http.StatusNotFound
				f.beforeDelete = nil // The fake models the already-removed response.
			case "missing":
				fake.created = false
			}
			if err := rt.DeleteNet(context.Background()); (err != nil) != tc.wantErr {
				t.Fatalf("DeleteNet() = %v", err)
			}
			if int(fake.removes.Load()) != tc.remove || f.deletes != tc.hostDelete {
				t.Fatalf(
					"Docker removals = %d, host deletions = %d",
					fake.removes.Load(),
					f.deletes,
				)
			}
		})
	}
}

func TestCreateMacvlanIPv6Host(t *testing.T) {
	for _, dualStack := range []bool{false, true} {
		rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
		t.Cleanup(cleanup)
		rt.mgmt = testMacvlanConfig()
		rt.mgmt.IPv6Subnet, rt.mgmt.IPv6Gw = "2001:db8::/64", "2001:db8::1"
		if !dualStack {
			rt.mgmt.IPv4Subnet, rt.mgmt.IPv4Gw, rt.mgmt.IPv4Range = "", "", ""
		}
		f := newFakeMacvlanNetlink()
		linkLocal, _ := netlink.ParseAddr("fe80::2/64")
		linkLocal.LinkIndex = 2
		f.addrs = []netlink.Addr{*linkLocal}
		rt.macvlanHost = clabutils.MacvlanHost{Links: f}
		for range 2 {
			if err := rt.CreateNet(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		ipv4 := fake.info.Labels[clabconstants.MacvlanAuxIPv4]
		ipv6 := fake.info.Labels[clabconstants.MacvlanAuxIPv6]
		for _, pool := range fake.info.IPAM.Config {
			if strings.Contains(pool.Subnet, ":") {
				if pool.AuxAddress["host"] != ipv6 {
					t.Fatalf("IPv6 reservation = %v", pool)
				}
			} else if pool.AuxAddress["host"] != ipv4 {
				t.Fatalf("IPv4 reservation = %v", pool)
			}
		}
		wantAddresses := 1
		if dualStack {
			wantAddresses = 2
		}
		if f.adds != 1 || f.addrAdds != wantAddresses || f.routeAdds != 0 {
			t.Fatalf("non-idempotent setup: %+v", f)
		}
		if f.addrs[len(f.addrs)-1].IPNet.String() != ipv6+"/128" {
			t.Fatalf("addresses = %v", f.addrs)
		}
		fake.info.IPAM.Config[len(fake.info.IPAM.Config)-1].AuxAddress = nil
		if err := rt.CreateNet(context.Background()); err == nil {
			t.Fatal("reused network without IPv6 reservation")
		}
	}
}

func TestMacvlanIPv6DAD(t *testing.T) {
	for _, flags := range []int{unix.IFA_F_DADFAILED, unix.IFA_F_TENTATIVE} {
		f := newFakeMacvlanNetlink()
		ip := netip.MustParseAddr("2001:db8::2")
		addr, _ := netlink.ParseAddr("2001:db8::2/128")
		addr.LinkIndex, addr.Flags = 2, flags
		f.addrs = []netlink.Addr{*addr}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := (clabutils.MacvlanHost{Links: f}).Ensure(
			ctx,
			"network-id",
			f.links["eth0"],
			[]netip.Addr{ip},
		)
		if err == nil {
			t.Fatal("accepted unusable IPv6 address")
		}
		if flags == unix.IFA_F_DADFAILED && !strings.Contains(err.Error(), "duplicate address") {
			t.Fatal(err)
		}
		if flags == unix.IFA_F_TENTATIVE && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if f.routeAdds != 0 {
			t.Fatal("added route before address became usable")
		}
	}
}

func TestMacvlanIPv6HostRouteOverridesParentRoute(t *testing.T) {
	f := newFakeMacvlanNetlink()
	_, dst, _ := net.ParseCIDR("2001:db8::/64")
	f.routes = []netlink.Route{{Dst: dst, LinkIndex: 1}}
	host := clabutils.MacvlanHost{Links: f}
	source := netip.MustParseAddr("2001:db8::2")
	if err := host.Ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		[]netip.Addr{source},
	); err != nil {
		t.Fatal(err)
	}
	if err := host.SyncRoutes(
		"network-id",
		[]netip.Addr{source},
		[]netip.Addr{netip.MustParseAddr("2001:db8::140")},
	); err != nil || f.routeAdds != 1 {
		t.Fatalf("exact host route did not override parent route: %v", err)
	}
}

func TestMacvlanParentSubnetDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		addresses             []string
		v4, v6, gateway, pool string
		want4, want6, wantErr string
	}{
		{name: "IPv4", addresses: []string{"192.0.2.10/24"}, want4: "192.0.2.0/24"},
		{name: "IPv6", addresses: []string{"2001:db8::10/64", "fe80::1/64"}, want6: "2001:db8::/64"},
		{name: "dual stack", addresses: []string{"192.0.2.10/24", "2001:db8::10/64"}, want4: "192.0.2.0/24", want6: "2001:db8::/64"},
		{name: "same subnet", addresses: []string{"192.0.2.10/24", "192.0.2.20/24"}, want4: "192.0.2.0/24"},
		{name: "IPv4 point to point", addresses: []string{"192.0.2.0/31"}, wantErr: "at least four"},
		{name: "IPv4 host", addresses: []string{"192.0.2.1/32"}, wantErr: "at least four"},
		{name: "IPv6 point to point", addresses: []string{"2001:db8::/127"}, wantErr: "at least four"},
		{name: "IPv6 host", addresses: []string{"2001:db8::1/128"}, wantErr: "at least four"},
		{name: "ambiguous IPv4", addresses: []string{"192.0.2.10/24", "198.51.100.10/24"}, wantErr: "multiple subnets"},
		{name: "ambiguous IPv6", addresses: []string{"2001:db8::10/64", "2001:db8:1::10/64"}, wantErr: "multiple subnets"},
		{name: "no addresses", wantErr: "no usable IP subnet"},
		{name: "link local only", addresses: []string{"fe80::1/64", "169.254.1.1/16"}, wantErr: "no usable IP subnet"},
		{name: "explicit overrides parent", addresses: []string{"192.0.2.1/32", "2001:db8::1/128"}, v4: "198.51.100.0/24", v6: "2001:db8:1::/64", want4: "198.51.100.0/24", want6: "2001:db8:1::/64"},
		{name: "do not infer unrequested IPv6", addresses: []string{"2001:db8::10/64"}, v4: "192.0.2.0/24", want4: "192.0.2.0/24"},
		{name: "do not infer unrequested IPv4", addresses: []string{"192.0.2.10/24"}, v6: "2001:db8::/64", want6: "2001:db8::/64"},
		{name: "wrong gateway", addresses: []string{"192.0.2.10/24"}, gateway: "198.51.100.1", wantErr: "gateway"},
		{name: "wrong pool", addresses: []string{"192.0.2.10/24"}, pool: "198.51.100.0/25", wantErr: "IP range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
			defer cleanup()
			f := newFakeMacvlanNetlink()
			rt.macvlanHost = clabutils.MacvlanHost{Links: f}
			for _, value := range tc.addresses {
				addr, err := netlink.ParseAddr(value)
				if err != nil {
					t.Fatal(err)
				}
				addr.LinkIndex = f.links["eth0"].Attrs().Index
				f.addrs = append(f.addrs, *addr)
			}
			rt.mgmt = &clabtypes.MgmtNet{
				Network:       "macvlan-test",
				Driver:        "macvlan",
				MacvlanParent: "eth0",
				IPv4Subnet:    tc.v4,
				IPv6Subnet:    tc.v6,
				IPv4Gw:        tc.gateway,
				IPv4Range:     tc.pool,
				IPAM:          clabtypes.MgmtIPAM{DAD: new(false)},
			}
			err := rt.CreateNet(context.Background())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %s", err, tc.wantErr)
				}
				if fake.creates.Load() != 0 || f.adds != 0 {
					t.Fatal("invalid discovery mutated networking")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rt.mgmt.IPv4Subnet != tc.want4 || rt.mgmt.IPv6Subnet != tc.want6 {
				t.Fatalf("subnets = %s, %s", rt.mgmt.IPv4Subnet, rt.mgmt.IPv6Subnet)
			}
			if fake.creates.Load() != 1 {
				t.Fatal("network was not created")
			}
			if err := rt.CreateNet(context.Background()); err != nil {
				t.Fatal(err)
			}
			if fake.creates.Load() != 1 {
				t.Fatal("existing network was recreated")
			}
		})
	}
}
