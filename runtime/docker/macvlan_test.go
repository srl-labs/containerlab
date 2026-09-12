package docker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	networkapi "github.com/docker/docker/api/types/network"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
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

func (f *fakeMacvlanNetlink) LinkAdd(link netlink.Link) error {
	f.adds++
	link.Attrs().Index = 2
	f.links[link.Attrs().Name] = link
	if f.concurrent {
		// Simulate a competing deploy that has already marked its new link.
		link.Attrs().Alias = macvlanHostAlias("network-id")
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

func testMacvlanConfig() *clabtypes.MgmtNet {
	return &clabtypes.MgmtNet{
		Network: "macvlan-test", Driver: "macvlan", MacvlanParent: "eth0",
		IPv4Subnet: "192.0.2.0/24", IPv4Gw: "192.0.2.1", IPv4Range: "192.0.2.128/26",
		MacvlanAux: "192.0.2.129/26",
	}
}

func TestMacvlanHostRepairAndReuse(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		f := newFakeMacvlanNetlink()
		f.concurrent = concurrent
		host := macvlanHost{links: f}
		ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
		for range 2 {
			if err := host.ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				ip,
				prefix,
			); err != nil {
				t.Fatal(err)
			}
		}
		if f.adds != 1 || f.addrAdds != 1 || f.routeAdds != 1 {
			t.Fatalf("reuse recreated resources: %+v", f)
		}
		// Reuse must repair missing routes and a down interface even if the IP exists.
		f.routes = nil
		f.links[macvlanHostName("network-id")].Attrs().Flags = 0
		if err := host.ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			ip,
			prefix,
		); err != nil {
			t.Fatal(err)
		}
		if len(f.routes) != 1 || f.ups != 3 || f.adds != 1 {
			t.Fatalf("incomplete repair: %+v", f)
		}
		if len(f.addrs) != 1 || f.addrs[0].String() != "192.0.2.129/32" {
			t.Fatalf("addresses = %v", f.addrs)
		}
		if f.routes[0].Dst.String() != "192.0.2.128/26" {
			t.Fatalf("route = %v", f.routes[0])
		}
		if err := host.remove("network-id"); err != nil {
			t.Fatal(err)
		}
		if err := host.remove("network-id"); err != nil {
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
			name := macvlanHostName("network-id")
			attrs := netlink.LinkAttrs{
				Name:        name,
				Index:       2,
				ParentIndex: 1,
				Alias:       macvlanHostAlias("network-id"),
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
			ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
			host := macvlanHost{links: f}
			if err := host.ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				ip,
				prefix,
			); err == nil {
				t.Fatal("accepted incompatible host interface")
			}
			if f.adds+f.addrAdds+f.routeAdds+f.ups+f.deletes != 0 {
				t.Fatal("modified incompatible host interface")
			}
			if kind == "foreign macvlan" || kind == "wrong type" {
				if err := host.remove("network-id"); err == nil {
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
	for _, operation := range []string{"lookup", "address", "route", "route list"} {
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
			ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
			host := macvlanHost{links: f}
			if err := host.ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				ip,
				prefix,
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
			if err := host.ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				ip,
				prefix,
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
		{"0.0.0.0/0", false}, {"192.0.2.0/24", false}, {"198.51.100.0/24", false},
		{"192.0.2.128/26", true}, {"192.0.2.128/27", true},
	} {
		t.Run(tc.route, func(t *testing.T) {
			f := newFakeMacvlanNetlink()
			_, dst, _ := net.ParseCIDR(tc.route)
			f.routes = []netlink.Route{{Dst: dst, LinkIndex: 1}}
			ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
			err := (macvlanHost{links: f}).ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				ip,
				prefix,
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

func TestMacvlanHostName(t *testing.T) {
	a := macvlanHostName(strings.Repeat("a", 64))
	b := macvlanHostName(strings.Repeat("b", 64))
	if len(a) > 15 || a == b || a != macvlanHostName(strings.Repeat("a", 64)) {
		t.Fatalf("invalid names: %q, %q", a, b)
	}
}

func TestMacvlanHostAddressConflicts(t *testing.T) {
	for _, index := range []int{1, 2} {
		f := newFakeMacvlanNetlink()
		host := macvlanHost{links: f}
		ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
		if err := host.ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			ip,
			prefix,
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
		if err := host.ensure(
			context.Background(),
			"network-id",
			f.links["eth0"],
			ip,
			prefix,
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
	host := macvlanHost{links: f}
	ip, prefix, _ := testMacvlanConfig().MacvlanHostAddress()
	if err := host.ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		ip,
		prefix,
	); !errors.Is(
		err,
		unix.EPERM,
	) {
		t.Fatalf("error = %v", err)
	}
	if f.deletes != 1 || len(f.links) != 1 || f.addrAdds != 0 {
		t.Fatal("failed to roll back unmarked new interface")
	}
	f.aliasErr = nil
	if err := host.ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		ip,
		prefix,
	); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMacvlanNetwork(t *testing.T) {
	for _, aux := range []string{"", "192.0.2.129/26"} {
		t.Run(aux, func(t *testing.T) {
			rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
			defer cleanup()
			rt.mgmt = testMacvlanConfig()
			rt.mgmt.MacvlanAux = aux
			rt.mgmt.IPv6Subnet, rt.mgmt.IPv6Gw = "2001:db8::/64", "2001:db8::1"
			f := newFakeMacvlanNetlink()
			rt.macvlanNetlink = f
			for range 2 {
				if err := rt.CreateNet(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if fake.creates.Load() != 1 {
				t.Fatal("recreated Docker network")
			}
			if rt.mgmt.Bridge != "" || rt.mgmt.IPv6Gw != "2001:db8::1" {
				t.Fatalf("mgmt = %+v", rt.mgmt)
			}
			if aux == "" && f.adds != 0 {
				t.Fatal("created unsolicited host interface")
			}
			if aux != "" &&
				(f.adds != 1 || fake.info.IPAM.Config[0].AuxAddress["host"] != "192.0.2.129") {
				t.Fatalf("host address not reserved/configured: %+v", fake.info)
			}
		})
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
		{"gateway", func(n *networkapi.Inspect) { n.IPAM.Config[0].Gateway = "192.0.2.2" }},
		{"pool", func(n *networkapi.Inspect) { n.IPAM.Config[0].IPRange = "192.0.2.0/25" }},
		{"reservation", func(n *networkapi.Inspect) { n.IPAM.Config[0].AuxAddress = nil }},
		{"host route", func(n *networkapi.Inspect) { n.Labels[clabconstants.MacvlanAux] = "192.0.2.129/24" }},
		{"external ownership", func(n *networkapi.Inspect) { delete(n.Labels, clabconstants.Containerlab) }},
	} {
		for _, concurrent := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				rt, fake, cleanup := newFakeDockerRuntime(t, "macvlan-test")
				defer cleanup()
				rt.mgmt = testMacvlanConfig()
				f := newFakeMacvlanNetlink()
				rt.macvlanNetlink = f
				opts, err := macvlanNetworkOptions(rt.mgmt)
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
	f := newFakeMacvlanNetlink()
	rt.macvlanNetlink = f
	opts, err := macvlanNetworkOptions(rt.mgmt)
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
		{"keep", 0, 0, false}, {"remove fails", 1, 0, true}, {"missing", 0, 0, false},
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
			rt.macvlanNetlink = f
			host := rt.macvlanHost()
			if err := host.ensure(
				context.Background(),
				"network-id",
				f.links["eth0"],
				netip.MustParseAddr("192.0.2.129"),
				netip.MustParsePrefix("192.0.2.128/26"),
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
		rt.mgmt.MacvlanAux = "2001:db8::8000:2/97"
		if !dualStack {
			rt.mgmt.IPv4Subnet, rt.mgmt.IPv4Gw, rt.mgmt.IPv4Range = "", "", ""
		}
		f := newFakeMacvlanNetlink()
		linkLocal, _ := netlink.ParseAddr("fe80::2/64")
		linkLocal.LinkIndex = 2
		f.addrs = []netlink.Addr{*linkLocal}
		rt.macvlanNetlink = f
		for range 2 {
			if err := rt.CreateNet(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		for _, pool := range fake.info.IPAM.Config {
			if strings.Contains(pool.Subnet, ":") {
				if pool.AuxAddress["host"] != "2001:db8::8000:2" {
					t.Fatalf("IPv6 reservation = %v", pool)
				}
			} else if len(pool.AuxAddress) != 0 {
				t.Fatalf("IPv6 reserved in IPv4 pool: %v", pool)
			}
		}
		if f.adds != 1 || f.addrAdds != 1 || f.routeAdds != 1 {
			t.Fatalf("non-idempotent setup: %+v", f)
		}
		if f.addrs[1].IPNet.String() != "2001:db8::8000:2/128" {
			t.Fatalf("address = %v", f.addrs[1])
		}
		if f.routes[0].Dst.String() != "2001:db8::8000:0/97" {
			t.Fatalf("route = %v", f.routes[0])
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
		err := (macvlanHost{links: f}).ensure(
			ctx,
			"network-id",
			f.links["eth0"],
			ip,
			netip.MustParsePrefix("2001:db8::/64"),
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

func TestMacvlanIPv6RouteConflict(t *testing.T) {
	f := newFakeMacvlanNetlink()
	_, dst, _ := net.ParseCIDR("2001:db8::/64")
	f.routes = []netlink.Route{{Dst: dst, LinkIndex: 1}}
	err := (macvlanHost{links: f}).ensure(
		context.Background(),
		"network-id",
		f.links["eth0"],
		netip.MustParseAddr("2001:db8::2"),
		netip.MustParsePrefix("2001:db8::/64"),
	)
	if err == nil || f.routeAdds != 0 {
		t.Fatal("accepted conflicting IPv6 route")
	}
}
