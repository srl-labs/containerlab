// Package capture writes the pcapng streams of `containerlab capture`: one section with one
// interface per capture target, packets merged across targets in timestamp order. The local
// capturer reads lab link interfaces of Docker and Podman nodes through packet sockets; the c9s
// lab runtime feeds the same writer from clabwire streams.
package capture

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

// pcapng block layout (https://www.ietf.org/archive/id/draft-ietf-opsawg-pcapng-03.html). clabwire
// writes little-endian sections, and so does the merged stream containerlab writes.
const (
	pcapngSectionHeaderType  = 0x0a0d0d0a
	pcapngInterfaceType      = 0x00000001
	pcapngEnhancedPacketType = 0x00000006
	pcapngByteOrderMagic     = 0x1a2b3c4d
	pcapngOptionEnd          = 0
	pcapngOptionUserAppl     = 4
	pcapngOptionIfName       = 2
	pcapngOptionIfTSResol    = 9
	pcapngLinkTypeEthernet   = 1
	// pcapngNanoseconds is the if_tsresol value for 10^-9 s resolution.
	pcapngNanoseconds = 9
	// pcapngMaxBlockLength bounds a block read from a capture stream: a packet of the largest
	// clabwire snap length (1 MiB) plus framing.
	pcapngMaxBlockLength = 1<<20 + 1024
	// captureUserApplication names the writer in the merged section header.
	captureUserApplication = "containerlab"
)

var errPCAPNGByteOrder = errors.New("pcapng section is not little-endian")

// InterfaceDescription is an Interface Description Block.
type InterfaceDescription struct {
	LinkType   uint16
	SnapLength uint32
	Name       string
	// TSResol is the raw if_tsresol value; 6 (microseconds) when the block has none.
	TSResol byte
}

// Packet is an Enhanced Packet Block with its timestamp in nanoseconds.
type Packet struct {
	InterfaceID    uint32
	Timestamp      int64
	OriginalLength uint32
	Data           []byte
}

// Reader reads the blocks of a pcapng stream as clabwire writes it: one or more
// little-endian sections, each with its Interface Description Blocks before its packets.
type Reader struct {
	r          *bufio.Reader
	interfaces []InterfaceDescription
	// sections counts the Section Header Blocks read so far.
	sections int
}

// NewReader returns a reader of the pcapng stream r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReaderSize(r, 64<<10)}
}

// Sections returns the number of Section Header Blocks read so far.
func (p *Reader) Sections() int {
	return p.sections
}

// Event is one block of interest: a new interface or a packet.
type Event struct {
	Interface *InterfaceDescription
	Packet    *Packet
}

// Next returns the next interface or packet, skipping other block types. It returns io.EOF at
// the clean end of the stream, and io.ErrUnexpectedEOF for a stream cut inside a block.
func (p *Reader) Next() (Event, error) {
	for {
		var header [8]byte
		if _, err := io.ReadFull(p.r, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return Event{}, io.EOF
			}

			return Event{}, io.ErrUnexpectedEOF
		}

		blockType := binary.LittleEndian.Uint32(header[0:4])
		length := binary.LittleEndian.Uint32(header[4:8])
		if blockType == pcapngSectionHeaderType {
			// the byte-order magic tells a big-endian section apart before its length is
			// trusted: that length reads byte swapped
			magic, err := p.r.Peek(4)
			if err != nil {
				return Event{}, io.ErrUnexpectedEOF
			}
			if binary.BigEndian.Uint32(magic) == pcapngByteOrderMagic {
				return Event{}, errPCAPNGByteOrder
			}
		}
		if length < 12 || length%4 != 0 || length > pcapngMaxBlockLength {
			return Event{}, fmt.Errorf("invalid pcapng block length %d (at most %d)", length,
				pcapngMaxBlockLength)
		}

		body := make([]byte, length-8)
		if _, err := io.ReadFull(p.r, body); err != nil {
			return Event{}, io.ErrUnexpectedEOF
		}
		if binary.LittleEndian.Uint32(body[len(body)-4:]) != length {
			return Event{}, fmt.Errorf("pcapng block lengths do not match")
		}
		body = body[:len(body)-4]

		switch blockType {
		case pcapngSectionHeaderType:
			if len(body) < 16 {
				return Event{}, fmt.Errorf("short pcapng section header")
			}
			if binary.LittleEndian.Uint32(body[0:4]) != pcapngByteOrderMagic {
				return Event{}, errPCAPNGByteOrder
			}
			p.sections++
			p.interfaces = nil
		case pcapngInterfaceType:
			if p.sections == 0 {
				return Event{}, fmt.Errorf("pcapng interface before a section header")
			}
			intf, err := parsePCAPNGInterface(body)
			if err != nil {
				return Event{}, err
			}
			p.interfaces = append(p.interfaces, intf)

			return Event{Interface: &p.interfaces[len(p.interfaces)-1]}, nil
		case pcapngEnhancedPacketType:
			packet, err := p.parsePacket(body)
			if err != nil {
				return Event{}, err
			}

			return Event{Packet: &packet}, nil
		}
	}
}

func parsePCAPNGInterface(body []byte) (InterfaceDescription, error) {
	if len(body) < 8 {
		return InterfaceDescription{}, fmt.Errorf("short pcapng interface description")
	}
	intf := InterfaceDescription{
		LinkType:   binary.LittleEndian.Uint16(body[0:2]),
		SnapLength: binary.LittleEndian.Uint32(body[4:8]),
		TSResol:    6,
	}

	options := body[8:]
	for len(options) >= 4 {
		code := binary.LittleEndian.Uint16(options[0:2])
		size := int(binary.LittleEndian.Uint16(options[2:4]))
		if code == pcapngOptionEnd {
			break
		}
		padded := size + pcapngPadding(size)
		if 4+padded > len(options) {
			return InterfaceDescription{}, fmt.Errorf("truncated pcapng interface option")
		}
		value := options[4 : 4+size]
		switch code {
		case pcapngOptionIfName:
			intf.Name = string(value)
		case pcapngOptionIfTSResol:
			if size == 1 {
				intf.TSResol = value[0]
			}
		}
		options = options[4+padded:]
	}

	return intf, nil
}

func (p *Reader) parsePacket(body []byte) (Packet, error) {
	if len(body) < 20 {
		return Packet{}, fmt.Errorf("short pcapng enhanced packet")
	}
	packet := Packet{
		InterfaceID:    binary.LittleEndian.Uint32(body[0:4]),
		OriginalLength: binary.LittleEndian.Uint32(body[16:20]),
	}
	if int(packet.InterfaceID) >= len(p.interfaces) {
		return Packet{}, fmt.Errorf("pcapng packet for undeclared interface %d",
			packet.InterfaceID)
	}
	captured := binary.LittleEndian.Uint32(body[12:16])
	if int(captured) > len(body)-20 {
		return Packet{}, fmt.Errorf("pcapng packet data exceeds its block")
	}
	packet.Data = body[20 : 20+captured]

	raw := uint64(binary.LittleEndian.Uint32(body[4:8]))<<32 |
		uint64(binary.LittleEndian.Uint32(body[8:12]))
	timestamp, err := pcapngNanosecondsFrom(raw, p.interfaces[packet.InterfaceID].TSResol)
	if err != nil {
		return Packet{}, err
	}
	packet.Timestamp = timestamp

	return packet, nil
}

// pcapngNanosecondsFrom converts a timestamp in an interface's resolution to nanoseconds.
// Only decimal resolutions up to nanoseconds occur in clabwire streams.
func pcapngNanosecondsFrom(raw uint64, tsresol byte) (int64, error) {
	if tsresol&0x80 != 0 || tsresol > pcapngNanoseconds {
		return 0, fmt.Errorf("unsupported pcapng timestamp resolution %#x", tsresol)
	}
	scale := uint64(1)
	for range pcapngNanoseconds - int(tsresol) {
		scale *= 10
	}
	if raw > math.MaxInt64/scale {
		return 0, fmt.Errorf("pcapng timestamp out of range")
	}

	return int64(raw * scale), nil //nolint:gosec // bounded above
}

// Writer writes one little-endian section with nanosecond timestamps. Every block goes out
// in one Write under a lock, so concurrent writers never interleave blocks and a stream cut
// between writes is still a valid capture.
type Writer struct {
	mu  sync.Mutex
	w   io.Writer
	err error
}

// NewWriter returns a writer of one pcapng section to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteHeader writes the Section Header Block and one Interface Description Block per name, in
// order; packets refer to the interfaces by that index.
func (p *Writer) WriteHeader(names []string, snapLength uint32) error {
	section := make([]byte, 16)
	binary.LittleEndian.PutUint32(section[0:], pcapngByteOrderMagic)
	binary.LittleEndian.PutUint16(section[4:], 1)
	binary.LittleEndian.PutUint16(section[6:], 0)
	binary.LittleEndian.PutUint64(section[8:], math.MaxUint64)
	section = appendPCAPNGOption(section, pcapngOptionUserAppl, []byte(captureUserApplication))
	section = appendPCAPNGOption(section, pcapngOptionEnd, nil)

	header := pcapngBlock(pcapngSectionHeaderType, section)
	for _, name := range names {
		description := make([]byte, 8)
		binary.LittleEndian.PutUint16(description[0:], pcapngLinkTypeEthernet)
		binary.LittleEndian.PutUint32(description[4:], snapLength)
		description = appendPCAPNGOption(description, pcapngOptionIfName, []byte(name))
		description = appendPCAPNGOption(description, pcapngOptionIfTSResol,
			[]byte{pcapngNanoseconds})
		description = appendPCAPNGOption(description, pcapngOptionEnd, nil)
		header = append(header, pcapngBlock(pcapngInterfaceType, description)...)
	}

	return p.write(header)
}

// WritePacket writes an Enhanced Packet Block for the interface with the given index.
func (p *Writer) WritePacket(interfaceID uint32, packet *Packet) error {
	timestamp := uint64(packet.Timestamp) //nolint:gosec // capture timestamps are positive
	body := make([]byte, 20, 20+len(packet.Data)+3)
	binary.LittleEndian.PutUint32(body[0:], interfaceID)
	binary.LittleEndian.PutUint32(body[4:], uint32(timestamp>>32))
	binary.LittleEndian.PutUint32(body[8:], uint32(timestamp)) //nolint:gosec // low 32 bits
	binary.LittleEndian.PutUint32(body[12:], uint32(len(packet.Data)))
	binary.LittleEndian.PutUint32(body[16:], packet.OriginalLength)
	body = append(body, packet.Data...)
	body = append(body, make([]byte, pcapngPadding(len(packet.Data)))...)

	return p.write(pcapngBlock(pcapngEnhancedPacketType, body))
}

// Err returns the first write error, or nil.
func (p *Writer) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.err
}

func (p *Writer) write(block []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.err != nil {
		return p.err
	}
	if _, err := p.w.Write(block); err != nil {
		p.err = fmt.Errorf("failed to write the capture: %w", err)
	}

	return p.err
}

func pcapngBlock(blockType uint32, body []byte) []byte {
	total := uint32(12 + len(body)) //nolint:gosec // bounded by the snap length
	block := make([]byte, 8, total)
	binary.LittleEndian.PutUint32(block[0:], blockType)
	binary.LittleEndian.PutUint32(block[4:], total)
	block = append(block, body...)

	return binary.LittleEndian.AppendUint32(block, total)
}

func appendPCAPNGOption(body []byte, code uint16, value []byte) []byte {
	body = binary.LittleEndian.AppendUint16(body, code)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(value))) //nolint:gosec // short
	body = append(body, value...)

	return append(body, make([]byte, pcapngPadding(len(value)))...)
}

func pcapngPadding(length int) int {
	return (4 - length%4) % 4
}
