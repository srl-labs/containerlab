package ovs

import (
	"context"
	"os"
	"os/exec"
	"testing"

	goOvs "github.com/digitalocean/go-openvswitch/ovs"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
)

func TestOVSDeviceMissing(t *testing.T) {
	t.Parallel()

	if !ovsDeviceMissing(
		`"could not open network device clab-deadbeef (No such device)"`,
	) {
		t.Fatal("expected a missing netdev to be detected")
	}
	if ovsDeviceMissing("[]") {
		t.Fatal("a live interface reports an empty error set")
	}
	if !clabAutoPort.MatchString("clab-1f57ad50") {
		t.Fatal("expected a containerlab auto port name to match")
	}
	if clabAutoPort.MatchString("ixr-dut-30-e1-1") {
		t.Fatal("topology interface names are not auto ports")
	}
	if got := ovsMapValue("ixr-dut-30-e1-1"); got != "ixr-dut-30-e1-1" {
		t.Fatalf("ovsMapValue() = %q", got)
	}
	if got := ovsMapValue("has space"); got != `"has space"` {
		t.Fatalf("ovsMapValue() = %q", got)
	}
}

func TestOVSEndpointPortDoesNotAccumulate(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	if _, err := exec.LookPath("ovs-vsctl"); err != nil {
		t.Skip("ovs-vsctl not found")
	}

	// ixr-dut-30-e1-1 is 15 characters, so it is the OVS port name.
	// ixr-dut-30-e1-10 is 16 and is recorded as a clab-* port instead.
	for _, endpoint := range []string{"ixr-dut-30-e1-1", "ixr-dut-30-e1-10"} {
		t.Run(endpoint, func(t *testing.T) {
			assertReattachKeepsOnePort(t, endpoint)
		})
	}
}

func assertReattachKeepsOnePort(t *testing.T, endpoint string) {
	t.Helper()

	bridge := "br-ovs-leak"
	_ = goOvs.New().VSwitch.DeleteBridge(bridge)
	if err := goOvs.New().VSwitch.AddBridge(bridge); err != nil {
		t.Fatalf("add bridge: %v", err)
	}
	t.Cleanup(func() {
		_ = goOvs.New().VSwitch.DeleteBridge(bridge)
	})

	n := &ovs{}
	if err := n.Init(&clabtypes.NodeConfig{ShortName: bridge}); err != nil {
		t.Fatalf("init: %v", err)
	}
	ep := clablinks.NewEndpointBridge(
		clablinks.NewEndpointGeneric(n, endpoint, nil),
		false,
	)
	if err := n.AddEndpoint(ep); err != nil {
		t.Fatalf("add endpoint: %v", err)
	}

	first, peer := addVethPair(t)
	if err := n.AddLinkToContainer(
		context.Background(),
		first,
		clablinks.SetNameMACAndUpInterface(first, ep),
	); err != nil {
		t.Fatalf("add link: %v", err)
	}
	if err := netlink.LinkDel(peer); err != nil {
		t.Fatalf("delete peer: %v", err)
	}
	if ports := dataPorts(t, bridge); len(ports) != 1 {
		t.Fatalf("peer delete left %v, want the stale port", ports)
	}

	second, secondPeer := addVethPair(t)
	t.Cleanup(func() { _ = netlink.LinkDel(secondPeer) })
	if err := n.AddLinkToContainer(
		context.Background(),
		second,
		clablinks.SetNameMACAndUpInterface(second, ep),
	); err != nil {
		t.Fatalf("reattach: %v", err)
	}
	if ports := dataPorts(t, bridge); len(ports) != 1 {
		t.Fatalf("ports after reattach = %v, want 1", ports)
	}

	if err := n.Delete(context.Background()); err != nil {
		t.Fatalf("delete node: %v", err)
	}
	if ports := dataPorts(t, bridge); len(ports) != 0 {
		t.Fatalf("ports after node delete = %v", ports)
	}
}

func addVethPair(t *testing.T) (netlink.Link, netlink.Link) {
	t.Helper()

	name := clablinks.NewEndpointGeneric(nil, "x", nil).GetRandIfaceName()
	peerName := name + "p"
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add veth: %v", err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup veth: %v", err)
	}
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		t.Fatalf("lookup peer: %v", err)
	}

	return link, peer
}

func dataPorts(t *testing.T, bridge string) []string {
	t.Helper()

	ports, err := goOvs.New().VSwitch.ListPorts(bridge)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	var data []string
	for _, port := range ports {
		if port != bridge {
			data = append(data, port)
		}
	}

	return data
}
