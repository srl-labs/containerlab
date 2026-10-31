package capture

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestPCAPNGWriterAndReaderRoundTrip(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writer := NewWriter(&out)
	if err := writer.WriteHeader([]string{"r1:e1-1", "r2:eth1"}, 1500); err != nil {
		t.Fatal(err)
	}
	packets := []Packet{
		{
			InterfaceID:    1,
			Timestamp:      1_761_559_200_123_456_789,
			OriginalLength: 98,
			Data:           []byte("abcde"),
		},
		{
			InterfaceID:    0,
			Timestamp:      1_761_559_200_123_456_790,
			OriginalLength: 4,
			Data:           []byte("wxyz"),
		},
	}
	for idx := range packets {
		if err := writer.WritePacket(packets[idx].InterfaceID, &packets[idx]); err != nil {
			t.Fatal(err)
		}
	}

	data := out.Bytes()
	// the section header names the writer
	if !bytes.Contains(data[:64], []byte(captureUserApplication)) {
		t.Fatal("section header has no shb_userappl")
	}
	if len(data)%4 != 0 {
		t.Fatalf("stream length %d is not 32-bit aligned", len(data))
	}

	reader := NewReader(bytes.NewReader(data))
	var names []string
	var read []Packet
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if event.Interface != nil {
			names = append(names, event.Interface.Name)
			if event.Interface.SnapLength != 1500 || event.Interface.LinkType != 1 {
				t.Fatalf("interface = %+v", event.Interface)
			}
		}
		if event.Packet != nil {
			read = append(read, *event.Packet)
		}
	}
	if len(names) != 2 || names[0] != "r1:e1-1" || names[1] != "r2:eth1" {
		t.Fatalf("names = %v", names)
	}
	for idx := range packets {
		if read[idx].InterfaceID != packets[idx].InterfaceID ||
			read[idx].Timestamp != packets[idx].Timestamp ||
			read[idx].OriginalLength != packets[idx].OriginalLength ||
			!bytes.Equal(read[idx].Data, packets[idx].Data) {
			t.Fatalf("packet %d = %+v, want %+v", idx, read[idx], packets[idx])
		}
	}
}

func TestPCAPNGReaderHandlesRepeatedSectionsAndResolutions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	first := NewWriter(&out)
	_ = first.WriteHeader([]string{"eth1"}, 64)
	_ = first.WritePacket(0, &Packet{Timestamp: 5, Data: []byte{1}})

	// a second section whose interface has microsecond timestamps (no if_tsresol)
	section := make([]byte, 16)
	binary.LittleEndian.PutUint32(section[0:], pcapngByteOrderMagic)
	binary.LittleEndian.PutUint16(section[4:], 1)
	out.Write(pcapngBlock(pcapngSectionHeaderType, section))
	description := make([]byte, 8)
	binary.LittleEndian.PutUint16(description[0:], 1)
	description = appendPCAPNGOption(description, pcapngOptionIfName, []byte("eth2"))
	out.Write(pcapngBlock(pcapngInterfaceType, description))
	body := make([]byte, 20)
	binary.LittleEndian.PutUint32(body[8:], 7) // 7 microseconds
	out.Write(pcapngBlock(pcapngEnhancedPacketType, body))

	reader := NewReader(&out)
	var timestamps []int64
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if event.Packet != nil {
			timestamps = append(timestamps, event.Packet.Timestamp)
		}
	}
	if reader.sections != 2 || len(timestamps) != 2 || timestamps[0] != 5 || timestamps[1] != 7000 {
		t.Fatalf("sections %d, timestamps %v", reader.sections, timestamps)
	}
}

func TestPCAPNGReaderRejectsInvalidStreams(t *testing.T) {
	t.Parallel()

	var valid bytes.Buffer
	writer := NewWriter(&valid)
	_ = writer.WriteHeader([]string{"eth1"}, 64)
	_ = writer.WritePacket(0, &Packet{Timestamp: 1, Data: []byte("data")})

	readAll := func(data []byte) error {
		reader := NewReader(bytes.NewReader(data))
		for {
			_, err := reader.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}

	if err := readAll(valid.Bytes()[:valid.Len()-6]); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated stream: %v", err)
	}

	bigEndian := append([]byte(nil), valid.Bytes()...)
	binary.BigEndian.PutUint32(bigEndian[4:], binary.LittleEndian.Uint32(bigEndian[4:]))
	binary.BigEndian.PutUint32(bigEndian[8:], pcapngByteOrderMagic)
	if err := readAll(bigEndian); !errors.Is(err, errPCAPNGByteOrder) {
		t.Fatalf("big-endian section: %v", err)
	}

	packetFirst := pcapngBlock(pcapngEnhancedPacketType, make([]byte, 20))
	if err := readAll(packetFirst); err == nil {
		t.Fatal("packet before any interface accepted")
	}
}
