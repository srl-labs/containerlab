package links

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// A link moved into a namespace where its ifindex is taken gets a new index;
// setup must change the moved link, not the interface holding the old index.
func TestSetNameMACAndUpInterfaceAfterIndexChange(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to create network namespaces")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	origin, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	defer func() { _ = netns.Set(origin) }()

	// The node's namespace: interfaces the node created itself take the
	// first free ifindexes after lo.
	target, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	occupied := map[int]string{}
	for _, name := range []string{"occupied1", "occupied2"} {
		if err := netlink.LinkAdd(
			&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}},
		); err != nil {
			t.Fatal(err)
		}
		link, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatal(err)
		}
		occupied[link.Attrs().Index] = name
	}

	// Containerlab's namespace: a fresh one gives the new veth those indexes too.
	source, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := netlink.LinkAdd(
		&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "clab-test-a"}, PeerName: "clab-test-b"},
	); err != nil {
		t.Fatal(err)
	}
	link, err := netlink.LinkByName("clab-test-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := occupied[link.Attrs().Index]; !ok {
		t.Skipf("veth index %d does not collide with %v", link.Attrs().Index, occupied)
	}
	if err := netlink.LinkSetNsFd(link, int(target)); err != nil {
		t.Fatal(err)
	}

	if err := netns.Set(target); err != nil {
		t.Fatal(err)
	}
	ep := NewEndpointVeth(NewEndpointGeneric(newFakeNode("node1"), "eth1", nil))
	if err := SetNameMACAndUpInterface(context.Background(), link, ep)(nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range occupied {
		if _, err := netlink.LinkByName(name); err != nil {
			t.Fatalf("an interface at a colliding index was changed: %s: %v", name, err)
		}
	}
	renamed, err := netlink.LinkByName("eth1")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Type() != "veth" || renamed.Attrs().Flags&net.FlagUp == 0 {
		t.Fatalf(
			"eth1 is %s with flags %v, want the moved veth up",
			renamed.Type(),
			renamed.Attrs().Flags,
		)
	}
}

func TestSetNameMACAndUpInterfaceMissingLink(t *testing.T) {
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "clab-missing"}}
	ep := NewEndpointVeth(NewEndpointGeneric(newFakeNode("node1"), "eth1", nil))
	err := SetNameMACAndUpInterface(context.Background(), link, ep)(nil)
	if err == nil || !strings.Contains(err.Error(), "node1:eth1") || !strings.Contains(err.Error(), "clab-missing") {
		t.Fatalf("missing endpoint or link context in error: %v", err)
	}
}
