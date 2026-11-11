// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"encoding/hex"
	"math/rand/v2"
	"net"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

var (
	macA = net.HardwareAddr{0xaa, 0xc1, 0xab, 0x00, 0x00, 0x01}
	macB = net.HardwareAddr{0xaa, 0xc1, 0xab, 0x00, 0x00, 0x02}
	ipA  = net.IPv4(10, 0, 0, 1).To4()
	ipB  = net.IPv4(10, 0, 0, 2).To4()
)

// frame serializes protocol headers into the bytes of a frame.
func frame(t *testing.T, headers ...gopacket.SerializableLayer) []byte {
	t.Helper()

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}

	if err := gopacket.SerializeLayers(buf, opts, headers...); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func ethernet(kind layers.EthernetType) *layers.Ethernet {
	return &layers.Ethernet{SrcMAC: macA, DstMAC: macB, EthernetType: kind}
}

func ipv4(proto layers.IPProtocol) *layers.IPv4 {
	return &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: proto, SrcIP: ipA, DstIP: ipB}
}

// tcpSegment returns a frame with a TCP segment carrying the payload.
func tcpSegment(t *testing.T, src, dst layers.TCPPort, tcp layers.TCP, payload []byte) []byte {
	t.Helper()

	ip := ipv4(layers.IPProtocolTCP)
	tcp.SrcPort, tcp.DstPort = src, dst

	if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatal(err)
	}

	return frame(t, ethernet(layers.EthernetTypeIPv4), ip, &tcp, gopacket.Payload(payload))
}

// udpDatagram returns a frame with a UDP datagram carrying the payload.
func udpDatagram(t *testing.T, src, dst layers.UDPPort, payload []byte) []byte {
	t.Helper()

	ip := ipv4(layers.IPProtocolUDP)
	udp := &layers.UDP{SrcPort: src, DstPort: dst}

	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatal(err)
	}

	return frame(t, ethernet(layers.EthernetTypeIPv4), ip, udp, gopacket.Payload(payload))
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()

	data, err := hex.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// bgpMessageBytes builds a BGP message of the given type.
func bgpMessageBytes(kind byte, body []byte) []byte {
	msg := []byte(strings.Repeat("\xff", 16))
	length := 19 + len(body)
	msg = append(msg, byte(length>>8), byte(length), kind)

	return append(msg, body...)
}

func TestDissect(t *testing.T) {
	macs := [2]string{macA.String(), macB.String()}
	ips := [2]string{"10.0.0.1", "10.0.0.2"}

	// An OPEN of AS 4200000001, which only fits the capability for four byte AS numbers.
	open := bgpMessageBytes(1, unhex(t, "04 5ba0 005a 0a000001 08 02 06 41 04 fa56ea01"))

	tests := map[string]struct {
		data             []byte
		addrs            [2]string
		proto, info      string
		layerNames       string
		field, wantValue string
	}{
		"arp request": {
			data: frame(t, ethernet(layers.EthernetTypeARP), &layers.ARP{
				AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
				HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest,
				SourceHwAddress: macA, SourceProtAddress: ipA,
				DstHwAddress: make([]byte, 6), DstProtAddress: ipB,
			}),
			addrs: macs, proto: "ARP", info: "who has 10.0.0.2? tell 10.0.0.1",
			layerNames: "Ethernet, ARP, Padding", field: "ARP/sender IP", wantValue: "10.0.0.1",
		},
		"arp reply": {
			data: frame(t, ethernet(layers.EthernetTypeARP), &layers.ARP{
				AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
				HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPReply,
				SourceHwAddress: macA, SourceProtAddress: ipA,
				DstHwAddress: macB, DstProtAddress: ipB,
			}),
			addrs: macs, proto: "ARP", info: "10.0.0.1 is at aa:c1:ab:00:00:01",
			layerNames: "Ethernet, ARP, Padding",
		},
		"ping": {
			data: frame(t, ethernet(layers.EthernetTypeIPv4), ipv4(layers.IPProtocolICMPv4),
				&layers.ICMPv4{
					TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
					Id:       7, Seq: 3,
				}, gopacket.Payload("abcdefgh")),
			addrs: ips, proto: "ICMP", info: "echo request id=7 seq=3",
			layerNames: "Ethernet, IPv4, ICMP, Data", field: "IPv4/ttl", wantValue: "64",
		},
		"ping in a vlan": {
			data: frame(t, ethernet(layers.EthernetTypeDot1Q),
				&layers.Dot1Q{VLANIdentifier: 100, Type: layers.EthernetTypeIPv4},
				ipv4(layers.IPProtocolICMPv4),
				&layers.ICMPv4{
					TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoReply, 0),
					Id:       7, Seq: 3,
				}),
			addrs: ips, proto: "ICMP", info: "vlan 100 · echo reply id=7 seq=3",
			layerNames: "Ethernet, 802.1Q VLAN, IPv4, ICMP", field: "802.1Q VLAN/id",
			wantValue: "100",
		},
		"unreachable": {
			data: frame(t, ethernet(layers.EthernetTypeIPv4), ipv4(layers.IPProtocolICMPv4),
				&layers.ICMPv4{TypeCode: layers.CreateICMPv4TypeCode(
					layers.ICMPv4TypeDestinationUnreachable, layers.ICMPv4CodePort)}),
			addrs: ips, proto: "ICMP", info: "destination unreachable (port)",
			layerNames: "Ethernet, IPv4, ICMP",
		},
		"tcp syn": {
			data:  tcpSegment(t, 41794, 179, layers.TCP{SYN: true, Seq: 100, Window: 64240}, nil),
			addrs: ips, proto: "TCP", info: "41794 → 179 [SYN] seq=100 win=64240 len=0",
			layerNames: "Ethernet, IPv4, TCP", field: "TCP/flags", wantValue: "SYN",
		},
		"bgp keepalives and an update": {
			data: tcpSegment(t, 179, 41794, layers.TCP{ACK: true, PSH: true, Seq: 1, Ack: 2},
				slices3(bgpMessageBytes(4, nil), bgpMessageBytes(4, nil),
					bgpMessageBytes(2, []byte{0, 0, 0, 0}))),
			addrs: ips, proto: "BGP", info: "KEEPALIVE ×2, UPDATE",
			layerNames: "Ethernet, IPv4, TCP, BGP",
		},
		"bgp open": {
			data:  tcpSegment(t, 41794, 179, layers.TCP{ACK: true, PSH: true}, open),
			addrs: ips, proto: "BGP", info: "OPEN as=4200000001 hold=90 id=10.0.0.1",
			layerNames: "Ethernet, IPv4, TCP, BGP",
		},
		"bgp notification": {
			data: tcpSegment(t, 41794, 179, layers.TCP{ACK: true, PSH: true},
				bgpMessageBytes(3, []byte{6, 2})),
			addrs: ips, proto: "BGP", info: "NOTIFICATION cease (subcode 2)",
			layerNames: "Ethernet, IPv4, TCP, BGP",
		},
		"ssh data": {
			data: tcpSegment(
				t,
				50000,
				22,
				layers.TCP{ACK: true, PSH: true, Seq: 5, Ack: 6},
				[]byte("x"),
			),
			addrs:      ips,
			proto:      "SSH",
			info:       "50000 → 22 [PSH, ACK] seq=5 ack=6 win=0 len=1",
			layerNames: "Ethernet, IPv4, TCP, Data",
		},
		"udp of no known application": {
			data:  udpDatagram(t, 40000, 40001, []byte("hello")),
			addrs: ips, proto: "UDP", info: "40000 → 40001 len=5",
			layerNames: "Ethernet, IPv4, UDP, Data", field: "UDP/payload", wantValue: "5 bytes",
		},
		"ldp hello": {
			data:  udpDatagram(t, 646, 646, []byte("hello")),
			addrs: ips, proto: "LDP", info: "646 → 646 len=5",
			layerNames: "Ethernet, IPv4, UDP, Data",
		},
		"multihop bfd": {
			data: udpDatagram(t, 49152, 4784,
				unhex(t, "20 c0 03 18 00000001 00000002 000f4240 000f4240 00000000")),
			addrs: ips, proto: "BFD", info: "state=Up my=0x1 your=0x2",
			layerNames: "Ethernet, IPv4, UDP, BFD", field: "BFD/detect multiplier", wantValue: "3",
		},
		"lacp": {
			data: frame(t, ethernet(etherTypeSlowProtocols), gopacket.Payload(unhex(
				t,
				"01 01 01 14 8000 aac1ab000001 0001 8000 0002 3d 000000"+strings.Repeat(
					"00",
					40,
				),
			))),
			addrs: macs, proto: "LACP",
			info: "actor system=aa:c1:ab:00:00:01 key=1 port=2 " +
				"active, aggregatable, in sync, collecting, distributing",
			layerNames: "Ethernet, LACP",
		},
		"isis hello": {
			data: frame(
				t,
				&layers.Ethernet{SrcMAC: macA, DstMAC: macB, EthernetType: layers.EthernetTypeLLC},
				&layers.LLC{DSAP: 0xfe, SSAP: 0xfe, Control: 3},
				gopacket.Payload(
					unhex(t, "83 1b 01 00 10 01 00 00 02 010000000001 001e 05d9 40 01000000000101"),
				),
			),
			addrs:      macs,
			proto:      "IS-IS",
			info:       "L2 LAN hello source=0100.0000.0001 hold=30",
			layerNames: "Ethernet, LLC, IS-IS",
			field:      "IS-IS/holding time",
			wantValue:  "30 s",
		},
		"too short for its headers": {
			data:  frame(t, ethernet(layers.EthernetTypeIPv4), ipv4(layers.IPProtocolTCP))[:16],
			addrs: macs, proto: "Ethernet", info: "type IPv4",
			layerNames: "Ethernet, Not decoded",
		},
		"nothing at all": {
			data: nil, addrs: [2]string{}, proto: "Ethernet", info: "",
			layerNames: "Not decoded",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := dissect(tc.data, true)

			if got.src != tc.addrs[0] || got.dst != tc.addrs[1] {
				t.Errorf(
					"addresses %q → %q, want %q → %q",
					got.src,
					got.dst,
					tc.addrs[0],
					tc.addrs[1],
				)
			}

			if got.proto != tc.proto || got.summary() != tc.info {
				t.Errorf("got %s %q, want %s %q", got.proto, got.summary(), tc.proto, tc.info)
			}

			names := make([]string, len(got.layers))
			for i := range got.layers {
				names[i] = got.layers[i].Name
			}

			if diff := cmp.Diff(tc.layerNames, strings.Join(names, ", ")); diff != "" {
				t.Errorf("unexpected headers (-want +got):\n%s", diff)
			}

			if tc.field != "" {
				if value := fieldValue(got.layers, tc.field); value != tc.wantValue {
					t.Errorf("field %s is %q, want %q", tc.field, value, tc.wantValue)
				}
			}

			// The summary is the same whether or not the headers are asked for.
			brief := dissect(tc.data, false)
			if brief.proto != got.proto || brief.summary() != got.summary() || brief.layers != nil {
				t.Errorf("without details: %s %q, %d headers", brief.proto, brief.summary(),
					len(brief.layers))
			}
		})
	}
}

func slices3(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}

	return out
}

// fieldValue returns the value of a field given as header/name.
func fieldValue(headers []clabinternaltuibackend.PacketLayer, path string) string {
	header, name, _ := strings.Cut(path, "/")

	for i := range headers {
		if headers[i].Name != header {
			continue
		}

		for _, field := range headers[i].Fields {
			if field.Name == name {
				return field.Value
			}
		}
	}

	return ""
}

// TestDissectSurvivesAnything feeds frames that were cut or filled with noise: whatever
// comes off a link is described, never a reason to fail.
func TestDissectSurvivesAnything(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // reproducible noise

	seeds := [][]byte{
		tcpSegment(t, 179, 41794, layers.TCP{ACK: true}, bgpMessageBytes(1, make([]byte, 40))),
		udpDatagram(t, 49152, 3784, make([]byte, 24)),
		udpDatagram(t, 53, 53, make([]byte, 60)),
		udpDatagram(t, 68, 67, make([]byte, 300)),
		udpDatagram(t, 4789, 4789, make([]byte, 80)),
		append(frame(t, ethernet(etherTypeSlowProtocols)), make([]byte, 60)...),
		frame(t, &layers.Ethernet{SrcMAC: macA, DstMAC: macB, EthernetType: layers.EthernetTypeLLC},
			&layers.LLC{DSAP: 0xfe, SSAP: 0xfe, Control: 3}, gopacket.Payload(make([]byte, 40))),
		frame(
			t,
			ethernet(layers.EthernetTypeLinkLayerDiscovery),
			gopacket.Payload(make([]byte, 40)),
		),
		frame(t, ethernet(layers.EthernetTypeMPLSUnicast), gopacket.Payload(make([]byte, 40))),
		frame(t, ethernet(layers.EthernetTypeIPv6), gopacket.Payload(make([]byte, 60))),
	}

	for _, seed := range seeds {
		for cut := 0; cut <= len(seed); cut++ {
			dissect(seed[:cut], true)
		}

		for range 2000 {
			noisy := append([]byte(nil), seed...)

			// The first bytes stay, for the noise to reach the decoders behind them.
			for range 1 + rng.IntN(8) {
				noisy[14+rng.IntN(len(noisy)-14)] = byte(rng.IntN(256))
			}

			if got := dissect(noisy, true); got.proto == "" {
				t.Fatalf("no protocol for %x", noisy)
			}
		}
	}
}

func TestFriendly(t *testing.T) {
	tests := map[string]string{
		"EchoRequest":                  "echo request",
		"DestinationUnreachable(Port)": "destination unreachable (port)",
		"NeighborSolicitation":         "neighbor solicitation",
		"IGMPMembershipQuery":          "igmp membership query",
		"Solicit":                      "solicit",
		"":                             "",
	}

	for name, want := range tests {
		if got := friendly(name); got != want {
			t.Errorf("friendly(%q) = %q, want %q", name, got, want)
		}
	}
}

func BenchmarkDissect(b *testing.B) {
	ip := ipv4(layers.IPProtocolTCP)
	tcp := &layers.TCP{SrcPort: 179, DstPort: 41794, ACK: true, PSH: true}
	_ = tcp.SetNetworkLayerForChecksum(ip)

	buf := gopacket.NewSerializeBuffer()
	_ = gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true},
		ethernet(layers.EthernetTypeIPv4), ip, tcp, gopacket.Payload(bgpMessageBytes(4, nil)))

	data := buf.Bytes()

	for b.Loop() {
		dissect(data, false)
	}
}

func TestDissectUnknownTypes(t *testing.T) {
	got := dissect(frame(t, ethernet(0x9999), gopacket.Payload("0123456789")), true)

	if got.proto != "Ethernet" || got.summary() != "type 0x9999" {
		t.Errorf("got %s %q", got.proto, got.summary())
	}

	if value := fieldValue(got.layers, "Ethernet/type"); value != "0x9999" {
		t.Errorf("the type of the frame is told as %q", value)
	}
}
