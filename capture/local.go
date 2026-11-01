package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

const (
	// localReorderWindow is how long the merged stream holds packets of local sockets back to
	// write them in timestamp order. Each socket hands over its packets within
	// localBlockTimeout, so a few times that covers the delivery skew between sockets.
	localReorderWindow = 100 * time.Millisecond
	// localBlockTimeout is how long the kernel fills a ring block before it hands it over.
	localBlockTimeout = 10 * time.Millisecond
	// localPollTimeout is how often a socket reader checks whether the capture ended.
	localPollTimeout = 100 * time.Millisecond
	// localReopenInterval is how often a reader tries to reopen the socket of an interface
	// that went down or disappeared.
	localReopenInterval = 200 * time.Millisecond
	// localDrainQuiet is how long after the end of a capture a reader keeps waiting for packets
	// still in the socket's ring: the kernel hands over a block within two block timeouts.
	localDrainQuiet = 5 * localBlockTimeout
	// localDrainTimeout bounds the drain while packets keep arriving.
	localDrainTimeout = 250 * time.Millisecond
	// localRingBlocks and localMinBlockSize size the receive ring of each socket: eight blocks
	// of at least 1 MiB, each larger than the snap length.
	localRingBlocks   = 8
	localMinBlockSize = 1 << 20
	// vlanHeaderLength is the length of the 802.1Q tag put back into frames whose tag the
	// kernel moved out of band.
	vlanHeaderLength = 4
)

// errReadTimeout is returned by a packet source that has no packet within the poll timeout.
var errReadTimeout = errors.New("no packet within the poll timeout")

// ExecFunc runs f in the network namespace of a node.
type ExecFunc func(ctx context.Context, f func(ns.NetNS) error) error

// LocalTarget is one interface of a node on this machine (Docker or Podman) to capture.
type LocalTarget struct {
	// Node is the node name as the topology spells it.
	Node string
	// Interface is the interface name inside the node.
	Interface string
	// Exec runs a function in the node's network namespace.
	Exec ExecFunc
}

// packetSource reads the packets of one interface.
type packetSource interface {
	// ReadPacket returns the next packet; the data is only valid until the next call. It
	// returns errReadTimeout when no packet arrived within the poll timeout.
	ReadPacket() ([]byte, gopacket.CaptureInfo, error)
	Close()
}

// openSourcesFunc opens one packet source per interface in the network namespace exec enters.
type openSourcesFunc func(
	ctx context.Context,
	exec ExecFunc,
	interfaces []string,
	snapLength int,
) ([]packetSource, error)

// CaptureLocal streams pcapng from interfaces of nodes on this machine to w, with the same
// output and end conditions as labruntime.PacketCapturer.Capture: one section, one Interface
// Description Block per target named <node>:<interface>, then the packets of every target in
// timestamp order. targets lists the interfaces of req.Targets, resolved, in the same order.
//
// Each interface is read through a packet socket opened in the node's network namespace, so
// nothing runs in the node and the node image needs no capture tools; it needs CAP_NET_RAW and
// the right to enter the namespace, so root. An interface that goes down is reopened once it is
// up again; one that disappears (its node stopped or was recreated) is reattached when it is
// back, and the time it was gone is reported as a gap through req.OnGap (or logged).
func CaptureLocal(
	ctx context.Context,
	req clablabruntime.CaptureRequest,
	targets []LocalTarget,
	w io.Writer,
) error {
	return localCapture{
		open:           openPacketSockets,
		window:         localReorderWindow,
		reopenInterval: localReopenInterval,
	}.run(ctx, req, targets, w)
}

// localCapture is a local capture with its socket opener and timings, which tests replace.
type localCapture struct {
	open           openSourcesFunc
	window         time.Duration
	reopenInterval time.Duration
}

func (l localCapture) run(
	ctx context.Context,
	req clablabruntime.CaptureRequest,
	targets []LocalTarget,
	w io.Writer,
) error {
	if err := ValidateRequest(&req); err != nil {
		return err
	}
	if len(targets) != len(req.Targets) {
		return fmt.Errorf("capture has %d resolved targets for %d requested ones",
			len(targets), len(req.Targets))
	}

	sources, err := l.openAll(ctx, targets, req.SnapLength)
	if err != nil {
		return err
	}

	names := make([]string, len(targets))
	for idx, target := range targets {
		names[idx] = target.Node + ":" + target.Interface
	}

	captureCtx, stop := context.WithCancel(ctx)
	defer stop()
	if req.Duration > 0 {
		var cancelTimeout context.CancelFunc
		captureCtx, cancelTimeout = context.WithTimeout(captureCtx, req.Duration)
		defer cancelTimeout()
	}

	out, err := NewOutput(w, names, req.SnapLength, req.PacketLimit, l.window, stop)
	if err != nil {
		for _, source := range sources {
			source.Close()
		}

		return err
	}

	var wg sync.WaitGroup
	for idx, source := range sources {
		reader := &localReader{
			capture: l,
			out:     out,
			id:      uint32(idx), //nolint:gosec // bounded by the targets
			target:  targets[idx],
			snap:    req.SnapLength,
			onGap:   req.OnGap,
		}
		wg.Go(func() {
			reader.run(captureCtx, source)
		})
	}
	wg.Wait()

	return out.Close()
}

// openAll opens the sockets of every target before anything is written, one namespace entry
// per node, so a capture that cannot start fails without output.
func (l localCapture) openAll(
	ctx context.Context,
	targets []LocalTarget,
	snapLength int,
) ([]packetSource, error) {
	type nodeTargets struct {
		exec    ExecFunc
		indexes []int
		names   []string
	}
	var order []string
	byNode := map[string]*nodeTargets{}
	for idx, target := range targets {
		group := byNode[target.Node]
		if group == nil {
			group = &nodeTargets{exec: target.Exec}
			byNode[target.Node] = group
			order = append(order, target.Node)
		}
		group.indexes = append(group.indexes, idx)
		group.names = append(group.names, target.Interface)
	}

	sources := make([]packetSource, len(targets))
	closeAll := func() {
		for _, source := range sources {
			if source != nil {
				source.Close()
			}
		}
	}
	for _, node := range order {
		group := byNode[node]
		opened, err := l.open(ctx, group.exec, group.names, snapLength)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("failed to start the capture on node %q: %w", node, err)
		}
		for idx, source := range opened {
			sources[group.indexes[idx]] = source
		}
	}

	return sources, nil
}

// localReader reads one interface into the merged stream.
type localReader struct {
	capture localCapture
	out     *Output
	id      uint32
	target  LocalTarget
	snap    int
	onGap   func(clablabruntime.CaptureGap)
}

func (r *localReader) run(ctx context.Context, source packetSource) {
	defer func() {
		if source != nil {
			source.Close()
		}
	}()

	for {
		if ctx.Err() != nil {
			r.drain(source, time.Now())
			return
		}

		data, info, err := source.ReadPacket()
		if errors.Is(err, errReadTimeout) {
			continue
		}
		if err != nil {
			// The socket reports an error once its interface went down or was removed, and
			// keeps reporting it: it is reopened.
			log.Debug("Capture socket failed, reopening it", "node", r.target.Node,
				"interface", r.target.Interface, "error", err)
			source.Close()
			source = r.reopen(ctx)
			if source == nil {
				return
			}

			continue
		}

		if more, err := r.out.Emit(r.id, r.packet(data, info)); !more || err != nil {
			return
		}
	}
}

// drain writes the packets that arrived before end but were still in the socket's ring: the
// kernel hands over a ring block once it is full or its block timeout passed, so the last
// packets before a capture ended may not have been read yet.
func (r *localReader) drain(source packetSource, end time.Time) {
	deadline := end.Add(localDrainTimeout)
	for time.Now().Before(deadline) {
		data, info, err := source.ReadPacket()
		if errors.Is(err, errReadTimeout) && time.Since(end) < localDrainQuiet {
			continue
		}
		if err != nil || info.Timestamp.After(end) {
			return
		}
		if more, err := r.out.Emit(r.id, r.packet(data, info)); !more || err != nil {
			return
		}
	}
}

// packet copies a packet out of the socket's ring, cut to the snap length.
func (r *localReader) packet(data []byte, info gopacket.CaptureInfo) *Packet {
	length := info.Length
	for _, ancillary := range info.AncillaryData {
		if _, ok := ancillary.(afpacket.AncillaryVLAN); ok {
			// the tag was put back into the data and counts toward the frame length
			length += vlanHeaderLength
			break
		}
	}
	length = max(length, len(data))

	captured := make([]byte, min(len(data), r.snap))
	copy(captured, data)

	return &Packet{
		Timestamp:      info.Timestamp.UnixNano(),
		OriginalLength: uint32(length), //nolint:gosec // bounded by the interface MTU
		Data:           captured,
	}
}

// reopen opens the interface's socket again once its node and interface are back. It returns
// nil when the capture ended first. Time the interface could not be opened is reported as a gap;
// an interface that only went down is not, as it has no traffic while down.
func (r *localReader) reopen(ctx context.Context) packetSource {
	from := time.Now()
	var unavailable error

	timer := time.NewTimer(r.capture.reopenInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			if unavailable != nil {
				r.reportGap(from, time.Time{}, unavailable)
			}

			return nil
		case <-timer.C:
		}

		opened, err := r.capture.open(ctx, r.target.Exec, []string{r.target.Interface}, r.snap)
		if err != nil {
			if unavailable == nil {
				log.Debug("Capture interface is not available, waiting for it",
					"node", r.target.Node, "interface", r.target.Interface, "error", err)
			}
			unavailable = err
			resetTimer(timer, r.capture.reopenInterval)

			continue
		}
		if unavailable != nil {
			r.reportGap(from, time.Now(), unavailable)
		}

		return opened[0]
	}
}

func (r *localReader) reportGap(from, to time.Time, cause error) {
	ReportGap(r.onGap, clablabruntime.CaptureGap{
		Node:       r.target.Node,
		Interfaces: []string{r.target.Interface},
		From:       from,
		To:         to,
		Reason:     fmt.Sprintf("the interface was not available: %v", cause),
	})
}

// openPacketSockets opens one packet socket per interface in the namespace exec enters. A socket
// stays bound to its interface after the thread left the namespace.
func openPacketSockets(
	ctx context.Context,
	exec ExecFunc,
	interfaces []string,
	snapLength int,
) ([]packetSource, error) {
	sources := make([]packetSource, 0, len(interfaces))
	err := exec(ctx, func(ns.NetNS) error {
		for _, name := range interfaces {
			source, err := openPacketSocket(name, snapLength)
			if err != nil {
				return fmt.Errorf("interface %q: %w", name, err)
			}
			sources = append(sources, source)
		}

		return nil
	})
	if err != nil {
		for _, source := range sources {
			source.Close()
		}

		return nil, err
	}

	return sources, nil
}

// afpacketSource is a TPACKET_V3 receive ring on one interface.
type afpacketSource struct {
	handle *afpacket.TPacket
}

func openPacketSocket(name string, snapLength int) (*afpacketSource, error) {
	// A TPACKET_V3 block holds whole frames: it must be larger than the snap length, plus the
	// block and frame headers, and a multiple of the page size.
	pageSize := os.Getpagesize()
	blockSize := max(localMinBlockSize, snapLength+pageSize)
	blockSize = (blockSize + pageSize - 1) / pageSize * pageSize

	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(name),
		afpacket.OptTPacketVersion(afpacket.TPacketVersion3),
		afpacket.OptFrameSize(blockSize),
		afpacket.OptBlockSize(blockSize),
		afpacket.OptNumBlocks(localRingBlocks),
		afpacket.OptBlockTimeout(localBlockTimeout),
		afpacket.OptPollTimeout(localPollTimeout),
		// the kernel moves 802.1Q tags out of the frame; captures show them in place
		afpacket.OptAddVLANHeader(true),
	)
	if err != nil {
		return nil, err
	}
	// promiscuous mode is a membership of this socket and ends with it
	if err := handle.SetPromiscuous(true); err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to enable promiscuous mode: %w", err)
	}

	return &afpacketSource{handle: handle}, nil
}

func (s *afpacketSource) ReadPacket() ([]byte, gopacket.CaptureInfo, error) {
	data, info, err := s.handle.ZeroCopyReadPacketData()
	if errors.Is(err, afpacket.ErrTimeout) {
		return nil, info, errReadTimeout
	}

	return data, info, err
}

func (s *afpacketSource) Close() {
	s.handle.Close()
}
