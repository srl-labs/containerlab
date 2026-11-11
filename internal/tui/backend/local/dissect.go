// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	protoEthernet = "Ethernet"
	arrow         = " → "
	tagSeparator  = " · "

	etherTypeSlowProtocols layers.EthernetType = 0x8809
	// unknownEtherType is what the decoder calls a type it has no name for.
	unknownEtherType = "UnknownEthernetType"

	// maxDNSNameLen bounds the names of a DNS message in the summary of a packet.
	maxDNSNameLen = 60
)

// Applications told by the port they use, where the payload itself is not looked at. The
// ports are the ones that show up in a network lab.
var (
	tcpApplications = map[uint16]string{
		21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 49: "TACACS+", 53: "DNS", 80: "HTTP",
		179: "BGP", 323: "RPKI-RTR", 389: "LDAP", 443: "TLS", 514: "Syslog", 639: "MSDP",
		646: "LDP", 830: "NETCONF", 4189: "PCEP", 5201: "iPerf3", 6030: "gNMI",
		6514: "Syslog", 6640: "OVSDB", 6653: "OpenFlow", 8080: "HTTP", 8443: "TLS",
		9339: "gNMI", 9340: "gNOI", 32767: "gRPC", 50051: "gRPC", 57400: "gNMI",
	}
	udpApplications = map[uint16]string{
		69: "TFTP", 161: "SNMP", 162: "SNMP", 500: "IKE", 514: "Syslog", 520: "RIP",
		521: "RIPng", 646: "LDP", 862: "TWAMP", 1701: "L2TP", 1985: "HSRP", 2055: "NetFlow",
		3503: "LSP-Ping", 3785: "BFD Echo", 4342: "LISP", 4500: "IPsec", 4739: "IPFIX",
		4790: "VXLAN-GPE", 5201: "iPerf3", 5353: "mDNS", 6635: "MPLS-in-UDP", 8805: "PFCP",
	}
	// bfdPorts are the ports of BFD besides the one of single hop sessions.
	bfdPorts = map[uint16]bool{4784: true, 6784: true}
	// bgpMarker is what every BGP message starts with.
	bgpMarker = bytes.Repeat([]byte{0xff}, 16) //nolint:mnd // the length of the marker
)

// dissection is what is told about a packet: the line that sums it up, and its protocol
// headers when they were asked for.
type dissection struct {
	// detail asks for the fields of the headers.
	detail bool

	src, dst, proto, info string
	// tags name the encapsulations around what the summary describes, ie. a VLAN.
	tags   []string
	layers []clabinternaltuibackend.PacketLayer
	// decoded is set when the data that follows a header was decoded here, which leaves
	// nothing to tell about it as data. padding is set after a header that fills the
	// frame up to its smallest size.
	decoded, padding bool
}

// dissect decodes an Ethernet frame. It never fails: a frame that cannot be decoded is
// described as far as it could be.
func dissect(data []byte, detail bool) (d dissection) {
	d.detail = detail
	d.proto = protoEthernet

	// The frames come off the links of a lab, where anything can be sent. Whatever a
	// decoder trips over must not take the UI down.
	defer func() {
		if r := recover(); r != nil {
			d.info = "malformed packet"
		}
	}()

	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet,
		gopacket.DecodeOptions{NoCopy: true})

	found := packet.Layers()
	failure := packet.ErrorLayer()

	for i, layer := range found {
		// A decoder that gave up on a header still lists it, without its bytes and with
		// nothing in its fields.
		if failure != nil && i+2 == len(found) && i > 0 && len(layer.LayerContents()) == 0 {
			continue
		}

		d.layer(layer)
	}

	if failure != nil && d.detail && !d.decoded {
		d.add("Not decoded", "reason", failure.Error().Error())
	}

	return d
}

// summary returns the line that sums the packet up.
func (d *dissection) summary() string {
	if len(d.tags) == 0 {
		return d.info
	}

	tags := strings.Join(d.tags, " ")
	if d.info == "" {
		return tags
	}

	return tags + tagSeparator + d.info
}

// add records a protocol header with its fields, given as names and values in turn.
func (d *dissection) add(name string, fields ...string) {
	layer := clabinternaltuibackend.PacketLayer{Name: name}

	for i := 0; i+1 < len(fields); i += 2 {
		if fields[i+1] == "" {
			continue
		}

		layer.Fields = append(layer.Fields,
			clabinternaltuibackend.PacketField{Name: fields[i], Value: fields[i+1]})
	}

	d.layers = append(d.layers, layer)
}

// set names the protocol the packet is known by from here on, with what it says.
func (d *dissection) set(proto, info string) {
	d.proto, d.info = proto, info
}

//nolint:cyclop,funlen,gocyclo,maintidx // one case per protocol
func (d *dissection) layer(layer gopacket.Layer) {
	switch l := layer.(type) {
	case *layers.Ethernet:
		d.ethernet(l)
	case *layers.Dot1Q:
		d.tags = append(d.tags, "vlan "+strconv.Itoa(int(l.VLANIdentifier)))
		d.set("VLAN", "type "+l.Type.String())

		if d.detail {
			d.add("802.1Q VLAN", "id", strconv.Itoa(int(l.VLANIdentifier)),
				"priority", strconv.Itoa(int(l.Priority)), "type", l.Type.String())
		}
	case *layers.LLC:
		d.llc(l)
	case *layers.STP:
		d.stp(l)
	case *layers.ARP:
		d.arp(l)
	case *layers.IPv4:
		d.ipv4(l)
	case *layers.IPv6:
		d.ipv6(l)
	case *layers.MPLS:
		d.tags = append(d.tags, "label "+strconv.FormatUint(uint64(l.Label), 10))
		d.set("MPLS", "")

		if d.detail {
			d.add("MPLS", "label", strconv.FormatUint(uint64(l.Label), 10),
				"traffic class", strconv.Itoa(int(l.TrafficClass)),
				"bottom of stack", strconv.FormatBool(l.StackBottom),
				"ttl", strconv.Itoa(int(l.TTL)))
		}
	case *layers.ICMPv4:
		d.icmpv4(l)
	case *layers.ICMPv6:
		d.set("ICMPv6", friendly(l.TypeCode.String()))

		if d.detail {
			d.add("ICMPv6", "type", l.TypeCode.String())
		}
	case *layers.ICMPv6Echo:
		d.info += fmt.Sprintf(" id=%d seq=%d", l.Identifier, l.SeqNumber)

		if d.detail {
			d.add("ICMPv6 echo", "identifier", strconv.Itoa(int(l.Identifier)),
				"sequence", strconv.Itoa(int(l.SeqNumber)))
		}
	case *layers.ICMPv6NeighborSolicitation:
		d.info = "neighbor solicitation: who has " + l.TargetAddress.String() + "?"

		if d.detail {
			d.add("Neighbor solicitation", "target", l.TargetAddress.String())
		}
	case *layers.ICMPv6NeighborAdvertisement:
		d.info = "neighbor advertisement: " + l.TargetAddress.String() + " is here"

		if d.detail {
			d.add("Neighbor advertisement", "target", l.TargetAddress.String(),
				"router", strconv.FormatBool(l.Router()),
				"solicited", strconv.FormatBool(l.Solicited()),
				"override", strconv.FormatBool(l.Override()))
		}
	case *layers.ICMPv6RouterAdvertisement:
		if d.detail {
			d.add("Router advertisement", "hop limit", strconv.Itoa(int(l.HopLimit)),
				"router lifetime", fmt.Sprintf("%d s", l.RouterLifetime))
		}
	case *layers.IGMP:
		d.set("IGMP", friendly(l.Type.String())+" "+l.GroupAddress.String())

		if d.detail {
			d.add("IGMP", "type", l.Type.String(), "group", l.GroupAddress.String())
		}
	case *layers.IGMPv1or2:
		d.set("IGMP", friendly(l.Type.String())+" "+l.GroupAddress.String())

		if d.detail {
			d.add("IGMP", "type", l.Type.String(), "group", l.GroupAddress.String())
		}
	case *layers.TCP:
		d.tcp(l)
	case *layers.UDP:
		d.udp(l)
	case *layers.DNS:
		d.dns(l)
	case *layers.DHCPv4:
		d.dhcpv4(l)
	case *layers.DHCPv6:
		d.set("DHCPv6", friendly(l.MsgType.String()))

		if d.detail {
			d.add("DHCPv6", "message", l.MsgType.String())
		}
	case *layers.NTP:
		d.set("NTP", fmt.Sprintf("version %d, mode %d, stratum %d", l.Version, l.Mode, l.Stratum))

		if d.detail {
			d.add("NTP", "version", strconv.Itoa(int(l.Version)),
				"mode", strconv.Itoa(int(l.Mode)), "stratum", strconv.Itoa(int(l.Stratum)))
		}
	case *layers.BFD:
		d.bfd(l)
	case *layers.VXLAN:
		d.tags = append(d.tags, "vni "+strconv.FormatUint(uint64(l.VNI), 10))
		d.set("VXLAN", "")

		if d.detail {
			d.add("VXLAN", "vni", strconv.FormatUint(uint64(l.VNI), 10))
		}
	case *layers.Geneve:
		d.tags = append(d.tags, "vni "+strconv.FormatUint(uint64(l.VNI), 10))
		d.set("Geneve", "")

		if d.detail {
			d.add("Geneve", "vni", strconv.FormatUint(uint64(l.VNI), 10),
				"protocol", l.Protocol.String())
		}
	case *layers.GRE:
		d.set("GRE", "protocol "+l.Protocol.String())

		if d.detail {
			d.add("GRE", "protocol", l.Protocol.String())
		}
	case *layers.OSPFv2:
		d.ospf("OSPF", &l.OSPF)
	case *layers.OSPFv3:
		d.ospf("OSPFv3", &l.OSPF)
	case *layers.VRRPv2:
		d.vrrp(l)
	case *layers.LinkLayerDiscovery:
		d.lldp(l)
	case *layers.LinkLayerDiscoveryInfo:
		d.lldpInfo(l)
	case *gopacket.Payload:
		name := "Data"
		if d.padding {
			name = "Padding"
		}

		if d.detail && !d.decoded {
			d.add(name, "length", fmt.Sprintf("%d bytes", len(l.Payload())))
		}
	case *gopacket.DecodeFailure, *gopacket.Fragment:
		// Told as the reason the decoding stopped.
	default:
		name := layer.LayerType().String()
		d.set(name, "")

		if d.detail {
			d.add(name, "length", fmt.Sprintf("%d bytes", len(layer.LayerContents())))
		}
	}
}

func (d *dissection) ethernet(l *layers.Ethernet) {
	d.src, d.dst = l.SrcMAC.String(), l.DstMAC.String()

	kind := l.EthernetType.String()
	if kind == unknownEtherType {
		kind = fmt.Sprintf("%#04x", uint16(l.EthernetType))
	}

	d.set(protoEthernet, "type "+kind)

	if l.EthernetType == layers.EthernetTypeLLC {
		kind = fmt.Sprintf("802.3, %d bytes", l.Length)
		d.info = "802.3 frame"
	}

	if d.detail {
		d.add(protoEthernet, "source", d.src, "destination", d.dst, "type", kind)
	}

	if l.EthernetType == etherTypeSlowProtocols {
		d.slowProtocol(l.Payload)
	}
}

// llc handles the frames that carry a length in place of a type: the ones of the
// spanning tree, which has a decoder of its own, and of IS-IS.
func (d *dissection) llc(l *layers.LLC) {
	d.set("LLC", fmt.Sprintf("dsap %#02x ssap %#02x", l.DSAP, l.SSAP))

	if d.detail {
		d.add("LLC", "dsap", fmt.Sprintf("%#02x", l.DSAP), "ssap", fmt.Sprintf("%#02x", l.SSAP))
	}

	const (
		sapOSI = 0xfe
		sapSTP = 0x42
	)

	switch l.DSAP {
	case sapOSI:
		d.isis(l.Payload)
	case sapSTP:
		d.set("STP", "")
	}
}

func (d *dissection) stp(l *layers.STP) {
	const (
		bpduTCN  = 0x80
		rapidSTP = 2
	)

	name := "STP"
	if l.Version >= rapidSTP {
		name = "RSTP"
	}

	root := fmt.Sprintf("%d/%s", l.RouteID.Priority, l.RouteID.HwAddr)
	bridge := fmt.Sprintf("%d/%s", l.BridgeID.Priority, l.BridgeID.HwAddr)

	info := fmt.Sprintf("root=%s cost=%d bridge=%s port=%#x", root, l.Cost, bridge, l.PortID)
	if l.Type == bpduTCN {
		info = "topology change notification"
	} else if l.TC {
		info += " topology change"
	}

	d.set(name, info)

	if d.detail {
		d.add("Spanning tree", "version", strconv.Itoa(int(l.Version)), "root", root,
			"root path cost", strconv.FormatUint(uint64(l.Cost), 10), "bridge", bridge,
			"port", fmt.Sprintf("%#x", l.PortID),
			"topology change", strconv.FormatBool(l.TC))
	}
}

func (d *dissection) arp(l *layers.ARP) {
	senderIP, targetIP := net.IP(l.SourceProtAddress).String(), net.IP(l.DstProtAddress).String()
	senderMAC := net.HardwareAddr(l.SourceHwAddress).String()

	var info, operation string

	switch l.Operation {
	case layers.ARPRequest:
		operation = "request"
		info = "who has " + targetIP + "? tell " + senderIP

		if senderIP == targetIP {
			info = "gratuitous ARP for " + senderIP
		}
	case layers.ARPReply:
		operation = "reply"
		info = senderIP + " is at " + senderMAC
	default:
		operation = strconv.Itoa(int(l.Operation))
		info = "operation " + operation
	}

	d.set("ARP", info)
	d.padding = true

	if d.detail {
		d.add("ARP", "operation", operation, "sender MAC", senderMAC, "sender IP", senderIP,
			"target MAC", net.HardwareAddr(l.DstHwAddress).String(), "target IP", targetIP)
	}
}

func (d *dissection) ipv4(l *layers.IPv4) {
	d.src, d.dst = l.SrcIP.String(), l.DstIP.String()
	d.set("IPv4", "protocol "+l.Protocol.String())

	fragment := ""

	if l.Flags&layers.IPv4MoreFragments != 0 || l.FragOffset != 0 {
		// The offset counts units of eight bytes.
		fragment = fmt.Sprintf("offset %d", int(l.FragOffset)*8) //nolint:mnd // see above
		d.info = fmt.Sprintf("fragment of %s, id %#04x, %s", l.Protocol, l.Id, fragment)
	}

	if d.detail {
		d.add("IPv4", "source", d.src, "destination", d.dst,
			"protocol", l.Protocol.String(), "ttl", strconv.Itoa(int(l.TTL)),
			"length", strconv.Itoa(int(l.Length)), "id", fmt.Sprintf("%#04x", l.Id),
			"dscp", strconv.Itoa(int(l.TOS>>2)), //nolint:mnd // the upper six bits
			"flags", l.Flags.String(), "fragment", fragment)
	}

	const protoPIM = 103

	if l.Protocol == protoPIM && fragment == "" {
		d.pim(l.Payload)
	}
}

func (d *dissection) ipv6(l *layers.IPv6) {
	d.src, d.dst = l.SrcIP.String(), l.DstIP.String()
	d.set("IPv6", "next header "+l.NextHeader.String())

	if d.detail {
		d.add("IPv6", "source", d.src, "destination", d.dst,
			"next header", l.NextHeader.String(), "hop limit", strconv.Itoa(int(l.HopLimit)),
			"payload length", strconv.Itoa(int(l.Length)),
			"traffic class", strconv.Itoa(int(l.TrafficClass)),
			"flow label", fmt.Sprintf("%#05x", l.FlowLabel))
	}
}

func (d *dissection) icmpv4(l *layers.ICMPv4) {
	var info string

	switch l.TypeCode.Type() {
	case layers.ICMPv4TypeEchoRequest:
		info = fmt.Sprintf("echo request id=%d seq=%d", l.Id, l.Seq)
	case layers.ICMPv4TypeEchoReply:
		info = fmt.Sprintf("echo reply id=%d seq=%d", l.Id, l.Seq)
	default:
		info = friendly(l.TypeCode.String())
	}

	d.set("ICMP", info)

	if d.detail {
		d.add("ICMP", "type", l.TypeCode.String(), "identifier", strconv.Itoa(int(l.Id)),
			"sequence", strconv.Itoa(int(l.Seq)))
	}
}

func (d *dissection) tcp(l *layers.TCP) {
	var flags []string

	for _, flag := range []struct {
		set  bool
		name string
	}{
		{l.SYN, "SYN"},
		{l.FIN, "FIN"},
		{l.RST, "RST"},
		{l.PSH, "PSH"},
		{l.ACK, "ACK"},
		{l.URG, "URG"},
	} {
		if flag.set {
			flags = append(flags, flag.name)
		}
	}

	info := fmt.Sprintf("%d%s%d [%s] seq=%d", l.SrcPort, arrow, l.DstPort,
		strings.Join(flags, ", "), l.Seq)
	if l.ACK {
		info += fmt.Sprintf(" ack=%d", l.Ack)
	}

	info += fmt.Sprintf(" win=%d len=%d", l.Window, len(l.Payload))
	d.set("TCP", info)

	if d.detail {
		d.add("TCP", "source port", strconv.Itoa(int(l.SrcPort)),
			"destination port", strconv.Itoa(int(l.DstPort)),
			"flags", strings.Join(flags, ", "),
			"sequence", strconv.FormatUint(uint64(l.Seq), 10),
			"acknowledgment", strconv.FormatUint(uint64(l.Ack), 10),
			"window", strconv.Itoa(int(l.Window)),
			"payload", fmt.Sprintf("%d bytes", len(l.Payload)))
	}

	// A segment without data belongs to the connection, not to what the connection carries.
	if len(l.Payload) == 0 {
		return
	}

	app, known := tcpApplications[uint16(l.DstPort)]
	if !known {
		app, known = tcpApplications[uint16(l.SrcPort)]
	}

	if !known {
		return
	}

	d.proto = app

	if app == "BGP" {
		d.bgp(l.Payload)
	}
}

func (d *dissection) udp(l *layers.UDP) {
	d.set("UDP", fmt.Sprintf("%d%s%d len=%d", l.SrcPort, arrow, l.DstPort, len(l.Payload)))

	if d.detail {
		d.add("UDP", "source port", strconv.Itoa(int(l.SrcPort)),
			"destination port", strconv.Itoa(int(l.DstPort)),
			"payload", fmt.Sprintf("%d bytes", len(l.Payload)))
	}

	src, dst := uint16(l.SrcPort), uint16(l.DstPort)

	if bfdPorts[dst] {
		var bfd layers.BFD

		if bfd.DecodeFromBytes(l.Payload, gopacket.NilDecodeFeedback) == nil {
			d.bfd(&bfd)
			d.decoded = true
		}

		return
	}

	app, known := udpApplications[dst]
	if !known {
		app, known = udpApplications[src]
	}

	if known {
		d.proto = app
	}
}

func (d *dissection) dns(l *layers.DNS) {
	var info strings.Builder

	if l.QR {
		info.WriteString("response")
	} else {
		info.WriteString("query")
	}

	for i := range l.Questions {
		q := &l.Questions[i]
		info.WriteString(" " + q.Type.String() + " " + shorten(string(q.Name), maxDNSNameLen))
	}

	if l.QR {
		switch {
		case l.ResponseCode != layers.DNSResponseCodeNoErr:
			info.WriteString(arrow + friendly(l.ResponseCode.String()))
		case len(l.Answers) > 0:
			answers := make([]string, 0, len(l.Answers))

			for i := range l.Answers {
				answers = append(answers, dnsAnswer(&l.Answers[i]))
			}

			info.WriteString(arrow + strings.Join(answers, ", "))
		}
	}

	d.set("DNS", info.String())

	if d.detail {
		d.add("DNS", "id", fmt.Sprintf("%#04x", l.ID), "response", strconv.FormatBool(l.QR),
			"questions", strconv.Itoa(len(l.Questions)),
			"answers", strconv.Itoa(len(l.Answers)), "result", l.ResponseCode.String())
	}
}

func dnsAnswer(a *layers.DNSResourceRecord) string {
	switch {
	case a.IP != nil:
		return a.IP.String()
	case len(a.CNAME) > 0:
		return shorten(string(a.CNAME), maxDNSNameLen)
	case len(a.PTR) > 0:
		return shorten(string(a.PTR), maxDNSNameLen)
	default:
		return a.Type.String()
	}
}

func (d *dissection) dhcpv4(l *layers.DHCPv4) {
	message := l.Operation.String()

	for _, option := range l.Options {
		if option.Type == layers.DHCPOptMessageType && len(option.Data) == 1 {
			message = layers.DHCPMsgType(option.Data[0]).String()
		}
	}

	info := fmt.Sprintf("%s xid=%#x", message, l.Xid)
	if !l.YourClientIP.IsUnspecified() {
		info += " address=" + l.YourClientIP.String()
	}

	d.set("DHCP", info)

	if d.detail {
		d.add("DHCP", "message", message, "transaction", fmt.Sprintf("%#x", l.Xid),
			"client MAC", l.ClientHWAddr.String(), "client IP", l.ClientIP.String(),
			"offered IP", l.YourClientIP.String())
	}
}

func (d *dissection) bfd(l *layers.BFD) {
	info := fmt.Sprintf("state=%s my=%#x your=%#x", l.State, uint32(l.MyDiscriminator),
		uint32(l.YourDiscriminator))

	switch {
	case l.Poll:
		info += " poll"
	case l.Final:
		info += " final"
	}

	d.set("BFD", info)

	if d.detail {
		d.add("BFD", "state", l.State.String(), "diagnostic", l.Diagnostic.String(),
			"my discriminator", fmt.Sprintf("%#x", uint32(l.MyDiscriminator)),
			"your discriminator", fmt.Sprintf("%#x", uint32(l.YourDiscriminator)),
			"detect multiplier", strconv.Itoa(int(l.DetectMultiplier)),
			"desired min tx", fmt.Sprintf("%d µs", uint32(l.DesiredMinTxInterval)),
			"required min rx", fmt.Sprintf("%d µs", uint32(l.RequiredMinRxInterval)))
	}
}

func (d *dissection) ospf(name string, l *layers.OSPF) {
	router, area := dotted(l.RouterID), dotted(l.AreaID)

	kind := l.Type.String()
	if kind == "" {
		kind = "type " + strconv.Itoa(int(l.Type))
	}

	d.set(name, fmt.Sprintf("%s router=%s area=%s", kind, router, area))

	if d.detail {
		d.add(name, "type", kind, "router", router, "area", area,
			"length", strconv.Itoa(int(l.PacketLength)))
	}
}

func (d *dissection) vrrp(l *layers.VRRPv2) {
	addresses := make([]string, 0, len(l.IPAddress))
	for _, ip := range l.IPAddress {
		addresses = append(addresses, ip.String())
	}

	d.set("VRRP", fmt.Sprintf("advertisement vrid=%d priority=%d %s", l.VirtualRtrID,
		l.Priority, strings.Join(addresses, ",")))

	if d.detail {
		d.add("VRRP", "version", strconv.Itoa(int(l.Version)),
			"virtual router", strconv.Itoa(int(l.VirtualRtrID)),
			"priority", strconv.Itoa(int(l.Priority)),
			"interval", fmt.Sprintf("%d s", l.AdverInt),
			"addresses", strings.Join(addresses, ", "))
	}
}

func (d *dissection) lldp(l *layers.LinkLayerDiscovery) {
	chassis := lldpID(l.ChassisID.ID, l.ChassisID.Subtype == layers.LLDPChassisIDSubTypeMACAddr)
	port := lldpID(l.PortID.ID, l.PortID.Subtype == layers.LLDPPortIDSubtypeMACAddr)

	d.set("LLDP", fmt.Sprintf("chassis=%s port=%s ttl=%d", chassis, port, l.TTL))

	if d.detail {
		d.add("LLDP", "chassis", chassis, "port", port, "ttl", fmt.Sprintf("%d s", l.TTL))
	}
}

func (d *dissection) lldpInfo(l *layers.LinkLayerDiscoveryInfo) {
	if l.SysName != "" {
		d.info = "system=" + printable(l.SysName) + " " + d.info
	}

	if d.detail {
		d.add("LLDP details", "system name", printable(l.SysName),
			"system description", printable(l.SysDescription),
			"port description", printable(l.PortDescription))
	}
}

// lldpID renders an identifier of LLDP, which is an address or a name.
func lldpID(id []byte, mac bool) string {
	if mac {
		return net.HardwareAddr(id).String()
	}

	return printable(string(id))
}

// slowProtocol handles the frames of the slow protocols, of which LACP is the one to see
// in a lab.
func (d *dissection) slowProtocol(p []byte) {
	const (
		subtypeLACP   = 1
		subtypeMarker = 2
		subtypeOAM    = 3
		// actorEnd is where the information about the actor ends.
		actorEnd = 19
	)

	if len(p) == 0 {
		return
	}

	switch p[0] {
	case subtypeMarker:
		d.set("LACP", "marker")
	case subtypeOAM:
		d.set("OAM", "")
	case subtypeLACP:
		d.set("LACP", "")

		if len(p) < actorEnd {
			return
		}

		system := net.HardwareAddr(p[6:12]).String()
		key := binary.BigEndian.Uint16(p[12:14])
		port := binary.BigEndian.Uint16(p[16:18])
		state := lacpState(p[18])

		d.info = fmt.Sprintf("actor system=%s key=%d port=%d %s", system, key, port, state)
		d.decoded = true

		if d.detail {
			d.add("LACP", "actor system", system,
				"actor system priority", strconv.Itoa(int(binary.BigEndian.Uint16(p[4:6]))),
				"actor key", strconv.Itoa(int(key)), "actor port", strconv.Itoa(int(port)),
				"actor state", state)
		}
	}
}

func lacpState(state byte) string {
	names := []string{
		"active", "short timeout", "aggregatable", "in sync",
		"collecting", "distributing", "defaulted", "expired",
	}

	var set []string

	for bit, name := range names {
		if state&(1<<bit) != 0 {
			set = append(set, name)
		}
	}

	return strings.Join(set, ", ")
}

// isis handles a PDU of IS-IS.
func (d *dissection) isis(p []byte) {
	const (
		nlpidISIS = 0x83
		// typeMask selects the type of a PDU.
		typeMask = 0x1f
		// helloEnd and lspEnd are where the fields used here end in the PDUs.
		helloEnd = 17
		lspEnd   = 24
	)

	// The header all PDUs share is eight bytes long.
	if len(p) < 8 || p[0] != nlpidISIS {
		return
	}

	names := map[byte]string{
		15: "L1 LAN hello", 16: "L2 LAN hello", 17: "point-to-point hello",
		18: "L1 LSP", 20: "L2 LSP", 24: "L1 CSNP", 25: "L2 CSNP", 26: "L1 PSNP", 27: "L2 PSNP",
	}

	kind := p[4] & typeMask

	name, known := names[kind]
	if !known {
		name = "PDU type " + strconv.Itoa(int(kind))
	}

	info := name
	fields := []string{"type", name}

	switch {
	case kind <= 17 && len(p) >= helloEnd:
		source := systemID(p[9:15])
		hold := binary.BigEndian.Uint16(p[15:17])
		info += fmt.Sprintf(" source=%s hold=%d", source, hold)
		fields = append(fields, "source", source, "holding time", fmt.Sprintf("%d s", hold))
	case (kind == 18 || kind == 20) && len(p) >= lspEnd:
		id := fmt.Sprintf("%s.%02x-%02x", systemID(p[12:18]), p[18], p[19])
		seq := binary.BigEndian.Uint32(p[20:24])
		lifetime := binary.BigEndian.Uint16(p[10:12])
		info += fmt.Sprintf(" id=%s seq=%#x lifetime=%d", id, seq, lifetime)
		fields = append(fields, "LSP", id, "sequence", fmt.Sprintf("%#x", seq),
			"remaining lifetime", fmt.Sprintf("%d s", lifetime))
	}

	d.set("IS-IS", info)
	d.decoded = true

	if d.detail {
		d.add("IS-IS", fields...)
	}
}

// systemID renders a system ID of IS-IS the usual way, ie. 0100.0000.0001.
func systemID(id []byte) string {
	return fmt.Sprintf("%02x%02x.%02x%02x.%02x%02x", id[0], id[1], id[2], id[3], id[4], id[5])
}

// pim handles a message of PIM.
func (d *dissection) pim(p []byte) {
	if len(p) == 0 {
		return
	}

	names := map[byte]string{
		0: "hello", 1: "register", 2: "register stop", 3: "join/prune", 4: "bootstrap",
		5: "assert", 8: "candidate RP advertisement",
	}

	const typeMask = 0x0f

	name, known := names[p[0]&typeMask]
	if !known {
		name = "type " + strconv.Itoa(int(p[0]&typeMask))
	}

	d.set("PIM", name)
	d.decoded = true

	if d.detail {
		d.add("PIM", "type", name)
	}
}

// bgp handles the BGP messages a segment starts with. Messages that continue from an
// earlier segment are not recognized, as the connection is not followed.
func (d *dissection) bgp(p []byte) {
	const (
		headerLen = 19
		markerLen = 16
	)

	var messages []string

	for len(p) >= headerLen {
		if !bytes.Equal(p[:markerLen], bgpMarker) {
			break
		}

		length := int(binary.BigEndian.Uint16(p[16:18]))
		if length < headerLen {
			break
		}

		body := p[headerLen:min(length, len(p))]
		messages = append(messages, bgpMessage(p[18], body))

		if length >= len(p) {
			break
		}

		p = p[length:]
	}

	if len(messages) == 0 {
		return
	}

	d.info = countRuns(messages)
	d.decoded = true

	if d.detail {
		d.add("BGP", "messages", strings.Join(messages, "; "))
	}
}

// bgpMessage describes a BGP message of the given type.
func bgpMessage(kind byte, body []byte) string {
	const (
		kindOpen         = 1
		kindUpdate       = 2
		kindNotification = 3
		kindKeepalive    = 4
		kindRouteRefresh = 5
		// openLen is the size of an OPEN message up to its optional parameters.
		openLen = 10
	)

	switch kind {
	case kindOpen:
		if len(body) < openLen {
			return "OPEN"
		}

		return fmt.Sprintf("OPEN as=%d hold=%d id=%s", bgpAS(body),
			binary.BigEndian.Uint16(body[3:5]), net.IP(body[5:9]))
	case kindUpdate:
		return "UPDATE"
	case kindNotification:
		if len(body) < 2 { //nolint:mnd // the code and the subcode
			return "NOTIFICATION"
		}

		codes := map[byte]string{
			1: "message header error", 2: "OPEN message error", 3: "UPDATE message error",
			4: "hold timer expired", 5: "state machine error", 6: "cease",
		}

		code, known := codes[body[0]]
		if !known {
			code = "code " + strconv.Itoa(int(body[0]))
		}

		return fmt.Sprintf("NOTIFICATION %s (subcode %d)", code, body[1])
	case kindKeepalive:
		return "KEEPALIVE"
	case kindRouteRefresh:
		return "ROUTE-REFRESH"
	default:
		return "message type " + strconv.Itoa(int(kind))
	}
}

// bgpAS returns the AS number of an OPEN message: the one of the capability for four
// byte AS numbers when it is there, as the field of the message is too small for those.
func bgpAS(open []byte) uint32 {
	const (
		paramCapabilities = 2
		capFourByteAS     = 65
		asLen             = 4
	)

	as := uint32(binary.BigEndian.Uint16(open[1:3]))
	params := open[10:]

	if int(open[9]) < len(params) {
		params = params[:open[9]]
	}

	// A parameter starts with its type and its length, and so does a capability.
	for len(params) >= 2 {
		kind, size := params[0], int(params[1])
		if size > len(params)-2 {
			break
		}

		value := params[2 : 2+size]
		params = params[2+size:]

		if kind != paramCapabilities {
			continue
		}

		for len(value) >= 2 {
			code, n := value[0], int(value[1])
			if n > len(value)-2 {
				break
			}

			if code == capFourByteAS && n == asLen {
				return binary.BigEndian.Uint32(value[2:6])
			}

			value = value[2+n:]
		}
	}

	return as
}

// countRuns joins items, telling a run of the same item once with how often it occurs.
func countRuns(items []string) string {
	var out []string

	for i := 0; i < len(items); {
		n := 1
		for i+n < len(items) && items[i+n] == items[i] {
			n++
		}

		if n > 1 {
			out = append(out, fmt.Sprintf("%s ×%d", items[i], n))
		} else {
			out = append(out, items[i])
		}

		i += n
	}

	return strings.Join(out, ", ")
}

// friendly turns the name of a value as the decoder spells it, ie. "EchoRequest" or
// "DestinationUnreachable(Port)", into words.
func friendly(name string) string {
	var b strings.Builder

	for i, r := range name {
		upper := r >= 'A' && r <= 'Z'

		if upper && i > 0 {
			prev := name[i-1]
			nextLower := i+1 < len(name) && name[i+1] >= 'a' && name[i+1] <= 'z'

			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z' && nextLower) {
				b.WriteByte(' ')
			}
		}

		if r == '(' && i > 0 {
			b.WriteByte(' ')
		}

		if upper {
			r += 'a' - 'A'
		}

		b.WriteRune(r)
	}

	return b.String()
}

// dotted renders a 32 bit identifier the way an IPv4 address is written.
func dotted(id uint32) string {
	return net.IPv4(byte(id>>24), byte(id>>16), byte(id>>8), byte(id)).String() //nolint:mnd
}

// printable replaces what cannot be shown in a line of text.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f || r == '�' {
			return '.'
		}

		return r
	}, strings.TrimRight(s, "\x00"))
}

// shorten cuts a text that is longer than limit bytes.
func shorten(s string, limit int) string {
	s = printable(s)
	if len(s) <= limit {
		return s
	}

	return s[:limit] + "…"
}
