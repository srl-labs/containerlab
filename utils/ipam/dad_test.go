package ipam

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
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
				icmp := &layers.ICMPv6{
					TypeCode: layers.CreateICMPv6TypeCode(
						layers.ICMPv6TypeNeighborAdvertisement,
						0,
					),
				}
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
	p := &DADClient{}
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

func TestDADCachedConflicts(t *testing.T) {
	d := &DADClient{
		parent: "does-not-exist",
		cache: map[netip.Addr]struct{}{
			netip.MustParseAddr("192.0.2.123"):  {},
			netip.MustParseAddr("2001:db8::53"): {},
		},
	}
	for _, ip := range []string{"192.0.2.123", "2001:db8::53"} {
		available, err := d.Probe(context.Background(), netip.MustParseAddr(ip))
		if err != nil || available {
			t.Fatalf("cached address %s available=%t: %v", ip, available, err)
		}
	}
	if d.socket != nil {
		t.Fatal("opened wire probe for a cached conflict")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDADNeighbourCache(t *testing.T) {
	d := &DADClient{parent: "does-not-exist", cache: make(map[netip.Addr]struct{})}
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
		_, got := d.cache[netip.MustParseAddr(fmt.Sprintf("192.0.2.%d", i+1))]
		want := state == netlink.NUD_REACHABLE || state == netlink.NUD_PERMANENT
		if got != want {
			t.Fatalf("state %d reserved=%t, want %t", state, got, want)
		}
	}
	if _, got := d.cache[netip.MustParseAddr("2001:db8::1")]; got {
		t.Fatal("used neighbour from unrelated interface")
	}
	// A cached hit must return without trying to create a probe interface.
	available, err := d.Probe(context.Background(), netip.MustParseAddr("192.0.2.1"))
	if err != nil || available {
		t.Fatalf("cached address available=%t: %v", available, err)
	}
	if d.socket != nil {
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

func TestDADSchedulerRateAndWindows(t *testing.T) {
	const count = 80
	interval, observation := time.Millisecond, 20*time.Millisecond
	var mu sync.Mutex
	sends := map[netip.Addr][]time.Time{}
	var all []time.Time
	s := newDADScheduler(func(ip netip.Addr) error {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		all = append(all, now)
		sends[ip] = append(sends[ip], now)
		return nil
	}, func() error { return nil }, interval, observation)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Go(func() {
			ip := netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
			if i%2 == 1 {
				ip = netip.AddrFrom16([16]byte{0x20, 1, 0xd, 0xb8, 15: byte(i + 1)})
			}
			if err := s.Check(ctx, ip); err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			times := sends[ip]
			if len(times) != 3 || time.Since(times[2]) < observation {
				t.Errorf("accepted without three observation windows: %s %v", ip, times)
			}
		})
	}
	wg.Wait()
	if len(all) != count*3 {
		t.Fatalf("sent %d packets", len(all))
	}
	for i := dadSendBurst; i < len(all); i++ {
		if all[i].Sub(all[i-dadSendBurst]) < interval*dadSendBurst {
			t.Fatal("aggregate rate exceeded")
		}
	}
	for ip, times := range sends {
		for i := 1; i < len(times); i++ {
			if times[i].Sub(times[i-1]) < observation {
				t.Fatalf("probe spacing violated for %s", ip)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) != 0 || len(s.queue) != 0 {
		t.Fatal("completed requests retained")
	}
}

func TestDADClientConcurrentScale(t *testing.T) {
	for _, count := range []int{10, 100, 500, 1000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			var enteredOnce, releaseOnce sync.Once
			var sends atomic.Int64
			scheduler := newDADScheduler(
				func(netip.Addr) error {
					enteredOnce.Do(func() { close(entered) })
					<-release
					sends.Add(1)
					return nil
				},
				func() error { return nil },
				0,
				time.Millisecond,
			)
			defer scheduler.Close()
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			dad := &DADClient{
				cache:     make(map[netip.Addr]struct{}),
				socket:    &afpacket.TPacket{},
				scheduler: scheduler,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			results := make(chan error, count)
			for i := 2; i < count+2; i++ {
				ip := netip.AddrFrom4([4]byte{198, 19, byte(i / 256), byte(i % 256)})
				go func() {
					_, err := dad.Probe(ctx, ip)
					results <- err
				}()
			}

			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			for {
				scheduler.mu.Lock()
				pending := len(scheduler.pending)
				scheduler.mu.Unlock()
				if pending == count {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("only %d of %d probes were pending concurrently", pending, count)
				default:
					time.Sleep(time.Millisecond)
				}
			}

			unblock()
			for range count {
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
			if sends.Load() != int64(count*dadProbeAttempts) {
				t.Fatalf("sent %d probes, want %d", sends.Load(), count*dadProbeAttempts)
			}
		})
	}
}

func TestDADClientWireScale(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for network namespace and packet socket access")
	}

	goruntime.LockOSThread()
	hostNS, err := netns.Get()
	if err != nil {
		goruntime.UnlockOSThread()
		t.Fatal(err)
	}
	peerNS, err := netns.New()
	if err != nil {
		hostNS.Close()
		goruntime.UnlockOSThread()
		t.Fatal(err)
	}
	if err := netns.Set(hostNS); err != nil {
		peerNS.Close()
		hostNS.Close()
		goruntime.UnlockOSThread()
		t.Fatal(err)
	}
	hostNS.Close()
	goruntime.UnlockOSThread()
	defer peerNS.Close()

	parentName := fmt.Sprintf("dadp%x", os.Getpid())
	peerName := fmt.Sprintf("dadx%x", os.Getpid())
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: parentName},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatal(err)
	}
	parent, err := netlink.LinkByName(parentName)
	if err != nil {
		t.Fatal(err)
	}
	defer netlink.LinkDel(parent)
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetNsFd(peer, int(peerNS)); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(parent); err != nil {
		t.Fatal(err)
	}

	peerLinks, err := netlink.NewHandleAt(peerNS)
	if err != nil {
		t.Fatal(err)
	}
	defer peerLinks.Close()
	peer, err = peerLinks.LinkByName(peerName)
	if err != nil {
		t.Fatal(err)
	}
	if err := peerLinks.LinkSetUp(peer); err != nil {
		t.Fatal(err)
	}

	ipv4Addresses := make([]netip.Addr, 1000)
	ipv6Addresses := make([]netip.Addr, 1000)
	ipv4 := netip.MustParseAddr("198.19.0.0")
	ipv6 := netip.MustParseAddr("fd00:dad::")
	for i := range ipv4Addresses {
		ipv4 = ipv4.Next()
		ipv6 = ipv6.Next()
		ipv4Addresses[i], ipv6Addresses[i] = ipv4, ipv6
		for _, address := range []struct {
			ip    netip.Addr
			bits  int
			flags int
		}{
			{ip: ipv4, bits: 16},
			{ip: ipv6, bits: 64, flags: unix.IFA_F_NODAD},
		} {
			linkAddress := &netlink.Addr{
				IPNet: &net.IPNet{
					IP:   net.IP(address.ip.AsSlice()),
					Mask: net.CIDRMask(address.bits, address.ip.BitLen()),
				},
				Flags: address.flags,
			}
			if err := peerLinks.AddrAdd(peer, linkAddress); err != nil {
				t.Fatal(err)
			}
		}
	}

	families := []struct {
		name      string
		addresses func(int) []netip.Addr
	}{
		{name: "IPv4", addresses: func(count int) []netip.Addr {
			return ipv4Addresses[:count]
		}},
		{name: "IPv6", addresses: func(count int) []netip.Addr {
			return ipv6Addresses[:count]
		}},
		{name: "dual-stack", addresses: func(count int) []netip.Addr {
			addresses := append([]netip.Addr(nil), ipv4Addresses[:count]...)
			return append(addresses, ipv6Addresses[:count]...)
		}},
	}
	for _, family := range families {
		t.Run(family.name, func(t *testing.T) {
			for _, count := range []int{10, 100, 500, 1000} {
				t.Run(fmt.Sprint(count), func(t *testing.T) {
					dad, err := NewDADClient(parentName)
					if err != nil {
						t.Fatal(err)
					}
					defer dad.Close()

					targets := family.addresses(count)
					started := time.Now()
					results := make(chan error, len(targets))
					var probes sync.WaitGroup
					for _, ip := range targets {
						probes.Go(func() {
							available, err := dad.Probe(context.Background(), ip)
							if err == nil && available {
								err = fmt.Errorf("occupied address %s reported available", ip)
							}
							results <- err
						})
					}
					probes.Wait()
					close(results)
					for err := range results {
						if err != nil {
							t.Fatal(err)
						}
					}
					t.Logf(
						"%s scale %d (%d probes) completed in %s",
						family.name,
						count,
						len(targets),
						time.Since(started),
					)
				})
			}
		})
	}
}

func TestDADSchedulerCancellationAndLateConflict(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.2")
	sent := make(chan netip.Addr, 10)
	s := newDADScheduler(
		func(ip netip.Addr) error { sent <- ip; return nil },
		func() error { return nil },
		time.Millisecond,
		20*time.Millisecond,
	)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkCtx, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- s.Check(checkCtx, ip) }()
	select {
	case <-sent:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	s.mu.Lock()
	remaining := len(s.pending) + len(s.queue)
	s.mu.Unlock()
	if remaining != 0 {
		t.Fatal("cancelled request retained")
	}
	// A late response during the final observation window must still reject.
	go func() { result <- s.Check(ctx, ip.Next()) }()
	for i := 0; i < 3; i++ {
		select {
		case <-sent:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	s.conflict(ip.Next(), errDuplicateAddress)
	if err := <-result; !errors.Is(err, errDuplicateAddress) {
		t.Fatalf("late conflict: %v", err)
	}
}

func TestDADSchedulerErrorsAndDrain(t *testing.T) {
	for _, stage := range []string{"send", "receive", "late-drain"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("socket failure")
			ip := netip.MustParseAddr("2001:db8::2")
			var s *dadScheduler
			s = newDADScheduler(func(netip.Addr) error {
				if stage == "send" {
					return failure
				}
				return nil
			}, func() error {
				if stage == "late-drain" {
					s.conflict(ip, errDuplicateAddress)
					return nil
				}
				return failure
			}, time.Millisecond, time.Millisecond)
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			want := failure
			if stage == "late-drain" {
				want = errDuplicateAddress
			}
			if err := s.Check(ctx, ip); !errors.Is(err, want) {
				t.Fatalf("%s: %v", stage, err)
			}
		})
	}
}

func TestDADSchedulerRestartsAfterObservationLoss(t *testing.T) {
	var sends, drains int
	s := newDADScheduler(
		func(netip.Addr) error {
			sends++
			return nil
		},
		func() error {
			drains++
			if drains == 1 {
				return errDADObservationLost
			}
			return nil
		},
		time.Millisecond,
		time.Millisecond,
	)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Check(ctx, netip.MustParseAddr("192.0.2.2")); err != nil {
		t.Fatal(err)
	}
	if sends != 6 || drains != 2 {
		t.Fatalf("sends=%d drains=%d, want 6 and 2", sends, drains)
	}
}

func TestDADSchedulerSendDoesNotBlockConflicts(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.2")
	entered := make(chan struct{})
	release := make(chan struct{})
	s := newDADScheduler(
		func(netip.Addr) error {
			close(entered)
			<-release
			return nil
		},
		func() error { return nil },
		time.Millisecond,
		time.Second,
	)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- s.Check(ctx, ip) }()
	<-entered
	conflicted := make(chan struct{})
	go func() {
		s.conflict(ip, errDuplicateAddress)
		close(conflicted)
	}()
	select {
	case <-conflicted:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("socket write held the scheduler mutex")
	}
	close(release)
	if err := <-result; !errors.Is(err, errDuplicateAddress) {
		t.Fatalf("conflict result: %v", err)
	}
}
