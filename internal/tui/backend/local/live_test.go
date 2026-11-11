// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	clabcapture "github.com/srl-labs/containerlab/capture"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// captureStream builds what the capture command writes: a section with the given
// interfaces, to which packets are added.
type captureStream struct {
	buf    bytes.Buffer
	writer *clabcapture.Writer
}

func newCaptureStream(t *testing.T, interfaces ...string) *captureStream {
	t.Helper()

	s := &captureStream{}
	s.writer = clabcapture.NewWriter(&s.buf)

	if err := s.writer.WriteHeader(interfaces, 262144); err != nil {
		t.Fatal(err)
	}

	return s
}

func (s *captureStream) packet(t *testing.T, iface uint32, at time.Time, data []byte) {
	t.Helper()

	err := s.writer.WritePacket(iface, &clabcapture.Packet{
		Timestamp:      at.UnixNano(),
		OriginalLength: uint32(len(data)), //nolint:gosec // a test frame
		Data:           data,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// take returns what was written since the last call.
func (s *captureStream) take() string {
	defer s.buf.Reset()

	return s.buf.String()
}

func pingFrame(t *testing.T, seq uint16) []byte {
	t.Helper()

	return frame(t, ethernet(layers.EthernetTypeIPv4), ipv4(layers.IPProtocolICMPv4),
		&layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       7, Seq: seq,
		}, gopacket.Payload("abcdefgh"))
}

// liveCapture starts a live capture of the fabric lab and returns the job with the
// capture process.
func liveCapture(
	t *testing.T, b *testBackend, file string,
) (clabinternaltuibackend.JobID, *fakeProc) {
	t.Helper()

	b.setLabs(fabricContainers(t)...)

	id, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab: "fabric",
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1", "e1-2"}},
		},
		Live: true,
		File: file,
	})
	if err != nil {
		t.Fatal(err)
	}

	return id, b.run.proc(t, 0)
}

func TestLiveCapture(t *testing.T) {
	b := newTestBackend(t)
	id, capture := liveCapture(t, b, "")
	topo := absTestdata(t, "fabric.clab.yml")

	wantArgs := argv("capture -r docker -t " + topo + " --target leaf1:e1-1,e1-2 -w -")
	if diff := cmp.Diff(wantArgs, capture.spec.args); diff != "" {
		t.Errorf("unexpected capture arguments (-want +got):\n%s", diff)
	}

	info, _ := b.Job(id)
	if !info.Live || info.Kind != clabinternaltuibackend.JobCapture ||
		info.Command != "containerlab "+strings.Join(wantArgs, " ") {
		t.Errorf("unexpected job: %+v", info)
	}

	if busy := b.jobs.busyLabs(); busy.has("fabric", topo) {
		t.Error("a live capture marks its lab busy")
	}

	stream := newCaptureStream(t, "leaf1:e1-1", "leaf1:e1-2")
	start := time.Date(2026, time.November, 11, 10, 0, 0, 123456000, time.UTC)

	stream.packet(t, 0, start, pingFrame(t, 1))
	stream.packet(t, 1, start.Add(time.Millisecond), pingFrame(t, 2))

	// The stream arrives in pieces that do not end where the packets do.
	data := stream.take()
	total := len(data)

	b.drainTopics()
	capture.stdout(data[:30])
	capture.stdout(data[30 : len(data)-7])

	eventually(t, "the first packet", func() bool { return len(b.Packets(id, 0)) == 1 })
	capture.stdout(data[len(data)-7:])
	eventually(t, "the second packet", func() bool { return len(b.Packets(id, 0)) == 2 })

	want := []clabinternaltuibackend.Packet{
		{
			Number: 1, Time: start, Interface: "leaf1:e1-1", Source: "10.0.0.1",
			Destination: "10.0.0.2", Protocol: "ICMP", Length: 60,
			Info: "echo request id=7 seq=1",
		},
		{
			Number: 2, Time: start.Add(time.Millisecond), Interface: "leaf1:e1-2",
			Source: "10.0.0.1", Destination: "10.0.0.2", Protocol: "ICMP", Length: 60,
			Info: "echo request id=7 seq=2",
		},
	}

	got := b.Packets(id, 0)
	for i := range got {
		got[i].Time = got[i].Time.UTC()
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected packets (-want +got):\n%s", diff)
	}

	// Packets are read from a position on, like the lines of output.
	if rest := b.Packets(id, 1); len(rest) != 1 || rest[0].Number != 2 {
		t.Errorf("packets from position 1: %+v", rest)
	}

	if rest := b.Packets(id, 2); len(rest) != 0 {
		t.Errorf("packets from position 2: %+v", rest)
	}

	eventually(t, "the UI to be told about the packets", func() bool {
		return b.drainTopics().Has(clabinternaltuibackend.TopicJobOutput)
	})

	info, _ = b.Job(id)
	if info.Packets != 2 || info.Bytes != uint64(total) {
		t.Errorf("the job counts %d packets and %d bytes, want 2 and %d",
			info.Packets, info.Bytes, total)
	}

	detail, ok := b.PacketDetail(id, 2)
	if !ok {
		t.Fatal("no details for packet 2")
	}

	names := make([]string, len(detail.Layers))
	for i := range detail.Layers {
		names[i] = detail.Layers[i].Name
	}

	if strings.Join(names, ", ") != "Ethernet, IPv4, ICMP, Data" || detail.Number != 2 ||
		detail.Captured != 60 || !bytes.Equal(detail.Data, pingFrame(t, 2)) {
		t.Errorf("unexpected details: %+v", detail)
	}

	if _, ok := b.PacketDetail(id, 3); ok {
		t.Error("details for a packet that does not exist")
	}

	// The binary data is no output of the job.
	capture.stderr("10:00:00 INFO Capturing interfaces=2\n")

	if out := b.JobOutput(id, 0); len(out) != 1 || out[0].Text != "Capturing interfaces=2" {
		t.Errorf("unexpected job output: %+v", out)
	}

	// Stopping the capture is its regular end, and the packets stay to look at.
	capture.onSignal = func(syscall.Signal) {
		stream.packet(t, 0, start.Add(time.Second), pingFrame(t, 3))
		capture.stdout(stream.take())
		capture.exitWith(0)
	}

	b.CancelJob(id)

	if info := b.awaitJob(t, id); info.State != clabinternaltuibackend.JobSucceeded ||
		info.Packets != 3 {
		t.Errorf("unexpected job: %+v", info)
	}

	if len(b.Packets(id, 0)) != 3 {
		t.Error("the packets are gone with the capture")
	}
}

func TestLiveCaptureIntoFile(t *testing.T) {
	b := newTestBackend(t)

	dir := t.TempDir()
	id, capture := liveCapture(t, b, filepath.Join(dir, "out.pcapng"))

	stream := newCaptureStream(t, "leaf1:e1-1", "leaf1:e1-2")
	stream.packet(t, 0, time.Unix(1, 0), pingFrame(t, 1))

	data := stream.take()
	capture.stdout(data)
	capture.exitWith(0)

	if info := b.awaitJob(t, id); info.State != clabinternaltuibackend.JobSucceeded {
		t.Errorf("unexpected job: %+v", info)
	}

	// The file holds the capture as it came, complete once the job is through.
	eventually(t, "the capture file", func() bool {
		written, err := os.ReadFile(filepath.Join(dir, "out.pcapng"))

		return err == nil && string(written) == data
	})

	// The job tells where the packets went.
	out := b.JobOutput(id, 0)
	if len(out) != 1 || !strings.HasSuffix(out[0].Text, filepath.Join(dir, "out.pcapng")) {
		t.Errorf("unexpected job output: %+v", out)
	}

	if len(b.Packets(id, 0)) != 1 {
		t.Error("the packet was not decoded")
	}
}

func TestLiveCaptureFileThatCannotBeWritten(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	_, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab: "fabric",
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1"}},
		},
		Live: true,
		File: filepath.Join(t.TempDir(), "missing", "out.pcapng"),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot write the capture file") {
		t.Errorf("unexpected error: %v", err)
	}

	if b.run.count() != 0 {
		t.Error("a capture was started without a place for its file")
	}
}

// TestLiveCaptureThatCannotBeDecoded makes the command write something that is no
// capture: the job fails and tells why, and the command is never kept waiting.
func TestLiveCaptureThatCannotBeDecoded(t *testing.T) {
	b := newTestBackend(t)
	id, capture := liveCapture(t, b, "")

	capture.onSignal = func(syscall.Signal) { capture.exitWith(0) }
	capture.stdout(strings.Repeat("this is not a capture ", 64))

	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobFailed ||
		!strings.Contains(info.Err, "cannot decode the capture") {
		t.Errorf("unexpected job: %+v", info)
	}

	// Whatever the command still writes goes nowhere, without blocking it.
	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 64 {
			capture.stdout(strings.Repeat("x", 4096))
		}
	}()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("the command is kept waiting for a decoder that is gone")
	}
}

func TestLiveCaptureThatFailsRightAway(t *testing.T) {
	b := newTestBackend(t)
	id, capture := liveCapture(t, b, "")

	capture.stderr("10:00:00 ERRO node \"leaf1\" has no interface \"e1-1\"\n")
	capture.exitWith(1)

	info := b.awaitJob(t, id)
	if info.State != clabinternaltuibackend.JobFailed ||
		info.Err != `node "leaf1" has no interface "e1-1"` {
		t.Errorf("unexpected job: %+v", info)
	}
}

func TestLiveCaptureKeepsTheStartOfLargePackets(t *testing.T) {
	b := newTestBackend(t)
	id, capture := liveCapture(t, b, "")

	large := frame(t, ethernet(layers.EthernetTypeIPv4), ipv4(layers.IPProtocolUDP),
		gopacket.Payload(make([]byte, 8000)))

	stream := newCaptureStream(t, "leaf1:e1-1", "leaf1:e1-2")
	stream.packet(t, 0, time.Unix(1, 0), large)
	capture.stdout(stream.take())

	eventually(t, "the packet", func() bool { return len(b.Packets(id, 0)) == 1 })

	detail, _ := b.PacketDetail(id, 1)
	if len(detail.Data) != maxPacketBytes || detail.Captured != len(large) ||
		detail.Length != len(large) {
		t.Errorf("kept %d of %d bytes, the packet has %d", len(detail.Data), detail.Captured,
			detail.Length)
	}
}

func TestRing(t *testing.T) {
	r := ring[int]{limit: 3}

	for i := range 5 {
		r.add(i)
	}

	if diff := cmp.Diff([]int{2, 3, 4}, r.from(0)); diff != "" {
		t.Errorf("unexpected items (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]int{4}, r.from(4)); diff != "" {
		t.Errorf("unexpected items from 4 (-want +got):\n%s", diff)
	}

	if r.from(5) != nil {
		t.Error("items past the end")
	}

	for n, want := range map[int]bool{1: false, 2: true, 4: true, 5: false, -1: false} {
		item, ok := r.at(n)
		if ok != want || (ok && item != n) {
			t.Errorf("at(%d) = %d, %v", n, item, ok)
		}
	}
}
