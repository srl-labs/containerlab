package mgmt

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/libnetwork/resolvconf"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils/ipam"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// ErrDuplicateAddress indicates an occupied candidate; allocation may retry another address.
var ErrDuplicateAddress = errors.New("duplicate address")

// occupiedPrefix lets allocation skip a whole reserved route without trying
// every address in that route.
type occupiedPrefix struct{ prefix netip.Prefix }

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
			if err != nil {
				return fmt.Errorf("look up management bridge: %w", err)
			}
			bridgeIndex = bridge.Attrs().Index
		}
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err != nil {
			return fmt.Errorf("snapshot host routes: %w", err)
		}
		d.addRoutes(routes, bridgeIndex)
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

func (d *dadChecker) addRoutes(routes []netlink.Route, bridgeIndex int) {
	for _, route := range routes {
		if route.Scope != netlink.SCOPE_LINK || route.Dst == nil ||
			route.Dst.IP.IsUnspecified() ||
			(bridgeIndex != 0 && route.LinkIndex == bridgeIndex) {
			continue
		}
		if prefix, err := netip.ParsePrefix(route.Dst.String()); err == nil {
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
	fd        int
	mac       net.HardwareAddr
	scheduler *dadScheduler
	rxMu      sync.Mutex
	receiver  sync.WaitGroup
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
			Name:        fmt.Sprintf("cd-%x", nonce),
			ParentIndex: parent.Attrs().Index,
		},
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}
	if err = netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("create DAD interface: %w", err)
	}
	p := &macvlanProbe{link: link, fd: -1}
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
	if p.fd >= 0 {
		err = errors.Join(err, unix.Close(p.fd))
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
	return p.scheduler.Check(ctx, ip)
}

// CheckDuplicateAddresses probes a batch using a single temporary interface.
func CheckDuplicateAddresses(
	ctx context.Context,
	parentName string,
	addresses []netip.Addr,
) (err error) {
	p, err := newMacvlanProbe(parentName)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := p.Close(); cleanupErr != nil {
			err = fmt.Errorf("remove DAD interface (probe result: %v): %w", err, cleanupErr)
		}
	}()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	_, err = ipam.CheckIPAddresses(ctx, addresses, len(addresses), func(ip netip.Addr) bool {
		if err := p.Check(ctx, ip); err != nil {
			cancel(fmt.Errorf("management address %s: %w", ip, err))
			return false
		}
		return true
	})
	return err
}

// ProbeAddress checks an address on a caller-owned auxiliary interface.
func ProbeAddress(ctx context.Context, link *netlink.LinkAttrs, ip netip.Addr) error {
	p := &macvlanProbe{link: &netlink.GenericLink{LinkAttrs: *link}, fd: -1}
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
	protocol := int(binary.NativeEndian.Uint16([]byte{0, unix.ETH_P_ALL}))
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	if err != nil {
		return fmt.Errorf("open DAD socket: %w", err)
	}
	p.fd = fd
	link := p.link.Attrs()
	p.mac = append(net.HardwareAddr(nil), link.HardwareAddr...)
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: link.Index, Protocol: uint16(protocol)}); err != nil {
		return err
	}
	if err := unix.SetsockoptPacketMreq(fd, unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP,
		&unix.PacketMreq{Ifindex: int32(link.Index), Type: unix.PACKET_MR_ALLMULTI}); err != nil {
		return err
	}
	// Filter before packets enter the socket queue: ARP or direct ICMPv6 only.
	filter := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_H | unix.BPF_ABS, K: 12},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0x0806, Jt: 3},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 0x86dd, Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_B | unix.BPF_ABS, K: 20},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: 58, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: 2048},
		{Code: unix.BPF_RET | unix.BPF_K, K: 0},
	}
	if err := unix.SetsockoptSockFprog(fd, unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, &unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}); err != nil {
		return err
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, 4<<20); err != nil {
		return err
	}
	p.scheduler = newDADScheduler(func(ip netip.Addr) error {
		return unix.Sendto(fd, dadProbe(link.HardwareAddr, ip), 0, &unix.SockaddrLinklayer{Ifindex: link.Index, Protocol: uint16(protocol), Halen: 6})
	}, p.drain, time.Second/1000, time.Second)
	p.receiver.Go(func() {
		for p.scheduler.ctx.Err() == nil {
			if err := p.drain(); err != nil {
				p.scheduler.cancel(err)
				return
			}
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			if _, err := unix.Poll(fds, 100); err != nil && !errors.Is(err, unix.EINTR) {
				p.scheduler.cancel(err)
				return
			}
			if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
				p.scheduler.cancel(fmt.Errorf("DAD receive socket failed"))
				return
			}
		}
	})
	return nil
}

func (p *macvlanProbe) drain() error {
	p.rxMu.Lock()
	defer p.rxMu.Unlock()
	var buf [2048]byte
	for count := 0; count < 4096; count++ {
		if err := context.Cause(p.scheduler.ctx); err != nil {
			return err
		}
		n, from, err := unix.Recvfrom(p.fd, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if errors.Is(err, unix.EAGAIN) {
			stats, err := unix.GetsockoptTpacketStats(p.fd, unix.SOL_PACKET, unix.PACKET_STATISTICS)
			if err != nil {
				return err
			}
			if stats.Drops > 0 {
				return fmt.Errorf("DAD receive queue dropped %d packets", stats.Drops)
			}
			return nil
		}
		if err != nil {
			return err
		}
		if from, ok := from.(*unix.SockaddrLinklayer); ok && from.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		if ip, ok := dadConflictAddress(buf[:n], p.mac); ok {
			p.scheduler.conflict(ip, fmt.Errorf("%w detected (peer MAC %s)", ErrDuplicateAddress, net.HardwareAddr(buf[6:12])))
		}
	}
	return fmt.Errorf("DAD receive queue overloaded")
}

func dadProbe(mac net.HardwareAddr, ip netip.Addr) []byte {
	if ip.Is4() {
		b := make([]byte, 42)
		copy(b, []byte{255, 255, 255, 255, 255, 255})
		copy(b[6:12], mac)
		copy(b[12:22], []byte{8, 6, 0, 1, 8, 0, 6, 4, 0, 1})
		copy(b[22:28], mac)
		copy(b[38:42], ip.AsSlice())
		return b
	}
	b := make([]byte, 78)
	target := ip.As16()
	copy(b[:6], []byte{0x33, 0x33, 0xff, target[13], target[14], target[15]})
	copy(b[6:12], mac)
	copy(b[12:14], []byte{0x86, 0xdd})
	b[14] = 0x60
	b[19] = 24
	b[20] = 58
	b[21] = 255
	copy(
		b[38:54],
		[]byte{0xff, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0xff, target[13], target[14], target[15]},
	)
	b[54] = 135
	copy(b[62:78], target[:])
	binary.BigEndian.PutUint16(b[56:58], dadChecksum(b[22:54], b[54:]))
	return b
}

func dadChecksum(addresses, payload []byte) uint16 {
	sum := uint32(len(payload) + 58)
	for _, part := range [][]byte{addresses, payload} {
		for i := 0; i+1 < len(part); i += 2 {
			sum += uint32(binary.BigEndian.Uint16(part[i : i+2]))
		}
		if len(part)%2 != 0 {
			sum += uint32(part[len(part)-1]) << 8
		}
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// dadConflictAddress extracts the address claimed or probed by a peer.
// Frames from the local MAC are ignored.
func dadConflictAddress(b []byte, mac net.HardwareAddr) (netip.Addr, bool) {
	if len(b) < 14 || bytes.Equal(b[6:12], mac) {
		return netip.Addr{}, false
	}
	if b[12] == 8 && b[13] == 6 {
		if len(b) < 42 || !bytes.Equal(b[12:20], []byte{8, 6, 0, 1, 8, 0, 6, 4}) ||
			(binary.BigEndian.Uint16(b[20:22]) != 1 && binary.BigEndian.Uint16(b[20:22]) != 2) {
			return netip.Addr{}, false
		}
		sender := netip.AddrFrom4([4]byte(b[28:32]))
		if sender.IsUnspecified() && binary.BigEndian.Uint16(b[20:22]) == 1 {
			return netip.AddrFrom4([4]byte(b[38:42])), true
		}
		return sender, true
	}
	if len(b) < 78 || b[12] != 0x86 || b[13] != 0xdd || b[14]>>4 != 6 || b[20] != 58 || b[21] != 255 || b[55] != 0 {
		return netip.Addr{}, false
	}
	length := int(binary.BigEndian.Uint16(b[18:20]))
	if length < 24 || len(b) < 54+length || dadChecksum(b[22:54], b[54:54+length]) != 0 {
		return netip.Addr{}, false
	}
	if b[54] != 136 && !(b[54] == 135 && bytes.Equal(b[22:38], make([]byte, 16))) {
		return netip.Addr{}, false
	}
	return netip.AddrFrom16([16]byte(b[62:78])), true
}
func dadConflict(b []byte, mac net.HardwareAddr, ip netip.Addr) bool {
	target, ok := dadConflictAddress(b, mac)
	return ok && target == ip
}
