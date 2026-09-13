package mgmt

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
)

func TestDADFrames(t *testing.T) {
	local := net.HardwareAddr{2, 0, 0, 0, 0, 1}
	peer := net.HardwareAddr{2, 0, 0, 0, 0, 2}
	for _, address := range []string{"192.0.2.2", "2001:db8::2"} {
		t.Run(address, func(t *testing.T) {
			ip := netip.MustParseAddr(address)
			frame := dadProbe(peer, ip)
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
			if ip.Is4() {
				frame[21] = 2
				copy(frame[28:32], ip.AsSlice())
			} else {
				frame[54] = 136
				frame[58] = 0x20
				copy(frame[22:38], ip.AsSlice())
				frame[56] = 0
				frame[57] = 0
				binary.BigEndian.PutUint16(frame[56:58], dadChecksum(frame[22:54], frame[54:]))
			}
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

func TestDADRouteReservations(t *testing.T) {
	route := func(cidr string, scope netlink.Scope, index int) netlink.Route {
		_, dst, _ := net.ParseCIDR(cidr)
		return netlink.Route{Dst: dst, Scope: scope, LinkIndex: index}
	}
	d := &dadChecker{loaded: true}
	d.addRoutes([]netlink.Route{
		route("192.0.2.0/24", netlink.SCOPE_LINK, 7),
		route("198.51.100.0/24", netlink.SCOPE_LINK, 8),
		route("10.0.0.0/8", netlink.SCOPE_UNIVERSE, 9),
		route("0.0.0.0/0", netlink.SCOPE_UNIVERSE, 9),
	}, 7)
	for _, tc := range []struct {
		ip       string
		reserved bool
	}{{"192.0.2.5", false}, {"198.51.100.5", true}, {"10.0.0.5", false}, {"203.0.113.5", false}} {
		_, got := d.local.Lookup(netip.MustParseAddr(tc.ip))
		if got != tc.reserved {
			t.Fatalf("%s reserved=%t, want %t", tc.ip, got, tc.reserved)
		}
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
