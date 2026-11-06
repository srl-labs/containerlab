package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// fakeFrame is one packet a fake source returns.
type fakeFrame struct {
	data   []byte
	stamp  time.Time
	length int
	vlan   bool
	// visible delays the frame like a ring block the kernel has not handed over yet.
	visible time.Time
	// err ends the source with this error instead of a packet.
	err error
}

// fakeSource returns frames from a channel, errReadTimeout while none is queued.
type fakeSource struct {
	frames  chan fakeFrame
	closed  chan struct{}
	once    sync.Once
	pending *fakeFrame
}

func newFakeSource() *fakeSource {
	return &fakeSource{frames: make(chan fakeFrame, 64), closed: make(chan struct{})}
}

func (s *fakeSource) ReadPacket() ([]byte, gopacket.CaptureInfo, error) {
	if s.pending == nil {
		select {
		case frame := <-s.frames:
			s.pending = &frame
		case <-time.After(5 * time.Millisecond):
			return nil, gopacket.CaptureInfo{}, errReadTimeout
		}
	}
	if wait := time.Until(s.pending.visible); wait > 0 {
		time.Sleep(min(wait, 5*time.Millisecond))
		if time.Now().Before(s.pending.visible) {
			return nil, gopacket.CaptureInfo{}, errReadTimeout
		}
	}

	{
		frame := *s.pending
		s.pending = nil
		if frame.err != nil {
			return nil, gopacket.CaptureInfo{}, frame.err
		}
		info := gopacket.CaptureInfo{
			Timestamp: frame.stamp, CaptureLength: len(frame.data), Length: frame.length,
		}
		if frame.vlan {
			info.AncillaryData = []any{afpacket.AncillaryVLAN{VLAN: 10}}
		}

		return frame.data, info, nil
	}
}

func (s *fakeSource) Close() {
	s.once.Do(func() { close(s.closed) })
}

// fakeHost hands out fake sources per node and interface. Opening an interface listed in
// missing fails, like an interface whose node does not run.
type fakeHost struct {
	mu      sync.Mutex
	opened  map[string][]*fakeSource
	missing map[string]bool
	ready   chan string
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		opened: map[string][]*fakeSource{}, missing: map[string]bool{},
		ready: make(chan string, 64),
	}
}

func (h *fakeHost) exec(node string) ExecFunc {
	return func(_ context.Context, f func(ns.NetNS) error) error {
		h.mu.Lock()
		missing := h.missing[node]
		h.mu.Unlock()
		if missing {
			return fmt.Errorf("node %q: container network namespace is not available", node)
		}

		return f(nil)
	}
}

func (h *fakeHost) open(
	ctx context.Context,
	exec ExecFunc,
	interfaces []string,
	_ int,
) ([]packetSource, error) {
	var node string
	err := exec(ctx, func(ns.NetNS) error { return nil })
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	sources := make([]packetSource, 0, len(interfaces))
	for _, name := range interfaces {
		if h.missing[name] {
			return nil, fmt.Errorf("interface %q: no such network interface", name)
		}
		node = name
		source := newFakeSource()
		h.opened[name] = append(h.opened[name], source)
		sources = append(sources, source)
		h.ready <- node
	}

	return sources, nil
}

func (h *fakeHost) source(name string, generation int) *fakeSource {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.opened[name][generation]
}

func (h *fakeHost) setMissing(name string, missing bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.missing[name] = missing
}

func (h *fakeHost) capture() localCapture {
	return localCapture{
		open:           h.open,
		window:         20 * time.Millisecond,
		reopenInterval: time.Millisecond,
	}
}

// waitOpened waits until n sources were opened.
func (h *fakeHost) waitOpened(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-h.ready:
		case <-time.After(2 * time.Second):
			t.Fatal("source was not opened")
		}
	}
}

type parsedLocalCapture struct {
	interfaces []string
	packets    []Packet
}

func parseLocalCapture(t *testing.T, data []byte) parsedLocalCapture {
	t.Helper()

	var parsed parsedLocalCapture
	reader := NewReader(bytes.NewReader(data))
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("invalid capture: %v", err)
		}
		if event.Interface != nil {
			parsed.interfaces = append(parsed.interfaces, event.Interface.Name)
		}
		if event.Packet != nil {
			parsed.packets = append(parsed.packets, *event.Packet)
		}
	}
	if reader.Sections() != 1 {
		t.Fatalf("capture has %d sections", reader.Sections())
	}

	return parsed
}

func localRequest(packetLimit int, duration time.Duration) clablabruntime.CaptureRequest {
	return clablabruntime.CaptureRequest{
		Name: "lab",
		Targets: []clablabruntime.CaptureTarget{
			{Node: "r1", Interface: "eth1"}, {Node: "r2", Interface: "eth1"},
		},
		PacketLimit: packetLimit,
		Duration:    duration,
		SnapLength:  64,
	}
}

func TestCaptureLocalMergesInterfacesInTimestampOrder(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	// the sources are keyed by interface name in the fake host, so the two nodes use distinct
	// names behind the requested ones
	targets := []LocalTarget{
		{Node: "r1", Interface: "e1", Exec: host.exec("r1")},
		{Node: "r2", Interface: "e2", Exec: host.exec("r2")},
	}

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- host.capture().run(context.Background(), localRequest(4, 0), targets, &out)
	}()
	host.waitOpened(t, 2)

	base := time.Unix(1_761_559_200, 0)
	// r2's packets reach containerlab first although r1 stamped one of them earlier
	host.source("e2", 0).frames <- fakeFrame{
		data: []byte("r2-a"), stamp: base.Add(2 * time.Millisecond), length: 4,
	}
	host.source("e2", 0).frames <- fakeFrame{
		data: bytes.Repeat([]byte{7}, 100), stamp: base.Add(4 * time.Millisecond), length: 100,
	}
	host.source("e1", 0).frames <- fakeFrame{
		data: []byte("r1-a"), stamp: base.Add(1 * time.Millisecond), length: 4,
	}
	host.source("e1", 0).frames <- fakeFrame{
		data: []byte("r1-v"), stamp: base.Add(3 * time.Millisecond), length: 60, vlan: true,
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capture did not stop at the packet limit")
	}

	parsed := parseLocalCapture(t, out.Bytes())
	if want := []string{"r1:e1", "r2:e2"}; !slices.Equal(parsed.interfaces, want) {
		t.Fatalf("interfaces = %v, want %v", parsed.interfaces, want)
	}
	if len(parsed.packets) != 4 {
		t.Fatalf("captured %d packets, want 4", len(parsed.packets))
	}
	var ids []uint32
	for idx, packet := range parsed.packets {
		ids = append(ids, packet.InterfaceID)
		if want := base.Add(time.Duration(idx+1) * time.Millisecond).
			UnixNano(); packet.Timestamp != want {
			t.Fatalf("packet %d stamped %d, want %d", idx, packet.Timestamp, want)
		}
	}
	if !slices.Equal(ids, []uint32{0, 1, 0, 1}) {
		t.Fatalf("interface IDs = %v", ids)
	}
	// a re-inserted VLAN tag counts toward the frame length
	if parsed.packets[2].OriginalLength != 64 {
		t.Fatalf("VLAN frame length = %d, want 64", parsed.packets[2].OriginalLength)
	}
	// frames are cut to the snap length
	if last := parsed.packets[3]; len(last.Data) != 64 || last.OriginalLength != 100 {
		t.Fatalf("long frame kept %d of %d bytes", len(last.Data), last.OriginalLength)
	}
	for _, name := range []string{"e1", "e2"} {
		select {
		case <-host.source(name, 0).closed:
		default:
			t.Fatalf("source %s was not closed", name)
		}
	}
}

func TestCaptureLocalEndsAfterTheDurationOrWhenCanceled(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	targets := []LocalTarget{
		{Node: "r1", Interface: "e1", Exec: host.exec("r1")},
		{Node: "r1", Interface: "e2", Exec: host.exec("r1")},
	}

	var out bytes.Buffer
	start := time.Now()
	if err := host.capture().run(context.Background(), localRequest(0, 50*time.Millisecond),
		targets, &out); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("capture ran %s for a 50ms duration", elapsed)
	}
	if parsed := parseLocalCapture(t, out.Bytes()); len(parsed.interfaces) != 2 {
		t.Fatalf("interfaces = %v", parsed.interfaces)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- host.capture().run(ctx, localRequest(0, 0), targets, io.Discard)
	}()
	host.waitOpened(t, 4)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("canceled capture returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled capture did not end")
	}
}

func TestCaptureLocalWritesPacketsStillInTheRingWhenItEnds(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	targets := []LocalTarget{{Node: "r1", Interface: "e1", Exec: host.exec("r1")}}
	req := localRequest(0, 0)
	req.Targets = req.Targets[:1]

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- host.capture().run(ctx, req, targets, &out)
	}()
	host.waitOpened(t, 1)

	now := time.Now()
	// stamped before the end, handed over by the kernel only after it
	host.source("e1", 0).frames <- fakeFrame{
		data: []byte("before"), stamp: now, length: 6, visible: now.Add(30 * time.Millisecond),
	}
	// stamped after the end: not part of the capture
	host.source("e1", 0).frames <- fakeFrame{
		data: []byte("after"), stamp: now.Add(time.Hour), length: 5,
		visible: now.Add(40 * time.Millisecond),
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	parsed := parseLocalCapture(t, out.Bytes())
	if len(parsed.packets) != 1 || string(parsed.packets[0].Data) != "before" {
		t.Fatalf("packets = %+v, want only the one stamped before the end", parsed.packets)
	}
}

func TestCaptureLocalFailsWithoutOutputWhenAnInterfaceCannotBeOpened(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	host.setMissing("r2", true)
	targets := []LocalTarget{
		{Node: "r1", Interface: "e1", Exec: host.exec("r1")},
		{Node: "r2", Interface: "e2", Exec: host.exec("r2")},
	}

	var out bytes.Buffer
	err := host.capture().run(context.Background(), localRequest(0, 0), targets, &out)
	if err == nil || !strings.Contains(err.Error(), `failed to start the capture on node "r2"`) {
		t.Fatalf("error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("wrote %d bytes before failing", out.Len())
	}
	select {
	case <-host.source("e1", 0).closed:
	default:
		t.Fatal("the source opened before the failure was not closed")
	}
}

func TestCaptureLocalReopensInterfacesAndReportsWhileTheyWereGone(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	targets := []LocalTarget{{Node: "r1", Interface: "e1", Exec: host.exec("r1")}}
	req := localRequest(2, 0)
	req.Targets = req.Targets[:1]

	var mu sync.Mutex
	var gaps []clablabruntime.CaptureGap
	req.OnGap = func(gap clablabruntime.CaptureGap) {
		mu.Lock()
		defer mu.Unlock()
		gaps = append(gaps, gap)
	}

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- host.capture().run(context.Background(), req, targets, &out)
	}()
	host.waitOpened(t, 1)

	base := time.Unix(1_761_559_200, 0)
	host.source("e1", 0).frames <- fakeFrame{data: []byte("a"), stamp: base, length: 1}

	// the interface goes down: the socket fails and is reopened at once, without a gap
	host.source("e1", 0).frames <- fakeFrame{err: afpacket.ErrPoll}
	host.waitOpened(t, 1)

	// the node restarts: the interface is gone for a while, then back
	host.setMissing("r1", true)
	host.source("e1", 1).frames <- fakeFrame{err: afpacket.ErrPoll}
	time.Sleep(20 * time.Millisecond)
	host.setMissing("r1", false)
	host.waitOpened(t, 1)
	host.source("e1", 2).frames <- fakeFrame{
		data: []byte("b"), stamp: base.Add(time.Second), length: 1,
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capture did not stop at the packet limit")
	}

	if parsed := parseLocalCapture(t, out.Bytes()); len(parsed.packets) != 2 {
		t.Fatalf("captured %d packets across the reopen, want 2", len(parsed.packets))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(gaps) != 1 {
		t.Fatalf("gaps = %+v, want one for the restart", gaps)
	}
	gap := gaps[0]
	if gap.Node != "r1" || !slices.Equal(gap.Interfaces, []string{"e1"}) || gap.To.IsZero() ||
		!gap.To.After(gap.From) || !strings.Contains(gap.Reason, "not available") {
		t.Fatalf("gap = %+v", gap)
	}
}

func TestCaptureLocalChecksTheRequest(t *testing.T) {
	t.Parallel()

	host := newFakeHost()
	targets := []LocalTarget{{Node: "r1", Interface: "e1", Exec: host.exec("r1")}}

	req := localRequest(0, 0)
	req.SnapLength = 10
	if err := host.capture().run(context.Background(), req, targets[:1], io.Discard); err == nil ||
		!strings.Contains(err.Error(), "snap length 10") {
		t.Fatalf("small snap length: error = %v", err)
	}

	if err := host.capture().run(context.Background(), localRequest(0, 0), targets,
		io.Discard); err == nil || !strings.Contains(err.Error(), "1 resolved targets for 2") {
		t.Fatalf("target mismatch: error = %v", err)
	}
}
