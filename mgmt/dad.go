package mgmt

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	"github.com/gopacket/gopacket/layers"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/bpf"
)

// ErrDuplicateAddress indicates an occupied candidate; allocation may retry another address.
var ErrDuplicateAddress = errors.New("duplicate address")

// occupiedPrefix lets allocation skip a whole reserved route without trying
// every address in that route.
type occupiedPrefix struct{ prefix netip.Prefix }

var errDADObservationLost = errors.New("DAD observation lost")

func (e *occupiedPrefix) Error() string {
	return fmt.Sprintf("%s in host reservation %s", ErrDuplicateAddress, e.prefix)
}

func (e *occupiedPrefix) Unwrap() error { return ErrDuplicateAddress }

// dadChecker takes one host snapshot per allocation pass. Lookups are bounded
// by address width; macvlan wire probes reuse one temporary interface.
type dadChecker struct {
	mu     sync.Mutex
	local  clabtypes.IPPrefixSet
	loaded bool
	wire   *macvlanProbe
}

func (d *dadChecker) load(m *clabtypes.MgmtNet) error {
	addresses, err := netlink.AddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("snapshot host addresses: %w", err)
	}
	for _, address := range addresses {
		if ip, ok := netip.AddrFromSlice(address.IP); ok {
			ip = ip.Unmap()
			d.local.Add(netip.PrefixFrom(ip, ip.BitLen()))
		}
	}
	if m.Driver == "macvlan" {
		parent, err := netlink.LinkByName(m.MacvlanParent)
		if err != nil {
			return fmt.Errorf("look up macvlan parent: %w", err)
		}
		links, err := netlink.LinkList()
		if err != nil {
			return fmt.Errorf("snapshot host links: %w", err)
		}
		interfaces := map[int]bool{parent.Attrs().Index: true}
		for _, link := range links {
			if link.Type() == "macvlan" && link.Attrs().ParentIndex == parent.Attrs().Index {
				interfaces[link.Attrs().Index] = true
			}
		}
		neighbours, err := netlink.NeighList(0, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("snapshot neighbour cache: %w", err)
		}
		d.addNeighbours(neighbours, interfaces)
	}
	if m.Driver != "macvlan" {
		// Docker's reserved-network heuristic uses IPv4 on-link routes and DNS
		// servers, not default routes or broad VPN routes via a gateway.
		bridgeIndex := 0
		if m.Bridge != "" {
			bridge, err := netlink.LinkByName(m.Bridge)
			if err != nil && !errors.As(err, &netlink.LinkNotFoundError{}) {
				return fmt.Errorf("look up management bridge: %w", err)
			}
			if err == nil {
				bridgeIndex = bridge.Attrs().Index
			}
		}
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err != nil {
			return fmt.Errorf("snapshot host routes: %w", err)
		}
		subnet, _ := netip.ParsePrefix(m.IPv4Subnet)
		d.addRoutes(routes, bridgeIndex, subnet)
		if contents, err := os.ReadFile(resolvconf.Path()); err == nil {
			for _, prefix := range resolvconf.GetNameserversAsPrefix(contents) {
				d.local.Add(prefix)
			}
		}
	}
	d.loaded = true
	return nil
}

func (d *dadChecker) prepare(m *clabtypes.MgmtNet, ip netip.Addr) (*macvlanProbe, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.loaded {
		if err := d.load(m); err != nil {
			return nil, err
		}
	}
	if prefix, found := d.local.Lookup(ip); found {
		return nil, &occupiedPrefix{prefix: prefix}
	}
	if m.Driver == "macvlan" && m.EffectiveMacvlanMode() == "bridge" {
		if d.wire == nil {
			wire, err := newMacvlanProbe(m.MacvlanParent)
			if err != nil {
				return nil, err
			}
			d.wire = wire
		}
		return d.wire, nil
	}
	return nil, nil
}

// Check serializes snapshot/interface initialization and probes without the lock.
// Callers must finish all checks before Close removes the shared interface.
func (d *dadChecker) Check(ctx context.Context, m *clabtypes.MgmtNet, ip netip.Addr) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wire, err := d.prepare(m, ip)
	if err != nil {
		return err
	}
	if wire != nil {
		return wire.Check(ctx, ip)
	}
	return nil
}

func (d *dadChecker) Close() error { return d.wire.Close() }

func (d *dadChecker) addRoutes(routes []netlink.Route, bridgeIndex int, subnet netip.Prefix) {
	for _, route := range routes {
		if route.Scope != netlink.SCOPE_LINK || route.Dst == nil ||
			route.Dst.IP.IsUnspecified() ||
			(bridgeIndex != 0 && route.LinkIndex == bridgeIndex) {
			continue
		}
		if prefix, err := netip.ParsePrefix(route.Dst.String()); err == nil {
			if subnet.IsValid() && prefix.Bits() < subnet.Bits() &&
				prefix.Contains(subnet.Masked().Addr()) {
				continue
			}
			d.local.Add(prefix)
		}
	}
}

// Only recently confirmed or administratively pinned neighbours are treated as
// occupied. STALE/DELAY/PROBE entries still require a fresh wire probe.
func (d *dadChecker) addNeighbours(neighbours []netlink.Neigh, interfaces map[int]bool) {
	for _, neighbour := range neighbours {
		if !interfaces[neighbour.LinkIndex] || len(neighbour.HardwareAddr) == 0 ||
			neighbour.State&(netlink.NUD_REACHABLE|netlink.NUD_PERMANENT) == 0 {
			continue
		}
		if ip, ok := netip.AddrFromSlice(neighbour.IP); ok {
			ip = ip.Unmap()
			d.local.Add(netip.PrefixFrom(ip, ip.BitLen()))
		}
	}
}

// macvlanProbe reuses one temporary interface for an allocation pass.
type macvlanProbe struct {
	link      netlink.Link
	socket    *afpacket.TPacket
	mac       net.HardwareAddr
	scheduler *dadScheduler
	rxMu      sync.Mutex
	receiver  sync.WaitGroup
	drops     uint
}

func newMacvlanProbe(parentName string) (*macvlanProbe, error) {
	parent, err := netlink.LinkByName(parentName)
	if err != nil {
		return nil, err
	}
	var nonce [5]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	link := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:         fmt.Sprintf("cd-%x", nonce),
			ParentIndex:  parent.Attrs().Index,
			HardwareAddr: append(net.HardwareAddr{0x02}, nonce[:]...),
		},
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}
	if err = netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("create DAD interface: %w", err)
	}
	p := &macvlanProbe{link: link}
	if err = netlink.LinkSetUp(link); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	actual, err := netlink.LinkByName(link.Name)
	if err != nil {
		return nil, errors.Join(err, p.Close())
	}
	p.link = actual
	if err := p.start(); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

func (p *macvlanProbe) Close() error {
	if p == nil {
		return nil
	}
	var err error
	if p.scheduler != nil {
		err = p.scheduler.Close()
		p.receiver.Wait()
	}
	if p.socket != nil {
		p.socket.Close()
	}
	if p.link != nil {
		err = errors.Join(err, netlink.LinkDel(p.link))
	}
	return err
}
func (p *macvlanProbe) Check(ctx context.Context, ip netip.Addr) error {
	if !ip.IsValid() || ip.Is4In6() || !ip.IsGlobalUnicast() {
		return fmt.Errorf("invalid DAD address %s", ip)
	}
	err := p.scheduler.Check(ctx, ip)
	log.Debug("DAD check completed", "mac", p.mac, "address", ip, "error", err)
	return err
}

// ProbeAddress checks an address on a caller-owned auxiliary interface.
func ProbeAddress(ctx context.Context, link *netlink.LinkAttrs, ip netip.Addr) error {
	p := &macvlanProbe{link: &netlink.GenericLink{LinkAttrs: *link}}
	if err := p.start(); err != nil {
		p.link = nil
		return errors.Join(err, p.Close())
	}
	err := p.Check(ctx, ip)
	// Close must preserve the caller-owned interface.
	p.link = nil
	return errors.Join(err, p.Close())
}

func (p *macvlanProbe) start() error {
	link := p.link.Attrs()
	p.mac = append(net.HardwareAddr(nil), link.HardwareAddr...)
	log.Debug("Opening DAD socket", "interface", link.Name, "index", link.Index,
		"parent-index", link.ParentIndex, "mac", p.mac, "flags", link.Flags)

	socket, err := afpacket.NewTPacket(
		afpacket.OptInterface(link.Name),
		afpacket.OptTPacketVersion(afpacket.TPacketVersion2),
		afpacket.OptPollTimeout(0),
		afpacket.OptFrameSize(4096),
		afpacket.OptBlockSize(1<<20),
		afpacket.OptNumBlocks(4),
	)
	if err != nil {
		return fmt.Errorf("open DAD socket: %w", err)
	}
	p.socket = socket
	if err := socket.SetPromiscuous(true); err != nil {
		return err
	}
	filter, err := bpf.Assemble(dadCaptureFilter())
	if err != nil {
		return err
	}
	if err := socket.SetBPF(filter); err != nil {
		return err
	}
	p.scheduler = newDADScheduler(func(ip netip.Addr) error {
		frame, err := dadProbe(p.mac, ip)
		if err != nil {
			return err
		}
		err = socket.WritePacketData(frame)
		if log.GetLevel() == log.DebugLevel {
			log.Debug("DAD probe sent", "interface", link.Name, "address", ip,
				"frame", fmt.Sprintf("%x", frame), "error", err)
		}
		return err
	}, p.drain, time.Millisecond, time.Second)
	p.receiver.Go(func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if err := p.drain(); err != nil {
				if errors.Is(err, errDADObservationLost) {
					p.scheduler.restartPending()
				} else {
					p.scheduler.cancel(err)
					return
				}
			}
			select {
			case <-p.scheduler.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	return nil
}

func dadCaptureFilter() []bpf.Instruction {
	return []bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(layers.EthernetTypeARP), SkipTrue: 7},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(layers.EthernetTypeIPv6), SkipFalse: 5},
		bpf.LoadAbsolute{Off: 20, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(layers.IPProtocolICMPv6), SkipFalse: 3},
		bpf.LoadAbsolute{Off: 54, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(layers.ICMPv6TypeNeighborSolicitation), SkipTrue: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(layers.ICMPv6TypeNeighborAdvertisement), SkipTrue: 1},
		bpf.RetConstant{Val: 0},
		bpf.RetConstant{Val: 2048},
	}
}

func (p *macvlanProbe) drain() error {
	p.rxMu.Lock()
	defer p.rxMu.Unlock()
	for count := 0; count < 4096; count++ {
		if err := context.Cause(p.scheduler.ctx); err != nil {
			return err
		}
		frame, info, err := p.socket.ZeroCopyReadPacketData()
		if errors.Is(err, afpacket.ErrTimeout) {
			stats, _, err := p.socket.SocketStats()
			if err != nil {
				return err
			}
			if err := p.checkDrops(stats.Drops()); err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			return err
		}
		if log.GetLevel() == log.DebugLevel {
			log.Debug("DAD frame received", "mac", p.mac,
				"captured-length", info.CaptureLength, "wire-length", info.Length,
				"frame", fmt.Sprintf("%x", frame))
		}
		if info.CaptureLength != info.Length {
			log.Debug("Ignoring truncated DAD frame")
			continue
		}
		if ip, ok := dadConflictAddress(frame, p.mac); ok {
			log.Debug("DAD conflict received", "mac", p.mac, "address", ip)
			p.scheduler.conflict(ip, fmt.Errorf("%w detected", ErrDuplicateAddress))
		} else {
			log.Debug("Ignoring DAD frame without a valid peer address")
		}
	}
	return fmt.Errorf("%w: receive queue overloaded", errDADObservationLost)
}

func (p *macvlanProbe) checkDrops(drops uint) error {
	if drops <= p.drops {
		return nil
	}
	delta := drops - p.drops
	p.drops = drops
	return fmt.Errorf("%w: receive queue dropped %d packets", errDADObservationLost, delta)
}

func dadProbe(mac net.HardwareAddr, ip netip.Addr) ([]byte, error) {
	ethernet := &layers.Ethernet{SrcMAC: mac, DstMAC: net.HardwareAddr{255, 255, 255, 255, 255, 255}, EthernetType: layers.EthernetTypeARP}
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if ip.Is4() {
		arp := &layers.ARP{
			AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4, Operation: layers.ARPRequest,
			SourceHwAddress: mac, SourceProtAddress: net.IPv4zero.To4(),
			DstHwAddress: make([]byte, 6), DstProtAddress: ip.AsSlice(),
		}
		err := gopacket.SerializeLayers(buffer, options, ethernet, arp)
		return buffer.Bytes(), err
	}
	target := ip.As16()
	destination := net.ParseIP("ff02::1:ff00:0").To16()
	copy(destination[13:], target[13:])
	ethernet.EthernetType = layers.EthernetTypeIPv6
	ethernet.DstMAC = net.HardwareAddr{0x33, 0x33, 0xff, target[13], target[14], target[15]}
	ipv6 := &layers.IPv6{Version: 6, HopLimit: 255, NextHeader: layers.IPProtocolICMPv6, SrcIP: net.IPv6zero, DstIP: destination}
	icmp := &layers.ICMPv6{TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeNeighborSolicitation, 0)}
	if err := icmp.SetNetworkLayerForChecksum(ipv6); err != nil {
		return nil, err
	}
	err := gopacket.SerializeLayers(buffer, options, ethernet, ipv6, icmp, &layers.ICMPv6NeighborSolicitation{TargetAddress: ip.AsSlice()})
	return buffer.Bytes(), err
}

// dadConflictAddress extracts the address claimed or probed by a peer.
// Frames from the local MAC are ignored.
func dadConflictAddress(frame []byte, mac net.HardwareAddr) (netip.Addr, bool) {
	packet := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.NoCopy)
	if packet.ErrorLayer() != nil || packet.Metadata().Truncated {
		return netip.Addr{}, false
	}
	ethernet, ok := packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	if !ok || bytes.Equal(ethernet.SrcMAC, mac) {
		return netip.Addr{}, false
	}
	if ethernet.EthernetType == layers.EthernetTypeARP {
		arp, ok := packet.Layer(layers.LayerTypeARP).(*layers.ARP)
		if !ok || arp.AddrType != layers.LinkTypeEthernet || arp.Protocol != layers.EthernetTypeIPv4 ||
			arp.HwAddressSize != 6 || arp.ProtAddressSize != 4 || (arp.Operation != layers.ARPRequest && arp.Operation != layers.ARPReply) {
			return netip.Addr{}, false
		}
		sender, ok := netip.AddrFromSlice(arp.SourceProtAddress)
		if ok && sender.IsUnspecified() && arp.Operation == layers.ARPRequest {
			return netip.AddrFromSlice(arp.DstProtAddress)
		}
		return sender, ok
	}
	ipv6, ok := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
	if !ok || ethernet.EthernetType != layers.EthernetTypeIPv6 || ipv6.Version != 6 ||
		ipv6.NextHeader != layers.IPProtocolICMPv6 || ipv6.HopLimit != 255 {
		return netip.Addr{}, false
	}
	icmp, ok := packet.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
	if !ok || icmp.TypeCode.Code() != 0 {
		return netip.Addr{}, false
	}
	if err := icmp.SetNetworkLayerForChecksum(ipv6); err != nil {
		return netip.Addr{}, false
	}
	if err, result := icmp.VerifyChecksum(); err != nil || !result.Valid {
		return netip.Addr{}, false
	}
	if advertisement, ok := packet.Layer(layers.LayerTypeICMPv6NeighborAdvertisement).(*layers.ICMPv6NeighborAdvertisement); ok {
		return netip.AddrFromSlice(advertisement.TargetAddress)
	}
	if solicitation, ok := packet.Layer(layers.LayerTypeICMPv6NeighborSolicitation).(*layers.ICMPv6NeighborSolicitation); ok && ipv6.SrcIP.IsUnspecified() {
		return netip.AddrFromSlice(solicitation.TargetAddress)
	}
	return netip.Addr{}, false
}
