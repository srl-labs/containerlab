// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"bytes"
	"encoding"
	"fmt"
	"net"
	"strings"
)

// Constants for use in Match names.
const (
	source      = "src"
	destination = "dst"

	sourceHardwareAddr = "sha"
	targetHardwareAddr = "tha"
	sourceProtocolAddr = "spa"
	targetProtocolAddr = "tpa"
)

// Constants of full Match names.
const (
	arpOp       = "arp_op"
	arpSHA      = "arp_sha"
	arpSPA      = "arp_spa"
	arpTHA      = "arp_tha"
	arpTPA      = "arp_tpa"
	conjID      = "conj_id"
	ctMark      = "ct_mark"
	ctState     = "ct_state"
	ctZone      = "ct_zone"
	dlDST       = "dl_dst"
	dlSRC       = "dl_src"
	dlType      = "dl_type"
	dlVLAN      = "dl_vlan"
	dlVLANPCP   = "dl_vlan_pcp"
	icmp6Code   = "icmpv6_code"
	icmp6Type   = "icmpv6_type"
	icmpCode    = "icmp_code"
	icmpType    = "icmp_type"
	ipFrag      = "ip_frag"
	ipv6DST     = "ipv6_dst"
	ipv6Label   = "ipv6_label"
	ipv6SRC     = "ipv6_src"
	metadata    = "metadata"
	ndSLL       = "nd_sll"
	ndTarget    = "nd_target"
	ndTLL       = "nd_tll"
	nwDST       = "nw_dst"
	nwECN       = "nw_ecn"
	nwProto     = "nw_proto"
	nwSRC       = "nw_src"
	nwTOS       = "nw_tos"
	nwTTL       = "nw_ttl"
	tcpFlags    = "tcp_flags"
	tpDST       = "tp_dst"
	tpSRC       = "tp_src"
	tunDST      = "tun_dst"
	tunFlags    = "tun_flags"
	tunGbpFlags = "tun_gbp_flags"
	tunGbpID    = "tun_gbp_id"
	tunID       = "tun_id"
	tunSRC      = "tun_src"
	tunTOS      = "tun_tos"
	tunTTL      = "tun_ttl"
	tunv6DST    = "tun_ipv6_dst"
	tunv6SRC    = "tun_ipv6_src"
	vlanTCI1    = "vlan_tci1"
	vlanTCI     = "vlan_tci"
)

// A Match is a type which can be marshaled into an OpenFlow packet matching
// statement.  Matches can be used with Flows to match specific packet types
// and fields.
//
// Matches must also implement fmt.GoStringer for code generation purposes.
type Match interface {
	encoding.TextMarshaler
	fmt.GoStringer
}

// DataLinkSource matches packets with a source hardware address and optional
// wildcard mask matching addr.
func DataLinkSource(addr string) Match {
	return &dataLinkMatch{
		srcdst: source,
		addr:   addr,
	}
}

// DataLinkDestination matches packets with a destination hardware address
// and optional wildcard mask matching addr.
func DataLinkDestination(addr string) Match {
	return &dataLinkMatch{
		srcdst: destination,
		addr:   addr,
	}
}

const (
	// ethernetAddrLen is the length in bytes of an ethernet hardware address.
	ethernetAddrLen = 6
)

var _ Match = &dataLinkMatch{}

// A dataLinkMatch is a Match returned by DataLink{Source,Destination}.
type dataLinkMatch struct {
	srcdst string
	addr   string
}

// GoString implements Match.
func (m *dataLinkMatch) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.DataLinkSource(%q)", m.addr)
	}

	return fmt.Sprintf("ovs.DataLinkDestination(%q)", m.addr)
}

// MarshalText implements Match.
func (m *dataLinkMatch) MarshalText() ([]byte, error) {
	// Split the string before possible wildcard mask
	ss := strings.SplitN(m.addr, "/", 2)

	hwAddr, err := net.ParseMAC(ss[0])
	if err != nil {
		return nil, err
	}
	if len(hwAddr) != ethernetAddrLen {
		return nil, fmt.Errorf("hardware address must be %d octets, but got %d",
			ethernetAddrLen, len(hwAddr))
	}

	if len(ss) == 1 {
		// Address has no wildcard mask
		return bprintf("dl_%s=%s", m.srcdst, hwAddr.String()), nil
	}

	wildcard, err := net.ParseMAC(ss[1])
	if err != nil {
		return nil, err
	}
	if len(wildcard) != ethernetAddrLen {
		return nil, fmt.Errorf("wildcard mask must be %d octets, but got %d",
			ethernetAddrLen, len(wildcard))
	}

	return bprintf("dl_%s=%s/%s", m.srcdst, hwAddr.String(), wildcard.String()), nil
}

// DataLinkType matches packets with the specified EtherType.
func DataLinkType(etherType uint16) Match {
	return &dataLinkTypeMatch{
		etherType: etherType,
	}
}

var _ Match = &dataLinkTypeMatch{}

// A dataLinkTypeMatch is a Match returned by DataLinkType.
type dataLinkTypeMatch struct {
	etherType uint16
}

// MarshalText implements Match.
func (m *dataLinkTypeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=0x%04x", dlType, m.etherType), nil
}

// GoString implements Match.
func (m *dataLinkTypeMatch) GoString() string {
	return fmt.Sprintf("ovs.DataLinkType(0x%04x)", m.etherType)
}

// VLANNone is a special value which indicates that DataLinkVLAN should only
// match packets with no VLAN tag specified.
const VLANNone = 0xffff

// DataLinkVLAN matches packets with the specified VLAN ID matching vid.
func DataLinkVLAN(vid int) Match {
	return &dataLinkVLANMatch{
		vid: vid,
	}
}

var _ Match = &dataLinkVLANMatch{}

// A dataLinkVLANMatch is a Match returned by DataLinkVLAN.
type dataLinkVLANMatch struct {
	vid int
}

// MarshalText implements Match.
func (m *dataLinkVLANMatch) MarshalText() ([]byte, error) {
	if !validVLANVID(m.vid) && m.vid != VLANNone {
		return nil, errInvalidVLANVID
	}

	if m.vid == VLANNone {
		return bprintf("%s=0xffff", dlVLAN), nil
	}

	return bprintf("%s=%d", dlVLAN, m.vid), nil
}

// GoString implements Match.
func (m *dataLinkVLANMatch) GoString() string {
	if m.vid == VLANNone {
		return "ovs.DataLinkVLAN(ovs.VLANNone)"
	}

	return fmt.Sprintf("ovs.DataLinkVLAN(%d)", m.vid)
}

// DataLinkVLANPCP matches packets with the specified VLAN PCP matching pcp.
func DataLinkVLANPCP(pcp int) Match {
	return &dataLinkVLANPCPMatch{
		pcp: pcp,
	}
}

var _ Match = &dataLinkVLANPCPMatch{}

// A dataLinkVLANPCPMatch is a Match returned by DataLinkVLANPCP.
type dataLinkVLANPCPMatch struct {
	pcp int
}

// MarshalText implements Match.
func (m *dataLinkVLANPCPMatch) MarshalText() ([]byte, error) {
	if !validVLANPCP(m.pcp) {
		return nil, errInvalidVLANPCP
	}

	return bprintf("%s=%d", dlVLANPCP, m.pcp), nil
}

// GoString implements Match.
func (m *dataLinkVLANPCPMatch) GoString() string {
	return fmt.Sprintf("ovs.DataLinkVLANPCP(%d)", m.pcp)
}

// NetworkSource matches packets with a source IPv4 address or IPv4 CIDR
// block matching ip.
func NetworkSource(ip string) Match {
	return &networkMatch{
		srcdst: source,
		ip:     ip,
	}
}

// NetworkDestination matches packets with a destination IPv4 address or
// IPv4 CIDR block matching ip.
func NetworkDestination(ip string) Match {
	return &networkMatch{
		srcdst: destination,
		ip:     ip,
	}
}

var _ Match = &networkMatch{}

// A networkMatch is a Match returned by Network{Source,Destination}.
type networkMatch struct {
	srcdst string
	ip     string
}

// MarshalText implements Match.
func (m *networkMatch) MarshalText() ([]byte, error) {
	return matchIPv4AddressOrCIDR(fmt.Sprintf("nw_%s", m.srcdst), m.ip)
}

// GoString implements Match.
func (m *networkMatch) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.NetworkSource(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.NetworkDestination(%q)", m.ip)
}

// NetworkECN creates a new networkECN
func NetworkECN(ecn int) Match {
	return &networkECN{
		ecn: ecn,
	}
}

var _ Match = &networkECN{}

// a networkECN is a match for network Explicit Congestion Notification
type networkECN struct {
	ecn int
}

// MarshalText implements Match.
func (e *networkECN) MarshalText() ([]byte, error) {
	return bprintf("nw_ecn=%d", e.ecn), nil
}

// GoString implements Match.
func (e *networkECN) GoString() string {
	return fmt.Sprintf("ovs.NetworkECN(%d)", e.ecn)
}

// NetworkTOS returns a new networkTOS type
func NetworkTOS(tos int) Match {
	return &networkTOS{
		tos: tos,
	}
}

var _ Match = &networkTOS{}

// networkTOS is a match for network type of service
type networkTOS struct {
	tos int
}

// MarshalText implements Match.
func (t *networkTOS) MarshalText() ([]byte, error) {
	return bprintf("nw_tos=%d", t.tos), nil
}

// GoString implements Match.
func (t *networkTOS) GoString() string {
	return fmt.Sprintf("ovs.NetworkTOS(%d)", t.tos)
}

// TunnelGBP returns a new tunnelGBP
func TunnelGBP(gbp int) Match {
	return &tunnelGBP{
		gbp: gbp,
	}
}

var _ Match = &tunnelGBP{}

// tunnelGBP is a match for tunnel GBP
type tunnelGBP struct {
	gbp int
}

// MarshalText implements Match.
func (t *tunnelGBP) MarshalText() ([]byte, error) {
	return bprintf("tun_gbp_id=%d", t.gbp), nil
}

// GoString implements Match.
func (t *tunnelGBP) GoString() string {
	return fmt.Sprintf("ovs.TunnelGBP(%d)", t.gbp)
}

// TunnelGbpFlags returns a new tunnelFlags
func TunnelGbpFlags(gbpFlags int) Match {
	return &tunnelGbpFlags{
		gbpFlags: gbpFlags,
	}
}

var _ Match = &tunnelGbpFlags{}

// tunnelGbpFlags is a match for tunnel Flags
type tunnelGbpFlags struct {
	gbpFlags int
}

// MarshalText implements Match.
func (t *tunnelGbpFlags) MarshalText() ([]byte, error) {
	return bprintf("tun_gbp_flags=%d", t.gbpFlags), nil
}

// GoString implements Match.
func (t *tunnelGbpFlags) GoString() string {
	return fmt.Sprintf("ovs.TunnelGbpFlags(%d)", t.gbpFlags)
}

// TunnelFlags returns a new tunnelFlags
func TunnelFlags(flags int) Match {
	return &tunnelFlags{
		flags: flags,
	}
}

var _ Match = &tunnelFlags{}

// tunnelFlags is a match for tunnel Flags
type tunnelFlags struct {
	flags int
}

// MarshalText implements Match.
func (t *tunnelFlags) MarshalText() ([]byte, error) {
	return bprintf("tun_flags=%d", t.flags), nil
}

// GoString implements Match.
func (t *tunnelFlags) GoString() string {
	return fmt.Sprintf("ovs.TunnelFlags(%d)", t.flags)
}

// NetworkTTL returns a new networkTTL
func NetworkTTL(ttl int) Match {
	return &networkTTL{
		ttl: ttl,
	}
}

var _ Match = &networkTTL{}

// networkTTL is a match for network time to live
type networkTTL struct {
	ttl int
}

// MarshalText implements Match.
func (t *networkTTL) MarshalText() ([]byte, error) {
	return bprintf("nw_ttl=%d", t.ttl), nil
}

// GoString implements Match.
func (t *networkTTL) GoString() string {
	return fmt.Sprintf("ovs.NetworkTTL(%d)", t.ttl)
}

// TunnelTTL returns a new tunnelTTL
func TunnelTTL(ttl int) Match {
	return &tunnelTTL{
		ttl: ttl,
	}
}

var _ Match = &tunnelTTL{}

// tunnelTTL is a match for a tunnel time to live
type tunnelTTL struct {
	ttl int
}

// MarshalText implements Match.
func (t *tunnelTTL) MarshalText() ([]byte, error) {
	return bprintf("tun_ttl=%d", t.ttl), nil
}

// GoString implements Match.
func (t *tunnelTTL) GoString() string {
	return fmt.Sprintf("ovs.TunnelTTL(%d)", t.ttl)
}

// ConjunctionID matches flows that have matched all dimension of a conjunction
// inside of the openflow table.
func ConjunctionID(id uint32) Match {
	return &conjunctionIDMatch{
		id: id,
	}
}

// TunnelTOS returns a new tunnelTOS
func TunnelTOS(tos int) Match {
	return &tunnelTOS{
		tos: tos,
	}
}

var _ Match = &tunnelTOS{}

// tunnelTOS is a match for a tunnel type of service
type tunnelTOS struct {
	tos int
}

// MarshalText implements Match.
func (t *tunnelTOS) MarshalText() ([]byte, error) {
	return bprintf("tun_tos=%d", t.tos), nil
}

// GoString implements Match.
func (t *tunnelTOS) GoString() string {
	return fmt.Sprintf("ovs.TunnelTOS(%d)", t.tos)
}

// A conjunctionIDMatch is a Match returned by ConjunctionID
type conjunctionIDMatch struct {
	id uint32
}

// MarshalText implements Match.
func (m *conjunctionIDMatch) MarshalText() ([]byte, error) {
	return bprintf("conj_id=%v", m.id), nil
}

// GoString implements Match.
func (m *conjunctionIDMatch) GoString() string {
	return fmt.Sprintf("ovs.ConjunctionID(%v)", m.id)
}

// NetworkProtocol matches packets with the specified IP or IPv6 protocol
// number matching num.  For example, specifying 1 when a Flow's Protocol
// is IPv4 matches ICMP packets, or 58 when Protocol is IPv6 matches ICMPv6
// packets.
func NetworkProtocol(num uint8) Match {
	return &networkProtocolMatch{
		num: num,
	}
}

var _ Match = &networkProtocolMatch{}

// A networkProtocolMatch is a Match returned by NetworkProtocol.
type networkProtocolMatch struct {
	num uint8
}

// MarshalText implements Match.
func (m *networkProtocolMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", nwProto, m.num), nil
}

// GoString implements Match.
func (m *networkProtocolMatch) GoString() string {
	return fmt.Sprintf("ovs.NetworkProtocol(%d)", m.num)
}

// IPv6Source matches packets with a source IPv6 address or IPv6 CIDR
// block matching ip.
func IPv6Source(ip string) Match {
	return &ipv6Match{
		srcdst: source,
		ip:     ip,
	}
}

// IPv6Destination matches packets with a destination IPv6 address or
// IPv6 CIDR block matching ip.
func IPv6Destination(ip string) Match {
	return &ipv6Match{
		srcdst: destination,
		ip:     ip,
	}
}

var _ Match = &ipv6Match{}

// An ipv6Match is a Match returned by IPv6{Source,Destination}.
type ipv6Match struct {
	srcdst string
	ip     string
}

// MarshalText implements Match.
func (m *ipv6Match) MarshalText() ([]byte, error) {
	return matchIPv6AddressOrCIDR(fmt.Sprintf("ipv6_%s", m.srcdst), m.ip)
}

// GoString implements Match.
func (m *ipv6Match) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.IPv6Source(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.IPv6Destination(%q)", m.ip)
}

// ICMPType matches packets with the specified ICMP type matching typ.
func ICMPType(typ uint8) Match {
	return &icmpTypeMatch{
		typ: typ,
	}
}

var _ Match = &icmpTypeMatch{}

// An icmpTypeMatch is a Match returned by ICMPType.
type icmpTypeMatch struct {
	typ uint8
}

// MarshalText implements Match.
func (m *icmpTypeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", icmpType, m.typ), nil
}

// GoString implements Match.
func (m *icmpTypeMatch) GoString() string {
	return fmt.Sprintf("ovs.ICMPType(%d)", m.typ)
}

// ICMPCode matches packets with the specified ICMP code.
func ICMPCode(code uint8) Match {
	return &icmpCodeMatch{
		code: code,
	}
}

var _ Match = &icmpCodeMatch{}

// An icmpCodeMatch is a Match returned by ICMPCode.
type icmpCodeMatch struct {
	code uint8
}

// MarshalText implements Match.
func (m *icmpCodeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", icmpCode, m.code), nil
}

// GoString implements Match.
func (m *icmpCodeMatch) GoString() string {
	return fmt.Sprintf("ovs.ICMPCode(%d)", m.code)
}

// ICMP6Type matches packets with the specified ICMP type matching typ.
func ICMP6Type(typ uint8) Match {
	return &icmp6TypeMatch{
		typ: typ,
	}
}

var _ Match = &icmp6TypeMatch{}

// An icmp6TypeMatch is a Match returned by ICMP6Type.
type icmp6TypeMatch struct {
	typ uint8
}

// MarshalText implements Match.
func (m *icmp6TypeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", icmp6Type, m.typ), nil
}

// GoString implements Match.
func (m *icmp6TypeMatch) GoString() string {
	return fmt.Sprintf("ovs.ICMP6Type(%d)", m.typ)
}

// ICMP6Code matches packets with the specified ICMP type matching typ.
func ICMP6Code(code uint8) Match {
	return &icmp6CodeMatch{
		code: code,
	}
}

var _ Match = &icmp6CodeMatch{}

// An icmp6CodeMatch is a Match returned by ICMP6Code.
type icmp6CodeMatch struct {
	code uint8
}

// MarshalText implements Match.
func (m *icmp6CodeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", icmp6Code, m.code), nil
}

// GoString implements Match.
func (m *icmp6CodeMatch) GoString() string {
	return fmt.Sprintf("ovs.ICMP6Code(%d)", m.code)
}

// InPortMatch matches packets ingressing from a specified OVS port
func InPortMatch(port int) Match {
	return &inPortMatch{
		port: port,
	}
}

var _ Match = &inPortMatch{}

// inPort matches packets ingressing from a specified OVS port
type inPortMatch struct {
	port int
}

// MarshalText implements Match.
func (i *inPortMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", inPort, i.port), nil
}

// GoString implements Match.
func (i *inPortMatch) GoString() string {
	return fmt.Sprintf("ovs.InPort(%q)", i.port)
}

// NeighborDiscoveryTarget matches packets with an IPv6 neighbor discovery target
// IPv6 address or IPv6 CIDR block matching ip.
func NeighborDiscoveryTarget(ip string) Match {
	return &neighborDiscoveryTargetMatch{
		ip: ip,
	}
}

var _ Match = &neighborDiscoveryTargetMatch{}

// A neighborDiscoveryTargetMatch is a Match returned by NeighborDiscoveryTarget.
type neighborDiscoveryTargetMatch struct {
	ip string
}

// MarshalText implements Match.
func (m *neighborDiscoveryTargetMatch) MarshalText() ([]byte, error) {
	return matchIPv6AddressOrCIDR(ndTarget, m.ip)
}

// GoString implements Match.
func (m *neighborDiscoveryTargetMatch) GoString() string {
	return fmt.Sprintf("ovs.NeighborDiscoveryTarget(%q)", m.ip)
}

// NeighborDiscoverySourceLinkLayer matches packets with an IPv6 neighbor
// solicitation source link-layer address matching addr.
func NeighborDiscoverySourceLinkLayer(addr net.HardwareAddr) Match {
	return &neighborDiscoveryLinkLayerMatch{
		srctgt: source,
		addr:   addr,
	}
}

// NeighborDiscoveryTargetLinkLayer matches packets with an IPv6 neighbor
// solicitation target link-layer address matching addr.
func NeighborDiscoveryTargetLinkLayer(addr net.HardwareAddr) Match {
	return &neighborDiscoveryLinkLayerMatch{
		srctgt: destination,
		addr:   addr,
	}
}

var _ Match = &neighborDiscoveryLinkLayerMatch{}

// A neighborDiscoveryLinkLayerMatch is a Match returned by DataLinkVLAN.
type neighborDiscoveryLinkLayerMatch struct {
	srctgt string
	addr   net.HardwareAddr
}

// MarshalText implements Match.
func (m *neighborDiscoveryLinkLayerMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchEthernetHardwareAddress(ndSLL, m.addr)
	}

	return matchEthernetHardwareAddress(ndTLL, m.addr)
}

// GoString implements Match.
func (m *neighborDiscoveryLinkLayerMatch) GoString() string {
	syntax := hwAddrGoString(m.addr)

	if m.srctgt == source {
		return fmt.Sprintf("ovs.NeighborDiscoverySourceLinkLayer(%s)", syntax)
	}

	return fmt.Sprintf("ovs.NeighborDiscoveryTargetLinkLayer(%s)", syntax)
}

// ARPOperation matches packets with the specified ARP operation matching oper.
func ARPOperation(oper uint16) Match {
	return &arpOperationMatch{
		oper: oper,
	}
}

var _ Match = &arpOperationMatch{}

// An arpOperationMatch is a Match returned by ARPOperation.
type arpOperationMatch struct {
	oper uint16
}

// MarshalText implements Match.
func (m *arpOperationMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", arpOp, m.oper), nil
}

// GoString implements Match.
func (m *arpOperationMatch) GoString() string {
	return fmt.Sprintf("ovs.ARPOperation(%d)", m.oper)
}

// ARPSourceHardwareAddress matches packets with an ARP source hardware address
// (SHA) matching addr.
func ARPSourceHardwareAddress(addr net.HardwareAddr) Match {
	return &arpHardwareAddressMatch{
		srctgt: source,
		addr:   addr,
	}
}

// ARPTargetHardwareAddress matches packets with an ARP target hardware address
// (THA) matching addr.
func ARPTargetHardwareAddress(addr net.HardwareAddr) Match {
	return &arpHardwareAddressMatch{
		srctgt: destination,
		addr:   addr,
	}
}

var _ Match = &arpHardwareAddressMatch{}

// An arpHardwareAddressMatch is a Match returned by ARP{Source,Target}HardwareAddress.
type arpHardwareAddressMatch struct {
	srctgt string
	addr   net.HardwareAddr
}

// MarshalText implements Match.
func (m *arpHardwareAddressMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchEthernetHardwareAddress(arpSHA, m.addr)
	}

	return matchEthernetHardwareAddress(arpTHA, m.addr)
}

// GoString implements Match.
func (m *arpHardwareAddressMatch) GoString() string {
	syntax := hwAddrGoString(m.addr)

	if m.srctgt == source {
		return fmt.Sprintf("ovs.ARPSourceHardwareAddress(%s)", syntax)
	}

	return fmt.Sprintf("ovs.ARPTargetHardwareAddress(%s)", syntax)
}

// ARPSourceProtocolAddress matches packets with an ARP source protocol address
// (SPA) IPv4 address or IPv4 CIDR block matching addr.
func ARPSourceProtocolAddress(ip string) Match {
	return &arpProtocolAddressMatch{
		srctgt: source,
		ip:     ip,
	}
}

// ARPTargetProtocolAddress matches packets with an ARP target protocol address
// (TPA) IPv4 address or IPv4 CIDR block matching addr.
func ARPTargetProtocolAddress(ip string) Match {
	return &arpProtocolAddressMatch{
		srctgt: destination,
		ip:     ip,
	}
}

var _ Match = &arpProtocolAddressMatch{}

// An arpProtocolAddressMatch is a Match returned by ARP{Source,Target}ProtocolAddress.
type arpProtocolAddressMatch struct {
	srctgt string
	ip     string
}

// MarshalText implements Match.
func (m *arpProtocolAddressMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchIPv4AddressOrCIDR(arpSPA, m.ip)
	}

	return matchIPv4AddressOrCIDR(arpTPA, m.ip)
}

// GoString implements Match.
func (m *arpProtocolAddressMatch) GoString() string {
	if m.srctgt == source {
		return fmt.Sprintf("ovs.ARPSourceProtocolAddress(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.ARPTargetProtocolAddress(%q)", m.ip)
}

// TransportSourcePort matches packets with a transport layer (TCP/UDP) source
// port matching port.
func TransportSourcePort(port uint16) Match {
	return &transportPortMatch{
		srcdst: source,
		port:   port,
		mask:   0,
	}
}

// TransportDestinationPort matches packets with a transport layer (TCP/UDP)
// destination port matching port.
func TransportDestinationPort(port uint16) Match {
	return &transportPortMatch{
		srcdst: destination,
		port:   port,
		mask:   0,
	}
}

// TransportSourceMaskedPort matches packets with a transport layer (TCP/UDP)
// source port matching a masked port range.
func TransportSourceMaskedPort(port uint16, mask uint16) Match {
	return &transportPortMatch{
		srcdst: source,
		port:   port,
		mask:   mask,
	}
}

// TransportDestinationMaskedPort matches packets with a transport layer (TCP/UDP)
// destination port matching a masked port range.
func TransportDestinationMaskedPort(port uint16, mask uint16) Match {
	return &transportPortMatch{
		srcdst: destination,
		port:   port,
		mask:   mask,
	}
}

// A transportPortMatch is a Match returned by Transport{Source,Destination}Port.
type transportPortMatch struct {
	srcdst string
	port   uint16
	mask   uint16
}

var _ Match = &transportPortMatch{}

// A TransportPortRanger represents a port range that can be expressed as an array of bitwise matches.
type TransportPortRanger interface {
	MaskedPorts() ([]Match, error)
}

// A TransportPortRange reprsents the start and end values of a transport protocol port range.
type transportPortRange struct {
	srcdst    string
	startPort uint16
	endPort   uint16
}

// TransportDestinationPortRange represent a port range intended for a transport protocol destination port.
func TransportDestinationPortRange(startPort uint16, endPort uint16) TransportPortRanger {
	return &transportPortRange{
		srcdst:    destination,
		startPort: startPort,
		endPort:   endPort,
	}
}

// TransportSourcePortRange represent a port range intended for a transport protocol source port.
func TransportSourcePortRange(startPort uint16, endPort uint16) TransportPortRanger {
	return &transportPortRange{
		srcdst:    source,
		startPort: startPort,
		endPort:   endPort,
	}
}

// MaskedPorts returns the represented port ranges as an array of bitwise matches.
func (pr *transportPortRange) MaskedPorts() ([]Match, error) {
	portRange := PortRange{
		Start: pr.startPort,
		End:   pr.endPort,
	}

	bitRanges, err := portRange.BitwiseMatch()
	if err != nil {
		return nil, err
	}

	var ports []Match

	for _, br := range bitRanges {
		maskedPortRange := &transportPortMatch{
			srcdst: pr.srcdst,
			port:   br.Value,
			mask:   br.Mask,
		}
		ports = append(ports, maskedPortRange)
	}

	return ports, nil
}

// MarshalText implements Match.
func (m *transportPortMatch) MarshalText() ([]byte, error) {
	return matchTransportPort(m.srcdst, m.port, m.mask)
}

// GoString implements Match.
func (m *transportPortMatch) GoString() string {
	if m.mask > 0 {
		if m.srcdst == source {
			return fmt.Sprintf("ovs.TransportSourceMaskedPort(%#x, %#x)", m.port, m.mask)
		}

		return fmt.Sprintf("ovs.TransportDestinationMaskedPort(%#x, %#x)", m.port, m.mask)
	}

	if m.srcdst == source {
		return fmt.Sprintf("ovs.TransportSourcePort(%d)", m.port)
	}

	return fmt.Sprintf("ovs.TransportDestinationPort(%d)", m.port)
}

// A vlanTCIMatch is a Match returned by VLANTCI.
type vlanTCIMatch struct {
	tci  uint16
	mask uint16
}

// VLANTCI matches packets based on their VLAN tag control information, using
// the specified TCI and optional mask value.
func VLANTCI(tci, mask uint16) Match {
	return &vlanTCIMatch{
		tci:  tci,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *vlanTCIMatch) MarshalText() ([]byte, error) {
	if m.mask != 0 {
		return bprintf("%s=0x%04x/0x%04x", vlanTCI, m.tci, m.mask), nil
	}

	return bprintf("%s=0x%04x", vlanTCI, m.tci), nil
}

// GoString implements Match.
func (m *vlanTCIMatch) GoString() string {
	return fmt.Sprintf("ovs.VLANTCI(0x%04x, 0x%04x)", m.tci, m.mask)
}

// A vlanTCI1Match is a Match returned by VLANTCI1.
type vlanTCI1Match struct {
	tci  uint16
	mask uint16
}

// VLANTCI1 matches packets based on their VLAN tag control information, using
// the specified TCI and optional mask value.
func VLANTCI1(tci, mask uint16) Match {
	return &vlanTCI1Match{
		tci:  tci,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *vlanTCI1Match) MarshalText() ([]byte, error) {
	if m.mask != 0 {
		return bprintf("%s=0x%04x/0x%04x", vlanTCI1, m.tci, m.mask), nil
	}

	return bprintf("%s=0x%04x", vlanTCI1, m.tci), nil
}

// GoString implements Match.
func (m *vlanTCI1Match) GoString() string {
	return fmt.Sprintf("ovs.VLANTCI1(0x%04x, 0x%04x)", m.tci, m.mask)
}

// An ipv6LabelMatch is a Match returned by IPv6Label.
type ipv6LabelMatch struct {
	label uint32
	mask  uint32
}

// IPv6Label matches packets based on their IPv6 label information, using
// the specified label and optional mask value.
func IPv6Label(label, mask uint32) Match {
	return &ipv6LabelMatch{
		label: label,
		mask:  mask,
	}
}

// MarshalText implements Match.
func (m *ipv6LabelMatch) MarshalText() ([]byte, error) {
	if !validIPv6Label(m.label) || !validIPv6Label(m.mask) {
		return nil, errInvalidIPv6Label
	}
	if m.mask != 0 {
		return bprintf("%s=0x%05x/0x%05x", ipv6Label, m.label, m.mask), nil
	}

	return bprintf("%s=0x%05x", ipv6Label, m.label), nil
}

// GoString implements Match.
func (m *ipv6LabelMatch) GoString() string {
	return fmt.Sprintf("ovs.IPv6Label(0x%04x, 0x%04x)", m.label, m.mask)
}

// An arpOpMatch is a Match returned by ArpOp.
type arpOpMatch struct {
	op uint16
}

// ArpOp matches packets based on their IPv6 label information, using
// the specified op.
func ArpOp(op uint16) Match {
	return &arpOpMatch{
		op: op,
	}
}

// MarshalText implements Match.
func (m *arpOpMatch) MarshalText() ([]byte, error) {
	if !validARPOP(m.op) {
		return nil, errInvalidARPOP
	}

	return bprintf("%s=%1d", arpOp, m.op), nil
}

// GoString implements Match.
func (m *arpOpMatch) GoString() string {
	return fmt.Sprintf("ovs.ArpOp(%01d)", m.op)
}

// A connectionTrackingMarkMatch is a Match returned by ConnectionTrackingMark.
type connectionTrackingMarkMatch struct {
	mark uint32
	mask uint32
}

// ConnectionTrackingMark matches a metadata associated with a connection tracking entry
func ConnectionTrackingMark(mark, mask uint32) Match {
	return &connectionTrackingMarkMatch{
		mark: mark,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *connectionTrackingMarkMatch) MarshalText() ([]byte, error) {
	if m.mask != 0 {
		return bprintf("%s=0x%08x/0x%08x", ctMark, m.mark, m.mask), nil
	}

	return bprintf("%s=0x%08x", ctMark, m.mark), nil
}

// GoString implements Match.
func (m *connectionTrackingMarkMatch) GoString() string {
	return fmt.Sprintf("ovs.ConnectionTrackingMark(0x%08x, 0x%08x)", m.mark, m.mask)
}

// A connectionTrackingZoneMatch is a Match returned by ConnectionTrackingZone.
type connectionTrackingZoneMatch struct {
	zone uint16
}

// ConnectionTrackingZone is a mechanism to define separate connection tracking contexts.
func ConnectionTrackingZone(zone uint16) Match {
	return &connectionTrackingZoneMatch{
		zone: zone,
	}
}

// MarshalText implements Match.
func (m *connectionTrackingZoneMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", ctZone, m.zone), nil
}

// GoString implements Match.
func (m *connectionTrackingZoneMatch) GoString() string {
	return fmt.Sprintf("ovs.ConnectionTrackingZone(%d)", m.zone)
}

// ConnectionTrackingState matches packets using their connection state, when
// connection tracking is enabled on the host.  Use the SetState and UnsetState
// functions to populate the parameter list for this function.
func ConnectionTrackingState(state ...string) Match {
	return &connectionTrackingMatch{
		state: state,
	}
}

var _ Match = &connectionTrackingMatch{}

// A connectionTrackingMatch is a Match returned by ConnectionTrackingState.
type connectionTrackingMatch struct {
	state []string
}

// MarshalText implements Match.
func (m *connectionTrackingMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", ctState, strings.Join(m.state, "")), nil
}

// GoString implements Match.
func (m *connectionTrackingMatch) GoString() string {
	buf := bytes.NewBuffer(nil)
	for i, s := range m.state {
		_, _ = buf.WriteString(fmt.Sprintf("%q", s))

		if i != len(m.state)-1 {
			_, _ = buf.WriteString(", ")
		}
	}

	return fmt.Sprintf("ovs.ConnectionTrackingState(%s)", buf.String())
}

// CTState is a connection tracking state, which can be used with the
// ConnectionTrackingState function.
type CTState string

// List of common CTState constants available in OVS 2.5.  Reference the
// ovs-ofctl man-page for a description of each one.
const (
	CTStateNew         CTState = "new"
	CTStateEstablished CTState = "est"
	CTStateRelated     CTState = "rel"
	CTStateReply       CTState = "rpl"
	CTStateInvalid     CTState = "inv"
	CTStateTracked     CTState = "trk"
)

// SetState sets the specified CTState flag.  This helper should be used
// with ConnectionTrackingState.
func SetState(state CTState) string {
	return fmt.Sprintf("+%s", state)
}

// UnsetState unsets the specified CTState flag.  This helper should be used
// with ConnectionTrackingState.
func UnsetState(state CTState) string {
	return fmt.Sprintf("-%s", state)
}

// Metadata returns a Match that matches the given Metadata exactly.
func Metadata(id uint64) Match {
	return &metadataMatch{
		data: id,
		mask: 0,
	}
}

// MetadataWithMask returns a Match with specified Metadata and mask.
func MetadataWithMask(id, mask uint64) Match {
	return &metadataMatch{
		data: id,
		mask: mask,
	}
}

var _ Match = &metadataMatch{}

// A metadataMatch is a Match against a Metadata field.
type metadataMatch struct {
	data uint64
	mask uint64
}

// GoString implements Match.
func (m *metadataMatch) GoString() string {
	if m.mask > 0 {
		return fmt.Sprintf("ovs.MetadataWithMask(%#x, %#x)", m.data, m.mask)
	}

	return fmt.Sprintf("ovs.Metadata(%#x)", m.data)
}

// MarshalText implements Match.
func (m *metadataMatch) MarshalText() ([]byte, error) {
	if m.mask == 0 {
		return bprintf("%s=%#x", metadata, m.data), nil
	}

	return bprintf("%s=%#x/%#x", metadata, m.data, m.mask), nil
}

// TCPFlags matches packets using their enabled TCP flags, when matching TCP
// flags on a TCP segment.   Use the SetTCPFlag and UnsetTCPFlag functions to
// populate the parameter list for this function.
func TCPFlags(flags ...string) Match {
	return &tcpFlagsMatch{
		flags: flags,
	}
}

var _ Match = &tcpFlagsMatch{}

// A tcpFlagsMatch is a Match returned by TCPFlags.
type tcpFlagsMatch struct {
	flags []string
}

// MarshalText implements Match.
func (m *tcpFlagsMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", tcpFlags, strings.Join(m.flags, "")), nil
}

// GoString implements Match.
func (m *tcpFlagsMatch) GoString() string {
	buf := bytes.NewBuffer(nil)
	for i, s := range m.flags {
		_, _ = buf.WriteString(fmt.Sprintf("%q", s))

		if i != len(m.flags)-1 {
			_, _ = buf.WriteString(", ")
		}
	}

	return fmt.Sprintf("ovs.TCPFlags(%s)", buf.String())
}

// TCPFlag represents a flag in the TCP header, which can be used with the
// TCPFlags function.
type TCPFlag string

// RFC 793 TCP Flags
const (
	TCPFlagURG TCPFlag = "urg"
	TCPFlagACK TCPFlag = "ack"
	TCPFlagPSH TCPFlag = "psh"
	TCPFlagRST TCPFlag = "rst"
	TCPFlagSYN TCPFlag = "syn"
	TCPFlagFIN TCPFlag = "fin"
)

// SetTCPFlag sets the specified TCPFlag.  This helper should be used
// with TCPFlags.
func SetTCPFlag(flag TCPFlag) string {
	return fmt.Sprintf("+%s", flag)
}

// UnsetTCPFlag unsets the specified TCPFlag.  This helper should be used
// with TCPFlags.
func UnsetTCPFlag(flag TCPFlag) string {
	return fmt.Sprintf("-%s", flag)
}

// TunnelID returns a Match that matches the given ID exactly.
func TunnelID(id uint64) Match {
	return &tunnelIDMatch{
		id:   id,
		mask: 0,
	}
}

// TunnelIDWithMask returns a Match with specified ID and mask.
func TunnelIDWithMask(id, mask uint64) Match {
	return &tunnelIDMatch{
		id:   id,
		mask: mask,
	}
}

var _ Match = &tunnelIDMatch{}

// A tunnelIDMatch is a Match against a tunnel ID.
type tunnelIDMatch struct {
	id   uint64
	mask uint64
}

// GoString implements Match.
func (m *tunnelIDMatch) GoString() string {
	if m.mask > 0 {
		return fmt.Sprintf("ovs.TunnelIDWithMask(%#x, %#x)", m.id, m.mask)
	}

	return fmt.Sprintf("ovs.TunnelID(%#x)", m.id)
}

// MarshalText implements Match.
func (m *tunnelIDMatch) MarshalText() ([]byte, error) {
	if m.mask == 0 {
		return bprintf("%s=%#x", tunID, m.id), nil
	}

	return bprintf("%s=%#x/%#x", tunID, m.id, m.mask), nil
}

// TunnelSrc returns a Match with specified Tunnel Source.
func TunnelSrc(addr string) Match {
	return &tunnelMatch{
		srcdst: source,
		ip:     addr,
	}
}

// TunnelDst returns a Match with specified Tunnel Destination.
func TunnelDst(addr string) Match {
	return &tunnelMatch{
		srcdst: destination,
		ip:     addr,
	}
}

var _ Match = &tunnelMatch{}

// A tunnelMatch is a Match against a tunnel {source|destination}.
type tunnelMatch struct {
	srcdst string
	ip     string
}

// GoString implements Match.
func (m *tunnelMatch) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.TunnelSrc(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.TunnelDst(%q)", m.ip)
}

// MarshalText implements Match.
func (m *tunnelMatch) MarshalText() ([]byte, error) {
	return matchIPv4AddressOrCIDR(fmt.Sprintf("tun_%s", m.srcdst), m.ip)
}

// matchIPv4AddressOrCIDR attempts to create a Match using the specified key
// and input string, which could be interpreted as an IPv4 address or IPv4
// CIDR block.
func matchIPv4AddressOrCIDR(key string, ip string) ([]byte, error) {
	errInvalidIPv4 := fmt.Errorf("%q is not a valid IPv4 address or IPv4 CIDR block", ip)

	if ipAddr, _, err := net.ParseCIDR(ip); err == nil {
		if ipAddr.To4() == nil {
			return nil, errInvalidIPv4
		}

		return bprintf("%s=%s", key, ip), nil
	}

	if ipAddr := net.ParseIP(ip); ipAddr != nil {
		if ipAddr.To4() == nil {
			return nil, errInvalidIPv4
		}

		return bprintf("%s=%s", key, ipAddr.String()), nil
	}

	return nil, errInvalidIPv4
}

// matchIPv6AddressOrCIDR attempts to create a Match using the specified key
// and input string, which could be interpreted as an IPv6 address or IPv6
// CIDR block.
func matchIPv6AddressOrCIDR(key string, ip string) ([]byte, error) {
	errInvalidIPv6 := fmt.Errorf("%q is not a valid IPv6 address or IPv6 CIDR block", ip)

	if ipAddr, _, err := net.ParseCIDR(ip); err == nil {
		if ipAddr.To16() == nil || ipAddr.To4() != nil {
			return nil, errInvalidIPv6
		}

		return bprintf("%s=%s", key, ip), nil
	}

	if ipAddr := net.ParseIP(ip); ipAddr != nil {
		if ipAddr.To16() == nil || ipAddr.To4() != nil {
			return nil, errInvalidIPv6
		}

		return bprintf("%s=%s", key, ipAddr.String()), nil
	}

	return nil, errInvalidIPv6
}

// matchEthernetHardwareAddress attempts to create a Match using the specified
// key and input hardware address, which must be a 6-octet Ethernet hardware
// address.
func matchEthernetHardwareAddress(key string, addr net.HardwareAddr) ([]byte, error) {
	if len(addr) != ethernetAddrLen {
		return nil, fmt.Errorf("hardware address must be %d octets, but got %d",
			ethernetAddrLen, len(addr))
	}

	return bprintf("%s=%s", key, addr.String()), nil
}

// matchTransportPort is the common implementation for
// Transport{Source,Destination}Port.
func matchTransportPort(srcdst string, port uint16, mask uint16) ([]byte, error) {
	// No mask specified
	if mask == 0 {
		return bprintf("tp_%s=%d", srcdst, port), nil
	}

	return bprintf("tp_%s=0x%04x/0x%04x", srcdst, port, mask), nil
}

// IPFragFlag is a string type which can be used with the IPFragMatch.
type IPFragFlag string

// OvS IP frag flags.
// Source: http://www.openvswitch.org/support/dist-docs-2.5/ovs-ofctl.8.txt
const (
	IPFragFlagYes      IPFragFlag = "yes"
	IPFragFlagNo       IPFragFlag = "no"
	IPFragFlagFirst    IPFragFlag = "first"
	IPFragFlagLater    IPFragFlag = "later"
	IPFragFlagNotLater IPFragFlag = "not_later"
)

// IPFrag returns an ipFragMatch.
func IPFrag(flag IPFragFlag) Match {
	return &ipFragMatch{flag: flag}
}

// ipFragMatch implements the Match interface and is a match against
// a packet fragmentation value.
type ipFragMatch struct {
	flag IPFragFlag
}

var _ Match = &ipFragMatch{}

// GoString implements Match.
func (m *ipFragMatch) GoString() string {
	return fmt.Sprintf("ovs.IpFrag(%v)", m.flag)
}

// MarshalText implements Match.
func (m *ipFragMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", ipFrag, m.flag), nil
}

// FieldMatch returns an fieldMatch.
func FieldMatch(field, srcOrValue string) Match {
	return &fieldMatch{field: field, srcOrValue: srcOrValue}
}

// fieldMatch implements the Match interface and
// matches a given field against another a value, e.g. "0x123" or "1.2.3.4",
// or against another src field in the packet, e.g "arp_tpa" or "NXM_OF_ARP_TPA[]".
type fieldMatch struct {
	field      string
	srcOrValue string
}

var _ Match = &fieldMatch{}

// GoString implements Match.
func (m *fieldMatch) GoString() string {
	return fmt.Sprintf("ovs.FieldMatch(%v,%v)", m.field, m.srcOrValue)
}

// MarshalText implements Match.
func (m *fieldMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", m.field, m.srcOrValue), nil
}
