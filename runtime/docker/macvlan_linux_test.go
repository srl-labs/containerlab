package docker

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
)

// Run in an isolated network namespace with CAP_NET_ADMIN, for example using
// docker run --network none --cap-add NET_ADMIN -e CLAB_TEST_MACVLAN_NETLINK=1 ...
func TestMacvlanHostKernel(t *testing.T) {
	if os.Getenv("CLAB_TEST_MACVLAN_NETLINK") != "1" {
		t.Skip(
			"set CLAB_TEST_MACVLAN_NETLINK=1 in an isolated network namespace with CAP_NET_ADMIN",
		)
	}
	for _, tc := range []struct{ name, ip, prefix string }{
		{"IPv4", "192.0.2.129", "192.0.2.128/26"},
		{"IPv6", "2001:db8::8000:2", "2001:db8::8000:0/97"},
	} {
		t.Run(tc.name, func(t *testing.T) { testMacvlanHostKernel(t, tc.ip, tc.prefix) })
	}
}

func testMacvlanHostKernel(t *testing.T, ipText, prefixText string) {
	ip, prefix := netip.MustParseAddr(ipText), netip.MustParsePrefix(prefixText)
	links := &netlink.Handle{}
	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "cm-test-parent", MTU: 1500}}
	if err := links.LinkAdd(parent); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = links.LinkDel(parent) })
	if err := links.LinkSetUp(parent); err != nil {
		t.Fatal(err)
	}
	parentBefore, err := links.LinkByName(parent.Name)
	if err != nil {
		t.Fatal(err)
	}
	host := macvlanHost{links: links}
	networkID := fmt.Sprintf("kernel-test-%d", os.Getpid())
	t.Cleanup(func() { _ = host.remove(networkID) })
	for range 2 {
		if err := host.ensure(
			context.Background(),
			networkID,
			parentBefore,
			ip,
			prefix,
		); err != nil {
			t.Fatal(err)
		}
	}
	link, err := links.LinkByName(macvlanHostName(networkID))
	if err != nil {
		t.Fatal(err)
	}
	if link.Attrs().Alias != macvlanHostAlias(networkID) ||
		link.Attrs().MTU != parentBefore.Attrs().MTU {
		t.Fatalf("incorrect host link attributes: %+v", link.Attrs())
	}
	addrs, err := links.AddrList(link, macvlanFamily(ip))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, addr := range addrs {
		if addr.IPNet.String() == netip.PrefixFrom(ip, ip.BitLen()).String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("host address missing: %v", addrs)
	}
	routes, err := links.RouteList(link, macvlanFamily(ip))
	if err != nil {
		t.Fatal(err)
	}
	var hostRoute *netlink.Route
	for i := range routes {
		if routes[i].Dst != nil && routes[i].Dst.String() == prefix.String() {
			hostRoute = &routes[i]
		}
	}
	if hostRoute == nil {
		t.Fatalf("host route missing: %v", routes)
	}
	// Simulate a partial setup and verify the same link is repaired in place.
	if err := links.RouteDel(hostRoute); err != nil {
		t.Fatal(err)
	}
	if err := links.LinkSetDown(link); err != nil {
		t.Fatal(err)
	}
	if err := host.ensure(context.Background(), networkID, parentBefore, ip, prefix); err != nil {
		t.Fatal(err)
	}
	if err := host.remove(networkID); err != nil {
		t.Fatal(err)
	}
	if err := host.remove(networkID); err != nil {
		t.Fatal(err)
	}
	routes, err = links.RouteList(nil, macvlanFamily(ip))
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		if route.LinkIndex == link.Attrs().Index {
			t.Fatalf("route leaked after cleanup: %v", route)
		}
	}
	parentAfter, err := links.LinkByName(parent.Name)
	if err != nil || parentAfter.Attrs().Promisc != parentBefore.Attrs().Promisc ||
		parentAfter.Attrs().Flags != parentBefore.Attrs().Flags {
		t.Fatalf("parent state changed: %v, error = %v", parentAfter, err)
	}
}
