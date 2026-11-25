// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	clabcapture "github.com/srl-labs/containerlab/capture"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// maxLivePackets is the number of packets kept of a live capture.
	maxLivePackets = 10000
	// maxPacketBytes bounds what is kept of a packet for its details. The headers the
	// details are about sit at the start.
	maxPacketBytes = 2048
	// captureFileMode is the mode of the file a live capture is written to.
	captureFileMode = 0o644
)

// livePacket is a packet of a live capture: the line that sums it up and the start of its
// data.
type livePacket struct {
	summary clabinternaltuibackend.Packet
	data    []byte
	// captured is how many bytes of the packet the capture holds.
	captured int
}

// liveFeed takes the capture data a command writes to its standard output and passes it
// on to the decoder, and to a file when the capture is kept. It never fails: the command
// must be able to go on writing whatever happens to the readers.
type liveFeed struct {
	// count is told the size of every piece of data.
	count func(n int)
	// failed is called with the reason when the file cannot be written.
	failed func(reason string)

	mu      sync.Mutex
	decoder *io.PipeWriter
	file    *os.File
}

// Write implements io.Writer.
func (f *liveFeed) Write(p []byte) (int, error) {
	f.count(len(p))

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file != nil {
		if _, err := f.file.Write(p); err != nil {
			f.file.Close()
			f.file = nil
			f.failed(fmt.Sprintf("cannot write the capture file: %v", err))
		}
	}

	if f.decoder != nil {
		if _, err := f.decoder.Write(p); err != nil {
			f.decoder = nil
		}
	}

	return len(p), nil
}

// close ends the feed, which tells the decoder that the capture is complete. It returns
// the error of closing the capture file: the end of the capture may then be missing
// from it.
func (f *liveFeed) close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.decoder != nil {
		f.decoder.Close()
		f.decoder = nil
	}

	if f.file == nil {
		return nil
	}

	err := f.file.Close()
	f.file = nil

	return err
}

// captureLive starts a capture whose packets are decoded for the UI.
func (b *Backend) captureLive(
	req *clabinternaltuibackend.CaptureRequest,
	spec *jobSpec,
) (clabinternaltuibackend.JobID, error) {
	stream, decoder := io.Pipe()
	feed := &liveFeed{decoder: decoder}

	file := ""

	if req.File != "" {
		var err error

		if file, err = filepath.Abs(req.File); err != nil {
			return 0, err
		}

		feed.file, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, captureFileMode)
		if err != nil {
			return 0, fmt.Errorf("cannot write the capture file: %w", err)
		}
	}

	spec.args = b.captureCommand(req, "-")
	spec.live = true
	spec.output = func(j *job) io.Writer {
		feed.count = func(n int) { b.jobs.addBytes(j, n) }
		feed.failed = func(reason string) { b.jobs.abort(j, reason) }

		return feed
	}

	j, err := b.jobs.start(spec)
	if err != nil {
		// Nothing was written to the file, closing it has nothing to lose.
		_ = feed.close()
		stream.Close()

		return 0, err
	}

	if file != "" {
		j.note(b.clock.Now(), levelInfo, "the packets are also written to "+file)
	}

	b.spawn("capture decoder", func() { b.decodeCapture(j, stream) })
	b.spawn("capture end", func() {
		<-j.done

		if err := feed.close(); err != nil {
			j.note(b.clock.Now(), levelWarn,
				fmt.Sprintf("the capture file may be incomplete: %v", err))
			b.jobs.output.poke()
		}
	})

	return j.info.ID, nil
}

// decodeCapture reads the capture of a job and keeps its packets for the UI.
func (b *Backend) decodeCapture(j *job, stream *io.PipeReader) {
	// Whatever ends the decoding, the command must not wait for it.
	defer stream.Close()

	var (
		reader     = clabcapture.NewReader(stream)
		sections   int
		interfaces []string
	)

	for {
		event, err := reader.Next()
		if err != nil {
			// A capture that was cut short is at its end as well, only a capture that
			// cannot be read is a failure.
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				b.jobs.abort(j, fmt.Sprintf("cannot decode the capture: %v", err))
			}

			return
		}

		if reader.Sections() != sections {
			sections = reader.Sections()
			interfaces = interfaces[:0]
		}

		switch {
		case event.Interface != nil:
			interfaces = append(interfaces, event.Interface.Name)
		case event.Packet != nil:
			name := ""
			if id := int(event.Packet.InterfaceID); id < len(interfaces) {
				name = interfaces[id]
			}

			b.jobs.addPacket(j, name, event.Packet)
		}
	}
}

// addPacket adds a captured packet to its job.
func (m *jobManager) addPacket(j *job, iface string, packet *clabcapture.Packet) {
	decoded := dissect(packet.Data, false)

	// The data is copied when it is cut, for the rest of the packet to be released.
	data := packet.Data
	if len(data) > maxPacketBytes {
		data = slices.Clone(data[:maxPacketBytes])
	}

	j.mu.Lock()

	j.packets.add(livePacket{
		summary: clabinternaltuibackend.Packet{
			Number:      j.packets.total + 1,
			Time:        time.Unix(0, packet.Timestamp),
			Interface:   iface,
			Source:      decoded.src,
			Destination: decoded.dst,
			Protocol:    decoded.proto,
			Length:      int(packet.OriginalLength),
			Info:        decoded.summary(),
		},
		data:     data,
		captured: len(packet.Data),
	})
	j.info.Packets = j.packets.total

	j.mu.Unlock()

	m.output.poke()
}

// Packets returns the packets of a live capture from the given position on.
func (b *Backend) Packets(
	id clabinternaltuibackend.JobID,
	from int,
) []clabinternaltuibackend.Packet {
	j := b.jobs.get(id)
	if j == nil {
		return nil
	}

	j.mu.Lock()
	kept := j.packets.from(from)
	j.mu.Unlock()

	packets := make([]clabinternaltuibackend.Packet, len(kept))
	for i := range kept {
		packets[i] = kept[i].summary
	}

	return packets
}

// PacketDetail returns a packet of a live capture with its protocol headers.
func (b *Backend) PacketDetail(
	id clabinternaltuibackend.JobID,
	number int,
) (clabinternaltuibackend.PacketDetail, bool) {
	j := b.jobs.get(id)
	if j == nil {
		return clabinternaltuibackend.PacketDetail{}, false
	}

	j.mu.Lock()
	packet, ok := j.packets.at(number - 1)
	j.mu.Unlock()

	if !ok {
		return clabinternaltuibackend.PacketDetail{}, false
	}

	return clabinternaltuibackend.PacketDetail{
		Packet:   packet.summary,
		Layers:   dissect(packet.data, true).layers,
		Data:     slices.Clone(packet.data),
		Captured: packet.captured,
	}, true
}
