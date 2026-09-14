package mgmt

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/bpf"
)

func TestDADFrames(t *testing.T) {
	local := net.HardwareAddr{2, 0, 0, 0, 0, 1}
	peer := net.HardwareAddr{2, 0, 0, 0, 0, 2}
	for _, address := range []string{"192.0.2.2", "2001:db8::2"} {
		t.Run(address, func(t *testing.T) {
			ip := netip.MustParseAddr(address)
			frame, err := dadProbe(peer, ip)
			if err != nil {
				t.Fatal(err)
			}
			if ip.Is4() {
				frame = frame[:42]
			}
			if !dadConflict(frame, local, ip) {
				t.Fatal("missed simultaneous probe")
			}
			if dadConflict(frame, peer, ip) {
				t.Fatal("detected own probe")
			}
			if dadConflict(frame, local, ip.Next()) {
				t.Fatal("detected unrelated address")
			}
			for i := 0; i < len(frame); i++ {
				if dadConflict(frame[:i], local, ip) {
					t.Fatalf("accepted truncated frame at %d", i)
				}
			}
			packet := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
			ethernet := packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
			buffer := gopacket.NewSerializeBuffer()
			options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
			if ip.Is4() {
				arp := packet.Layer(layers.LayerTypeARP).(*layers.ARP)
				arp.Operation = layers.ARPReply
				arp.SourceProtAddress = ip.AsSlice()
				err = gopacket.SerializeLayers(buffer, options, ethernet, arp)
			} else {
				ipv6 := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
				ipv6.SrcIP = ip.AsSlice()
				icmp := &layers.ICMPv6{TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeNeighborAdvertisement, 0)}
				if err := icmp.SetNetworkLayerForChecksum(ipv6); err != nil {
					t.Fatal(err)
				}
				err = gopacket.SerializeLayers(buffer, options, ethernet, ipv6, icmp,
					&layers.ICMPv6NeighborAdvertisement{Flags: 0x20, TargetAddress: ip.AsSlice()})
			}
			if err != nil {
				t.Fatal(err)
			}
			frame = buffer.Bytes()
			if !dadConflict(frame, local, ip) {
				t.Fatal("missed address owner")
			}
			if !ip.Is4() {
				frame[57] ^= 1
				if dadConflict(frame, local, ip) {
					t.Fatal("accepted bad checksum")
				}
			}
		})
	}
}

func TestDADCaptureFilter(t *testing.T) {
	vm, err := bpf.NewVM(dadCaptureFilter())
	if err != nil {
		t.Fatal(err)
	}
	mac := net.HardwareAddr{2, 0, 0, 0, 0, 1}
	arp, err := dadProbe(mac, netip.MustParseAddr("192.0.2.2"))
	if err != nil {
		t.Fatal(err)
	}
	ns, err := dadProbe(mac, netip.MustParseAddr("2001:db8::2"))
	if err != nil {
		t.Fatal(err)
	}
	icmpv6 := func(kind byte) []byte {
		frame := append([]byte(nil), ns...)
		frame[54] = byte(kind)
		return frame
	}
	ipv4 := append([]byte(nil), arp...)
	ipv4[12], ipv4[13] = 0x08, 0x00
	for _, tc := range []struct {
		name   string
		frame  []byte
		accept bool
	}{
		{"ARP", arp, true},
		{"neighbour solicitation", ns, true},
		{"neighbour advertisement", icmpv6(byte(layers.ICMPv6TypeNeighborAdvertisement)), true},
		{"router advertisement", icmpv6(byte(layers.ICMPv6TypeRouterAdvertisement)), false},
		{"echo request", icmpv6(byte(layers.ICMPv6TypeEchoRequest)), false},
		{"IPv4", ipv4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captured, err := vm.Run(tc.frame)
			if err != nil {
				t.Fatal(err)
			}
			if (captured != 0) != tc.accept {
				t.Fatalf("captured %d bytes, accept=%t", captured, tc.accept)
			}
		})
	}
}

func TestDADDropDeltas(t *testing.T) {
	p := &macvlanProbe{}
	if err := p.checkDrops(0); err != nil {
		t.Fatal(err)
	}
	if err := p.checkDrops(2); !errors.Is(err, errDADObservationLost) ||
		!strings.Contains(err.Error(), "dropped 2 packets") {
		t.Fatalf("first drop delta: %v", err)
	}
	if err := p.checkDrops(2); err != nil {
		t.Fatalf("unchanged cumulative drops retriggered loss: %v", err)
	}
	if err := p.checkDrops(3); !errors.Is(err, errDADObservationLost) ||
		!strings.Contains(err.Error(), "dropped 1 packets") {
		t.Fatalf("second drop delta: %v", err)
	}
}

func TestDADSnapshotLookups(t *testing.T) {
	d := &dadChecker{loaded: true}
	for _, prefix := range []string{"192.0.2.0/24", "2001:db8::53/128"} {
		d.local.Add(netip.MustParsePrefix(prefix))
	}
	m := &clabtypes.MgmtNet{Driver: "bridge", Bridge: "does-not-exist"}
	for _, ip := range []string{"192.0.2.123", "2001:db8::53"} {
		err := d.Check(context.Background(), m, netip.MustParseAddr(ip))
		var occupied *occupiedPrefix
		if !errors.As(err, &occupied) || !errors.Is(err, ErrDuplicateAddress) {
			t.Fatalf("missed indexed conflict %s: %v", ip, err)
		}
	}
	if err := d.Check(context.Background(), m, netip.MustParseAddr("198.51.100.2")); err != nil {
		t.Fatal(err)
	}
	if d.wire != nil {
		t.Fatal("non-macvlan check opened a wire probe")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDADSnapshotToleratesMissingBridge(t *testing.T) {
	const bridge = "does-not-exist"
	if _, err := netlink.LinkByName(bridge); !errors.As(err, &netlink.LinkNotFoundError{}) {
		t.Skipf("test bridge unexpectedly exists or lookup failed differently: %v", err)
	}
	d := &dadChecker{}
	if err := d.load(&clabtypes.MgmtNet{
		Driver:     "bridge",
		Bridge:     bridge,
		IPv4Subnet: "192.0.2.0/24",
	}); err != nil {
		t.Fatalf("missing deferred bridge blocked DAD snapshot: %v", err)
	}
}

func TestDADRouteReservations(t *testing.T) {
	route := func(cidr string, scope netlink.Scope, index int) netlink.Route {
		_, dst, _ := net.ParseCIDR(cidr)
		return netlink.Route{Dst: dst, Scope: scope, LinkIndex: index}
	}
	d := &dadChecker{loaded: true}
	d.addRoutes([]netlink.Route{
		route("192.0.2.0/24", netlink.SCOPE_LINK, 7),
		route("198.51.100.0/24", netlink.SCOPE_LINK, 8),
		route("172.16.0.0/12", netlink.SCOPE_LINK, 9),
		route("10.0.0.0/8", netlink.SCOPE_UNIVERSE, 9),
		route("0.0.0.0/0", netlink.SCOPE_UNIVERSE, 9),
	}, 7, netip.MustParsePrefix("172.20.20.0/24"))
	for _, tc := range []struct {
		ip       string
		reserved bool
	}{{"192.0.2.5", false}, {"198.51.100.5", true}, {"172.20.20.5", false}, {"10.0.0.5", false}, {"203.0.113.5", false}} {
		_, got := d.local.Lookup(netip.MustParseAddr(tc.ip))
		if got != tc.reserved {
			t.Fatalf("%s reserved=%t, want %t", tc.ip, got, tc.reserved)
		}
	}

	d = &dadChecker{loaded: true}
	subnet := netip.MustParsePrefix("172.20.20.0/24")
	d.addRoutes([]netlink.Route{
		route(subnet.String(), netlink.SCOPE_LINK, 9),
	}, 7, subnet)
	if _, reserved := d.local.Lookup(netip.MustParseAddr("172.20.20.5")); !reserved {
		t.Fatal("ignored an equal-prefix route on another interface")
	}
}

func TestDADNeighbourCache(t *testing.T) {
	d := &dadChecker{loaded: true}
	entries := []netlink.Neigh{}
	states := []int{
		netlink.NUD_REACHABLE,
		netlink.NUD_PERMANENT,
		netlink.NUD_STALE,
		netlink.NUD_DELAY,
		netlink.NUD_PROBE,
		netlink.NUD_FAILED,
		netlink.NUD_INCOMPLETE,
	}
	for i, state := range states {
		entries = append(
			entries,
			netlink.Neigh{
				LinkIndex:    7,
				IP:           net.ParseIP(fmt.Sprintf("192.0.2.%d", i+1)),
				HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 1},
				State:        state,
			},
		)
	}
	entries = append(
		entries,
		netlink.Neigh{
			LinkIndex:    8,
			IP:           net.ParseIP("2001:db8::1"),
			HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 2},
			State:        netlink.NUD_REACHABLE,
		},
	)
	d.addNeighbours(entries, map[int]bool{7: true})
	for i, state := range states {
		_, got := d.local.Lookup(netip.MustParseAddr(fmt.Sprintf("192.0.2.%d", i+1)))
		want := state == netlink.NUD_REACHABLE || state == netlink.NUD_PERMANENT
		if got != want {
			t.Fatalf("state %d reserved=%t, want %t", state, got, want)
		}
	}
	if _, got := d.local.Lookup(netip.MustParseAddr("2001:db8::1")); got {
		t.Fatal("used neighbour from unrelated interface")
	}
	// A cached hit must return without trying to create a probe interface.
	m := &clabtypes.MgmtNet{Driver: "macvlan", MacvlanParent: "does-not-exist"}
	if err := d.Check(
		context.Background(),
		m,
		netip.MustParseAddr("192.0.2.1"),
	); !errors.Is(
		err,
		ErrDuplicateAddress,
	) {
		t.Fatalf("cache was not checked first: %v", err)
	}
	if d.wire != nil {
		t.Fatal("opened wire probe for a cached occupied address")
	}
}

func dadConflict(frame []byte, mac net.HardwareAddr, ip netip.Addr) bool {
	target, ok := dadConflictAddress(frame, mac)
	return ok && target == ip
}

func TestDADProbeWireFormat(t *testing.T) {
	for _, tc := range []struct{ ip, frame string }{
		{"192.0.2.2", "ffffffffffff0200000000020806000108000604000102000000000200000000000000000000c0000202000000000000000000000000000000000000"},
		{"2001:db8::2", "3333ff00000202000000000286dd6000000000183aff00000000000000000000000000000000ff0200000000000000000001ff00000287004ceb0000000020010db8000000000000000000000002"},
	} {
		t.Run(tc.ip, func(t *testing.T) {
			want, err := hex.DecodeString(tc.frame)
			if err != nil {
				t.Fatal(err)
			}
			got, err := dadProbe(net.HardwareAddr{2, 0, 0, 0, 0, 2}, netip.MustParseAddr(tc.ip))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("probe = %x, want %x", got, want)
			}
		})
	}
}

func TestDADRejectsInvalidIPv6Headers(t *testing.T) {
	local, peer := net.HardwareAddr{2, 0, 0, 0, 0, 1}, net.HardwareAddr{2, 0, 0, 0, 0, 2}
	ip := netip.MustParseAddr("2001:db8::2")
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"version", 14, 0x40},
		{"next header", 20, 17},
		{"hop limit", 21, 254},
		{"code", 55, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame, err := dadProbe(peer, ip)
			if err != nil {
				t.Fatal(err)
			}
			frame[tc.offset] = tc.value
			if dadConflict(frame, local, ip) {
				t.Fatal("invalid header accepted")
			}
		})
	}
}

func TestDADRejectsInvalidARPHeaders(t *testing.T) {
	local, peer := net.HardwareAddr{2, 0, 0, 0, 0, 1}, net.HardwareAddr{2, 0, 0, 0, 0, 2}
	ip := netip.MustParseAddr("192.0.2.2")
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"hardware type", 15, 2},
		{"protocol", 16, 0x86},
		{"hardware address length", 18, 5},
		{"protocol address length", 19, 3},
		{"operation", 21, 3},
		{"ethernet type", 12, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame, err := dadProbe(peer, ip)
			if err != nil {
				t.Fatal(err)
			}
			frame[tc.offset] = tc.value
			if address, ok := dadConflictAddress(frame, local); ok {
				t.Fatalf("invalid ARP claimed %s", address)
			}
		})
	}
}

func TestDADDistinguishesAddressClaimsFromQueries(t *testing.T) {
	local, peer := net.HardwareAddr{2, 0, 0, 0, 0, 1}, net.HardwareAddr{2, 0, 0, 0, 0, 2}
	for _, address := range []string{"192.0.2.2", "2001:db8::2"} {
		t.Run(address, func(t *testing.T) {
			target := netip.MustParseAddr(address)
			sender := target.Next()
			frame, err := dadProbe(peer, target)
			if err != nil {
				t.Fatal(err)
			}
			packet := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
			ethernet := packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
			buffer := gopacket.NewSerializeBuffer()
			options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
			if target.Is4() {
				arp := packet.Layer(layers.LayerTypeARP).(*layers.ARP)
				arp.SourceProtAddress = sender.AsSlice()
				err = gopacket.SerializeLayers(buffer, options, ethernet, arp)
			} else {
				ipv6 := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
				ipv6.SrcIP = sender.AsSlice()
				icmp := packet.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
				if err := icmp.SetNetworkLayerForChecksum(ipv6); err != nil {
					t.Fatal(err)
				}
				solicitation := packet.Layer(layers.LayerTypeICMPv6NeighborSolicitation).(*layers.ICMPv6NeighborSolicitation)
				err = gopacket.SerializeLayers(buffer, options, ethernet, ipv6, icmp, solicitation)
			}
			if err != nil {
				t.Fatal(err)
			}
			claimed, ok := dadConflictAddress(buffer.Bytes(), local)
			if target.Is4() {
				if !ok || claimed != sender {
					t.Fatalf("ARP request claimed %s (%t), want sender %s", claimed, ok, sender)
				}
			} else if ok {
				t.Fatalf("ordinary neighbour solicitation claimed %s", claimed)
			}
		})
	}
}
