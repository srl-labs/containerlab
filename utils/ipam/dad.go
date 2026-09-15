package ipam

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/bpf"
)

const (
	dadProbeAttempts        = 3
	dadObservationTimeout   = time.Second
	dadSendInterval         = time.Millisecond
	dadSendBurst            = 8
	dadReceivePollInterval  = 10 * time.Millisecond
	dadSocketFrameSize      = 4096
	dadSocketBlockSize      = 1 << 20
	dadSocketBlocks         = 4
	dadCaptureSnapshotBytes = 2048
	dadDrainBatchSize       = 4096
)

var errDuplicateAddress = errors.New("duplicate address")
var errDADObservationLost = errors.New("DAD observation lost")

// DADClient checks address availability on a macvlan parent segment.
type DADClient struct {
	mu        sync.Mutex
	parent    string
	cache     map[netip.Addr]struct{}
	link      netlink.Link
	socket    *afpacket.TPacket
	mac       net.HardwareAddr
	scheduler *dadScheduler
	rxMu      sync.Mutex
	receiver  sync.WaitGroup
	drops     uint
}

// NewDADClient snapshots host and neighbour addresses for the parent segment.
func NewDADClient(parentName string) (*DADClient, error) {
	if parentName == "" {
		return nil, errors.New("DAD requires an interface")
	}

	d := &DADClient{parent: parentName, cache: make(map[netip.Addr]struct{})}
	if err := d.load(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DADClient) load() error {
	addresses, err := netlink.AddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("snapshot host addresses: %w", err)
	}

	for _, address := range addresses {
		if ip, ok := netip.AddrFromSlice(address.IP); ok {
			d.cache[ip.Unmap()] = struct{}{}
		}
	}

	parent, err := netlink.LinkByName(d.parent)
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

	return nil
}

// Probe reports whether ip is available.
func (d *DADClient) Probe(ctx context.Context, ip netip.Addr) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !ip.IsValid() || ip.Is4In6() || !ip.IsGlobalUnicast() {
		return false, fmt.Errorf("invalid DAD address %s", ip)
	}

	d.mu.Lock()
	if _, occupied := d.cache[ip]; occupied {
		d.mu.Unlock()
		return false, nil
	}
	if d.socket == nil {
		if err := d.startProbe(); err != nil {
			d.mu.Unlock()
			return false, err
		}
	}
	d.mu.Unlock()

	err := d.check(ctx, ip)
	if errors.Is(err, errDuplicateAddress) {
		return false, nil
	}
	return err == nil, err
}

// Close releases the temporary probe interface and packet socket.
// It must not run concurrently with Probe.
func (d *DADClient) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.close()
}

// Only recently confirmed or administratively pinned neighbours are treated as
// occupied. STALE/DELAY/PROBE entries still require a fresh wire probe.
func (d *DADClient) addNeighbours(neighbours []netlink.Neigh, interfaces map[int]bool) {
	for _, neighbour := range neighbours {
		if !interfaces[neighbour.LinkIndex] || len(neighbour.HardwareAddr) == 0 ||
			neighbour.State&(netlink.NUD_REACHABLE|netlink.NUD_PERMANENT) == 0 {
			continue
		}
		if ip, ok := netip.AddrFromSlice(neighbour.IP); ok {
			d.cache[ip.Unmap()] = struct{}{}
		}
	}
}

func (d *DADClient) startProbe() error {
	parent, err := netlink.LinkByName(d.parent)
	if err != nil {
		return err
	}
	var nonce [5]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
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
		return fmt.Errorf("create DAD interface: %w", err)
	}
	d.link = link
	if err = netlink.LinkSetUp(link); err != nil {
		return errors.Join(err, d.close())
	}
	actual, err := netlink.LinkByName(link.Name)
	if err != nil {
		return errors.Join(err, d.close())
	}
	d.link = actual
	if err := d.start(); err != nil {
		return errors.Join(err, d.close())
	}
	return nil
}

func (d *DADClient) close() error {
	var err error
	if d.scheduler != nil {
		err = d.scheduler.Close()
		d.receiver.Wait()
		d.scheduler = nil
	}
	if d.socket != nil {
		d.socket.Close()
		d.socket = nil
	}
	if d.link != nil {
		err = errors.Join(err, netlink.LinkDel(d.link))
		d.link = nil
	}
	return err
}
func (d *DADClient) check(ctx context.Context, ip netip.Addr) error {
	err := d.scheduler.Check(ctx, ip)
	log.Debug("DAD check completed", "mac", d.mac, "address", ip, "error", err)
	return err
}

func (d *DADClient) start() error {
	link := d.link.Attrs()
	d.mac = append(net.HardwareAddr(nil), link.HardwareAddr...)
	log.Debug("Opening DAD socket", "interface", link.Name, "index", link.Index,
		"parent-index", link.ParentIndex, "mac", d.mac, "flags", link.Flags)

	socket, err := afpacket.NewTPacket(
		afpacket.OptInterface(link.Name),
		afpacket.OptTPacketVersion(afpacket.TPacketVersion2),
		afpacket.OptPollTimeout(0),
		afpacket.OptFrameSize(dadSocketFrameSize),
		afpacket.OptBlockSize(dadSocketBlockSize),
		afpacket.OptNumBlocks(dadSocketBlocks),
	)
	if err != nil {
		return fmt.Errorf("open DAD socket: %w", err)
	}
	d.socket = socket
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
	d.scheduler = newDADScheduler(func(ip netip.Addr) error {
		frame, err := dadProbe(d.mac, ip)
		if err != nil {
			return err
		}
		err = socket.WritePacketData(frame)
		if log.GetLevel() == log.DebugLevel {
			log.Debug("DAD probe sent", "interface", link.Name, "address", ip,
				"frame", fmt.Sprintf("%x", frame), "error", err)
		}
		return err
	}, d.drain, dadSendInterval, dadObservationTimeout)
	d.receiver.Go(func() {
		ticker := time.NewTicker(dadReceivePollInterval)
		defer ticker.Stop()
		for {
			if err := d.drain(); err != nil {
				if errors.Is(err, errDADObservationLost) {
					d.scheduler.restartPending()
				} else {
					d.scheduler.cancel(err)
					return
				}
			}
			select {
			case <-d.scheduler.ctx.Done():
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
		bpf.JumpIf{
			Cond:     bpf.JumpEqual,
			Val:      uint32(layers.ICMPv6TypeNeighborSolicitation),
			SkipTrue: 2,
		},
		bpf.JumpIf{
			Cond:     bpf.JumpEqual,
			Val:      uint32(layers.ICMPv6TypeNeighborAdvertisement),
			SkipTrue: 1,
		},
		bpf.RetConstant{Val: 0},
		bpf.RetConstant{Val: dadCaptureSnapshotBytes},
	}
}

func (d *DADClient) drain() error {
	d.rxMu.Lock()
	defer d.rxMu.Unlock()
	for count := 0; count < dadDrainBatchSize; count++ {
		if err := context.Cause(d.scheduler.ctx); err != nil {
			return err
		}
		frame, info, err := d.socket.ZeroCopyReadPacketData()
		if errors.Is(err, afpacket.ErrTimeout) {
			stats, _, err := d.socket.SocketStats()
			if err != nil {
				return err
			}
			if err := d.checkDrops(stats.Drops()); err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			return err
		}
		if log.GetLevel() == log.DebugLevel {
			log.Debug("DAD frame received", "mac", d.mac,
				"captured-length", info.CaptureLength, "wire-length", info.Length,
				"frame", fmt.Sprintf("%x", frame))
		}
		if info.CaptureLength != info.Length {
			log.Debug("Ignoring truncated DAD frame")
			continue
		}
		if ip, ok := dadConflictAddress(frame, d.mac); ok {
			log.Debug("DAD conflict received", "mac", d.mac, "address", ip)
			d.scheduler.conflict(ip, fmt.Errorf("%w detected", errDuplicateAddress))
		} else {
			log.Debug("Ignoring DAD frame without a valid peer address")
		}
	}
	return fmt.Errorf("%w: receive queue overloaded", errDADObservationLost)
}

func (d *DADClient) checkDrops(drops uint) error {
	if drops <= d.drops {
		return nil
	}
	delta := drops - d.drops
	d.drops = drops
	return fmt.Errorf("%w: receive queue dropped %d packets", errDADObservationLost, delta)
}

func dadProbe(mac net.HardwareAddr, ip netip.Addr) ([]byte, error) {
	ethernet := &layers.Ethernet{
		SrcMAC:       mac,
		DstMAC:       net.HardwareAddr{255, 255, 255, 255, 255, 255},
		EthernetType: layers.EthernetTypeARP,
	}
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if ip.Is4() {
		arp := &layers.ARP{
			AddrType:          layers.LinkTypeEthernet,
			Protocol:          layers.EthernetTypeIPv4,
			Operation:         layers.ARPRequest,
			SourceHwAddress:   mac,
			SourceProtAddress: net.IPv4zero.To4(),
			DstHwAddress:      make([]byte, 6),
			DstProtAddress:    ip.AsSlice(),
		}
		err := gopacket.SerializeLayers(buffer, options, ethernet, arp)
		return buffer.Bytes(), err
	}
	target := ip.As16()
	destination := net.ParseIP("ff02::1:ff00:0").To16()
	copy(destination[13:], target[13:])
	ethernet.EthernetType = layers.EthernetTypeIPv6
	ethernet.DstMAC = net.HardwareAddr{0x33, 0x33, 0xff, target[13], target[14], target[15]}
	ipv6 := &layers.IPv6{
		Version:    6,
		HopLimit:   255,
		NextHeader: layers.IPProtocolICMPv6,
		SrcIP:      net.IPv6zero,
		DstIP:      destination,
	}
	icmp := &layers.ICMPv6{
		TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeNeighborSolicitation, 0),
	}
	if err := icmp.SetNetworkLayerForChecksum(ipv6); err != nil {
		return nil, err
	}
	err := gopacket.SerializeLayers(
		buffer,
		options,
		ethernet,
		ipv6,
		icmp,
		&layers.ICMPv6NeighborSolicitation{TargetAddress: ip.AsSlice()},
	)
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
			arp.HwAddressSize != 6 ||
			arp.ProtAddressSize != 4 ||
			(arp.Operation != layers.ARPRequest && arp.Operation != layers.ARPReply) {
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
	if solicitation, ok := packet.Layer(layers.LayerTypeICMPv6NeighborSolicitation).(*layers.ICMPv6NeighborSolicitation); ok &&
		ipv6.SrcIP.IsUnspecified() {
		return netip.AddrFromSlice(solicitation.TargetAddress)
	}
	return netip.Addr{}, false
}

var errDADClosed = errors.New("DAD scheduler closed")

type dadRequest struct {
	ip                     netip.Addr
	next                   time.Time
	attempts, index, users int
	done                   chan struct{}
	err                    error
}

type dadQueue []*dadRequest

func (q dadQueue) Len() int           { return len(q) }
func (q dadQueue) Less(i, j int) bool { return q[i].next.Before(q[j].next) }
func (q dadQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}
func (q *dadQueue) Push(x any) {
	r := x.(*dadRequest)
	r.index = len(*q)
	*q = append(*q, r)
}
func (q *dadQueue) Pop() any {
	old := *q
	r := old[len(old)-1]
	old[len(old)-1] = nil
	*q = old[:len(old)-1]
	r.index = -1
	return r
}

// dadScheduler uses a timer heap for probe sends and observation deadlines.
// The aggregate send rate is bounded independently of the pending address count.
type dadScheduler struct {
	ctx                   context.Context
	cancel                context.CancelCauseFunc
	mu                    sync.Mutex
	pending               map[netip.Addr]*dadRequest
	queue                 dadQueue
	wake                  chan struct{}
	done                  chan struct{}
	interval, observation time.Duration
	nextSend              time.Time
	burstLeft             int
	send                  func(netip.Addr) error
	drain                 func() error
}

func newDADScheduler(
	send func(netip.Addr) error,
	drain func() error,
	interval, observation time.Duration,
) *dadScheduler {
	ctx, cancel := context.WithCancelCause(context.Background())
	s := &dadScheduler{
		ctx:         ctx,
		cancel:      cancel,
		pending:     make(map[netip.Addr]*dadRequest),
		wake:        make(chan struct{}, 1),
		done:        make(chan struct{}),
		interval:    interval,
		burstLeft:   dadSendBurst,
		observation: observation,
		send:        send,
		drain:       drain,
	}
	go s.run()
	return s
}
func (s *dadScheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *dadScheduler) Check(ctx context.Context, ip netip.Addr) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if err := context.Cause(s.ctx); err != nil {
		s.mu.Unlock()
		return err
	}
	r := s.pending[ip]
	if r == nil {
		r = &dadRequest{ip: ip, next: time.Now(), done: make(chan struct{})}
		s.pending[ip] = r
		heap.Push(&s.queue, r)
	}
	r.users++
	s.mu.Unlock()
	s.notify()
	defer func() {
		s.mu.Lock()
		r.users--
		if r.users == 0 && s.pending[ip] == r {
			s.finish(r, context.Cause(ctx))
		}
		s.mu.Unlock()
		s.notify()
	}()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case <-r.done:
		if err := context.Cause(s.ctx); err != nil {
			return err
		}
		return r.err
	}
}

// finish and heap changes require mu. Removing the map entry prevents late
// replies or cancellations from completing a request twice.
func (s *dadScheduler) finish(r *dadRequest, err error) {
	if s.pending[r.ip] != r {
		return
	}
	delete(s.pending, r.ip)
	if r.index >= 0 {
		heap.Remove(&s.queue, r.index)
	}
	r.err = err
	close(r.done)
}
func (s *dadScheduler) conflict(ip netip.Addr, err error) {
	s.mu.Lock()
	if r := s.pending[ip]; r != nil {
		s.finish(r, err)
	}
	s.mu.Unlock()
	s.notify()
}
func (s *dadScheduler) restartPending() {
	s.mu.Lock()
	now := time.Now()
	for _, r := range s.pending {
		r.attempts = 0
		r.next = now
	}
	heap.Init(&s.queue)
	s.mu.Unlock()
	s.notify()
}
func (s *dadScheduler) Close() error {
	s.cancel(errDADClosed)
	<-s.done
	if err := context.Cause(s.ctx); !errors.Is(err, errDADClosed) {
		return err
	}
	return nil
}
func (s *dadScheduler) run() {
	defer close(s.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		if len(s.queue) == 0 {
			s.mu.Unlock()
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		r := s.queue[0]
		due := r.next
		if r.attempts < dadProbeAttempts && s.nextSend.After(due) {
			due = s.nextSend
		}
		if delay := time.Until(due); delay > 0 {
			s.mu.Unlock()
			timer.Reset(delay)
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			case <-timer.C:
			}
			timer.Stop()
			continue
		}
		if r.attempts == dadProbeAttempts {
			// Drain already received packets before declaring an address free. The
			// receiver and this drain share a lock, including conflict dispatch.
			s.mu.Unlock()
			if err := s.drain(); err != nil {
				if errors.Is(err, errDADObservationLost) {
					s.restartPending()
					continue
				}
				s.cancel(err)
				return
			}
			s.mu.Lock()
			if s.pending[r.ip] == r && r.attempts == dadProbeAttempts {
				s.finish(r, nil)
			}
			s.mu.Unlock()
			continue
		}
		// Only this goroutine sends. Space bounded bursts from actual send
		// completion; scheduler pauses must not produce catch-up bursts.
		s.mu.Unlock()
		if err := s.send(r.ip); err != nil {
			s.cancel(err)
			return
		}
		now := time.Now()
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		s.burstLeft--
		if s.burstLeft == 0 {
			s.nextSend = now.Add(s.interval * dadSendBurst)
			s.burstLeft = dadSendBurst
		}
		if s.pending[r.ip] != r {
			s.mu.Unlock()
			continue
		}
		r.attempts++
		r.next = now.Add(s.observation)
		heap.Fix(&s.queue, r.index)
		s.mu.Unlock()
	}
}
