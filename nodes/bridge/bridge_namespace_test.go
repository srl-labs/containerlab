package bridge

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type namespaceTestNode struct {
	clabnodes.Node
	target netns.NsHandle
	move   bool
}

func (n namespaceTestNode) AddLinkToContainer(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	if n.move {
		if err := netlink.LinkSetNsFd(link, int(n.target)); err != nil {
			return err
		}
	}
	if err := netns.Set(n.target); err != nil {
		return err
	}
	return f(nil)
}

func TestAddLinkToContainerNamespace(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to create network namespaces")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	defer func() { _ = netns.Set(original) }()
	target, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := netlink.LinkAdd(&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "brtest"}}); err != nil {
		t.Fatal(err)
	}
	br, err := netlink.LinkByName("brtest")
	if err != nil {
		t.Fatal(err)
	}
	source, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "moved"}}); err != nil {
		t.Fatal(err)
	}
	link, err := netlink.LinkByName("moved")
	if err != nil {
		t.Fatal(err)
	}
	// Both namespaces start with a single device after loopback, so the
	// source index should collide with the destination bridge's index.
	if link.Attrs().Index != br.Attrs().Index {
		t.Fatalf("source index %d does not collide with bridge index %d", link.Attrs().Index, br.Attrs().Index)
	}
	b := &bridge{DefaultNode: clabnodes.DefaultNode{Cfg: &clabtypes.NodeConfig{ShortName: "brtest|node", NetworkMode: "container:node"}}, nodesMap: map[string]clabnodes.Node{"node": namespaceTestNode{target: target, move: true}}}
	called := false
	if err := b.addLinkToContainerNamespace(context.Background(), link, func(ns.NetNS) error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	moved, err := netlink.LinkByName("moved")
	if err != nil {
		t.Fatal(err)
	}
	if !called || moved.Attrs().MasterIndex != br.Attrs().Index {
		t.Fatalf("callback called=%v, master=%d, want %d", called, moved.Attrs().MasterIndex, br.Attrs().Index)
	}
	missing := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "missing"}}
	b.nodesMap["node"] = namespaceTestNode{target: target}
	err = b.addLinkToContainerNamespace(context.Background(), missing, func(ns.NetNS) error { t.Fatal("unexpected callback"); return nil })
	if err == nil || !strings.Contains(err.Error(), "failed to find link \"missing\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}
