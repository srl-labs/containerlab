// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fake

import (
	"fmt"
	"slices"
	"strings"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// demoSSH is how the demo host pretends to be reached.
	demoSSH = demoOwner + "@192.0.2.10"

	// packetsPerTick is the number of packets a simulated live capture sees per step.
	packetsPerTick = 4
	// packetSpacing is the time between two of them.
	packetSpacing = 180 * time.Millisecond
	// maxPackets is the number of packets kept of a simulated capture.
	maxPackets = 10000
	// maxPacketData bounds the made-up bytes of a packet.
	maxPacketData = 128

	broadcastMAC = "ff:ff:ff:ff:ff:ff"
	lldpMAC      = "01:80:c2:00:00:0e"
)

// packetTemplate describes a made-up packet between the two ends of a link. The words in
// braces are replaced by the addresses of the ends.
type packetTemplate struct {
	proto string
	// reverse makes the packet travel from the far end to the near one.
	reverse bool
	// l2 addresses the packet by MAC address, dst overrides its destination.
	l2     bool
	dst    string
	length int
	info   string
}

//nolint:mnd // the sizes of the packets are what such packets measure
var packetTemplates = []packetTemplate{
	{
		proto: "LLDP", l2: true, dst: lldpMAC, length: 214,
		info: "system={node} chassis={mac} port={port} ttl=120",
	},
	{proto: "ARP", l2: true, dst: broadcastMAC, length: 42, info: "who has {peer}? tell {ip}"},
	{proto: "ARP", l2: true, reverse: true, length: 42, info: "{ip} is at {mac}"},
	{proto: "ICMP", length: 98, info: "echo request id=7 seq={n}"},
	{proto: "ICMP", reverse: true, length: 98, info: "echo reply id=7 seq={n}"},
	{proto: "TCP", length: 74, info: "41794 → 179 [SYN] seq=2881736152 win=64240 len=0"},
	{proto: "BGP", length: 127, info: "OPEN as=65001 hold=90 id={ip}"},
	{proto: "BGP", reverse: true, length: 85, info: "KEEPALIVE"},
	{proto: "BGP", length: 191, info: "UPDATE ×2"},
	{proto: "BFD", length: 66, info: "state=Up my=0x1 your=0x2"},
	{proto: "OSPF", dst: "224.0.0.5", length: 82, info: "Hello router={ip} area=0.0.0.0"},
	{proto: "BGP", length: 85, info: "KEEPALIVE"},
}

// capturePackets adds the packets a live capture sees in one step. The lock must be held.
func (b *Backend) capturePackets(j *job) {
	if len(j.targets) == 0 {
		return
	}

	for range packetsPerTick {
		n := j.info.Packets
		target := n / len(packetTemplates) % len(j.targets)

		packet := buildPacket(
			&packetTemplates[n%len(packetTemplates)],
			j.targets[target],
			target,
			n,
		)
		packet.Time = j.info.Started.Add(time.Duration(n+1) * packetSpacing)

		j.packets = append(j.packets, packet)
		j.info.Packets++
	}

	if extra := len(j.packets) - maxPackets; extra > 0 {
		j.packets = slices.Delete(j.packets, 0, extra)
	}
}

// buildPacket fills a template in for the n-th packet of a capture, seen on a target.
func buildPacket(
	tpl *packetTemplate, target string, index, n int,
) clabinternaltuibackend.Packet {
	node, port, _ := strings.Cut(target, ":")

	ends := [2]struct{ mac, ip string }{
		{fmt.Sprintf("1a:2b:00:00:%02x:01", index+1), fmt.Sprintf("10.1.%d.1", index+1)},
		{fmt.Sprintf("1a:2b:00:00:%02x:02", index+1), fmt.Sprintf("10.1.%d.2", index+1)},
	}

	from, to := ends[0], ends[1]
	if tpl.reverse {
		from, to = to, from
	}

	packet := clabinternaltuibackend.Packet{
		Number:      n + 1,
		Interface:   target,
		Source:      from.ip,
		Destination: to.ip,
		Protocol:    tpl.proto,
		Length:      tpl.length,
		Info: strings.NewReplacer(
			"{node}", node, "{port}", port, "{mac}", from.mac, "{ip}", from.ip,
			"{peer}", to.ip, "{n}", fmt.Sprint(n/len(packetTemplates)+1),
		).Replace(tpl.info),
	}

	if tpl.l2 {
		packet.Source, packet.Destination = from.mac, to.mac
	}

	if tpl.dst != "" {
		packet.Destination = tpl.dst
	}

	return packet
}

// livePackets returns the packets of a job with the number of the first one that is
// still kept. The lock must be held.
func (b *Backend) livePackets(
	id clabinternaltuibackend.JobID,
) (packets []clabinternaltuibackend.Packet, first int) {
	j := b.job(id)
	if j == nil {
		return nil, 0
	}

	return j.packets, j.info.Packets - len(j.packets)
}

// Packets implements clabinternaltuibackend.Backend.
func (b *Backend) Packets(
	id clabinternaltuibackend.JobID,
	from int,
) []clabinternaltuibackend.Packet {
	b.mu.Lock()
	defer b.mu.Unlock()

	packets, first := b.livePackets(id)

	from = max(from-first, 0)
	if from >= len(packets) {
		return nil
	}

	return slices.Clone(packets[from:])
}

// PacketDetail implements clabinternaltuibackend.Backend.
func (b *Backend) PacketDetail(
	id clabinternaltuibackend.JobID,
	number int,
) (clabinternaltuibackend.PacketDetail, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	packets, first := b.livePackets(id)

	i := number - 1 - first
	if i < 0 || i >= len(packets) {
		return clabinternaltuibackend.PacketDetail{}, false
	}

	packet := packets[i]
	detail := clabinternaltuibackend.PacketDetail{Packet: packet, Captured: packet.Length}

	// The made-up packets have no headers to decode: the layers repeat the summary.
	field := func(name, value string) clabinternaltuibackend.PacketField {
		return clabinternaltuibackend.PacketField{Name: name, Value: value}
	}
	layer := func(name string, fields ...clabinternaltuibackend.PacketField) {
		detail.Layers = append(detail.Layers,
			clabinternaltuibackend.PacketLayer{Name: name, Fields: fields})
	}

	if strings.Contains(packet.Source, ".") {
		layer("Ethernet", field("type", "IPv4"))
		layer("IPv4", field("source", packet.Source),
			field("destination", packet.Destination), field("ttl", "64"))
	} else {
		layer("Ethernet", field("source", packet.Source),
			field("destination", packet.Destination))
	}

	layer(packet.Protocol, field("summary", packet.Info))

	detail.Data = make([]byte, min(packet.Length, maxPacketData))
	for k := range detail.Data {
		detail.Data[k] = byte(k*7 + packet.Number) //nolint:gosec // a pattern, cut on purpose
	}

	return detail, true
}
