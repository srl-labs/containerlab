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
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
)

// parseMatch creates a Match function from the input string.
func parseMatch(key string, value string) (Match, error) {
	switch key {
	case arpSHA, arpTHA, ndSLL, ndTLL:
		return parseMACMatch(key, value)
	case arpOp:
		return parseArpOp(value)
	case icmpType, icmpCode, icmp6Type, icmp6Code, nwProto:
		return parseIntMatch(key, value, math.MaxUint8)
	case ctZone:
		return parseIntMatch(key, value, math.MaxUint16)
	case tpSRC, tpDST:
		return parsePort(key, value, math.MaxUint16)
	case conjID:
		return parseIntMatch(key, value, math.MaxUint32)
	case arpSPA:
		return ARPSourceProtocolAddress(value), nil
	case arpTPA:
		return ARPTargetProtocolAddress(value), nil
	case ctState:
		return parseCTState(value)
	case tcpFlags:
		return parseTCPFlags(value)
	case dlSRC:
		return DataLinkSource(value), nil
	case dlDST:
		return DataLinkDestination(value), nil
	case dlType:
		etherType, err := parseHexUint16(value)
		if err != nil {
			return nil, err
		}

		return DataLinkType(etherType), nil
	case dlVLANPCP:
		return parseDataLinkVLANPCP(value)
	case dlVLAN:
		return parseDataLinkVLAN(value)
	case ndTarget:
		return NeighborDiscoveryTarget(value), nil
	case nwECN:
		return parseIntMatch(key, value, math.MaxInt32)
	case nwTTL:
		return parseIntMatch(key, value, math.MaxInt32)
	case tunTTL:
		return parseIntMatch(key, value, math.MaxInt32)
	case tunTOS:
		return parseIntMatch(key, value, math.MaxInt32)
	case nwTOS:
		return parseIntMatch(key, value, math.MaxInt32)
	case tunGbpID:
		return parseIntMatch(key, value, math.MaxInt32)
	case tunGbpFlags:
		return parseIntMatch(key, value, math.MaxInt32)
	case tunFlags:
		return parseIntMatch(key, value, math.MaxInt32)
	case inPort:
		return parseIntMatch(key, value, math.MaxInt32)
	case ipv6SRC:
		return IPv6Source(value), nil
	case ipv6DST:
		return IPv6Destination(value), nil
	case metadata:
		return parseMetadata(value)
	case tunv6SRC:
		return IPv6Source(value), nil
	case tunv6DST:
		return IPv6Destination(value), nil
	case ipv6Label:
		return parseIPv6Label(value)
	case nwSRC:
		return NetworkSource(value), nil
	case tunSRC:
		return NetworkSource(value), nil
	case tunDST:
		return NetworkDestination(value), nil
	case nwDST:
		return NetworkDestination(value), nil
	case vlanTCI1:
		return parseVLANTCI1(value)
	case vlanTCI:
		return parseVLANTCI(value)
	case ctMark:
		return parseCTMark(value)
	case tunID:
		return parseTunID(value)
	}

	return nil, nil
}

// parseClampInt calls strconv.Atoi on s, and then ensures that s is less than
// or equal to the integer specified by max.
func parseClampInt(s string, max int) (int, error) {
	t, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if t > max {
		return 0, fmt.Errorf("integer %d too large; %d > %d", t, t, max)
	}

	return t, nil
}

// parseIntMatch parses an integer Match value from the input key and value,
// with a maximum possible value of max.
func parseIntMatch(key string, value string, max int) (Match, error) {
	t, err := parseClampInt(value, max)
	if err != nil {
		return nil, err
	}

	switch key {
	case icmpType:
		return ICMPType(uint8(t)), nil
	case icmpCode:
		return ICMPCode(uint8(t)), nil
	case icmp6Type:
		return ICMP6Type(uint8(t)), nil
	case icmp6Code:
		return ICMP6Code(uint8(t)), nil
	case inPort:
		return InPortMatch(int(t)), nil
	case nwECN:
		return NetworkECN(int(t)), nil
	case nwTTL:
		return NetworkTTL(int(t)), nil
	case tunTTL:
		return TunnelTTL(int(t)), nil
	case tunTOS:
		return TunnelTOS(int(t)), nil
	case nwTOS:
		return NetworkTOS(int(t)), nil
	case tunGbpID:
		return TunnelGBP(int(t)), nil
	case tunGbpFlags:
		return TunnelGbpFlags(int(t)), nil
	case tunFlags:
		return TunnelFlags(int(t)), nil
	case nwProto:
		return NetworkProtocol(uint8(t)), nil
	case ctZone:
		return ConnectionTrackingZone(uint16(t)), nil
	case conjID:
		return ConjunctionID(uint32(t)), nil
	}

	return nil, fmt.Errorf("no action matched for %s=%s", key, value)
}

// parsePort parses a port or port/mask Match value from the input key and value,
// with a maximum possible value of max.
func parsePort(key string, value string, max int) (Match, error) {

	var values []uint64
	//Split the string
	ss := strings.Split(value, "/")

	//If input is just port
	switch len(ss) {
	case 1:
		val, err := parseClampInt(value, max)
		if err != nil {
			return nil, err
		}
		values = append(values, uint64(val))
		values = append(values, 0)
		// If input is port/mask
	case 2:
		for _, s := range ss {
			val, err := parseHexUint64(s)
			if err != nil {
				return nil, err
			}
			// Return error if val > 65536 (uint16)
			if val > uint64(max) {
				return nil, fmt.Errorf("integer %d too large; %d > %d", val, val, max)
			}

			values = append(values, val)
		}
	default:
		return nil, fmt.Errorf("invalid value, no action matched for %s=%s", key, value)
	}

	switch key {
	case tpSRC:
		return TransportSourceMaskedPort(uint16(values[0]), uint16(values[1])), nil
	case tpDST:
		return TransportDestinationMaskedPort(uint16(values[0]), uint16(values[1])), nil
	}
	// Return error if input is invalid
	return nil, fmt.Errorf("no action matched for %s=%s", key, value)
}

// parseMACMatch parses a MAC address Match value from the input key and value.
func parseMACMatch(key string, value string) (Match, error) {
	mac, err := net.ParseMAC(value)
	if err != nil {
		return nil, err
	}

	switch key {
	case arpSHA:
		return ARPSourceHardwareAddress(mac), nil
	case arpTHA:
		return ARPTargetHardwareAddress(mac), nil
	case ndSLL:
		return NeighborDiscoverySourceLinkLayer(mac), nil
	case ndTLL:
		return NeighborDiscoveryTargetLinkLayer(mac), nil
	}

	return nil, fmt.Errorf("no action matched for %s=%s", key, value)
}

// parseCTState parses a series of connection tracking values into a Match.
func parseCTState(value string) (Match, error) {
	// If the format use bar:
	// "est|trk|dnat" => "+est+trk+dnat"
	if strings.Contains(value, "|") {
		value = strings.ReplaceAll(value, "|", "+")
		value = "+" + value
	}

	// Add space between flags
	// "+est+trk+dnat-snat" => "+est +trk +dnat -snat"
	if strings.Contains(value, "+") || strings.Contains(value, "-") {
		value = strings.ReplaceAll(value, "+", " +")
		value = strings.ReplaceAll(value, "-", " -")
		value = strings.Trim(value, " ")
	} else {
		// Assume only one state is specified: "ct_state=trk"
		// "trk" => "+trk"
		value = "+" + value
	}

	states := strings.Fields(value)
	return ConnectionTrackingState(states...), nil
}

// parseTCPFlags parses a series of TCP flags into a Match.  Open vSwitch's representation
// of These TCP flags are outlined in the ovs-field(7) man page,
func parseTCPFlags(value string) (Match, error) {
	// tcp_flag can also be decimal number
	if _, err := strconv.Atoi(value); err == nil {
		return TCPFlags(value), nil
	}

	if len(value)%4 != 0 {
		return nil, errors.New("tcp_flags length must be divisible by 4")
	}

	var buf bytes.Buffer
	var flags []string

	for i, r := range value {
		if i != 0 && i%4 == 0 {
			flags = append(flags, buf.String())
			buf.Reset()
		}

		_, _ = buf.WriteRune(r)
	}
	flags = append(flags, buf.String())

	return TCPFlags(flags...), nil
}

// hexPrefix denotes that a string integer is in hex format.
const hexPrefix = "0x"

// parseDataLinkVLAN parses a DataLinkVLAN Match from value.
func parseDataLinkVLAN(value string) (Match, error) {
	if !strings.HasPrefix(value, hexPrefix) {
		vlan, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}

		return DataLinkVLAN(vlan), nil
	}

	vlan, err := parseHexUint16(value)
	if err != nil {
		return nil, err
	}

	return DataLinkVLAN(int(vlan)), nil
}

// parseDataLinkVLANPCP parses a DataLinkVLANPCP Match from value.
func parseDataLinkVLANPCP(value string) (Match, error) {
	if !strings.HasPrefix(value, hexPrefix) {
		pcp, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}

		return DataLinkVLANPCP(pcp), nil
	}

	pcp, err := parseHexUint16(value)
	if err != nil {
		return nil, err
	}

	return DataLinkVLANPCP(int(pcp)), nil
}

// parseVLANTCI parses a VLANTCI Match from value.
func parseVLANTCI(value string) (Match, error) {
	var values []uint16
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint16(v))
			continue
		}

		v, err := parseHexUint16(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return VLANTCI(values[0], 0), nil
	case 2:
		return VLANTCI(values[0], values[1]), nil
	// Match had too many parts, e.g. "vlan_tci=10/10/10"
	default:
		return nil, fmt.Errorf("invalid vlan_tci match: %q", value)
	}
}

// parseVLANTCI1 parses a VLANTCI1 Match from value.
func parseVLANTCI1(value string) (Match, error) {
	var values []uint16
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint16(v))
			continue
		}

		v, err := parseHexUint16(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return VLANTCI1(values[0], 0), nil
	case 2:
		return VLANTCI1(values[0], values[1]), nil
	// Match had too many parts, e.g. "vlan_tci1=10/10/10"
	default:
		return nil, fmt.Errorf("invalid vlan_tci1 match: %q", value)
	}
}

// parseIPv6Label parses a IPv6Label Match from value.
func parseIPv6Label(value string) (Match, error) {
	var values []uint32
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint32(v))
			continue
		}

		v, err := parseHexUint32(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return IPv6Label(values[0], 0), nil
	case 2:
		return IPv6Label(values[0], values[1]), nil
	// Match had too many parts, e.g. "ipv6_label=10/10/10"
	default:
		return nil, fmt.Errorf("invalid ipv6_label match: %q", value)
	}
}

// parseArpOp parses a ArpOp Match from value.
func parseArpOp(value string) (Match, error) {
	if !strings.HasPrefix(value, hexPrefix) {
		parsed, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return nil, err
		}
		return ArpOp(uint16(parsed)), nil
	}

	v, err := parseHexUint16(value)
	if err != nil {
		return nil, err
	}
	return ArpOp(v), nil
}

// parseCTMark parses a CTMark Match from value.
func parseCTMark(value string) (Match, error) {
	var values []uint32
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint32(v))
			continue
		}

		v, err := parseHexUint32(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return ConnectionTrackingMark(values[0], 0), nil
	case 2:
		return ConnectionTrackingMark(values[0], values[1]), nil
	// Match had too many parts, e.g. "ct_mark=10/10/10"
	default:
		return nil, fmt.Errorf("invalid ct_mark match: %q", value)
	}
}

// parseMetadata parses a Metadata Match from value.
func parseMetadata(value string) (Match, error) {
	var values []uint64
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint64(v))
			continue
		}

		v, err := parseHexUint64(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return Metadata(values[0]), nil
	case 2:
		return MetadataWithMask(values[0], values[1]), nil
	// Match had too many parts, e.g. "metadata=10/10/10"
	default:
		return nil, fmt.Errorf("invalid metadata match: %q", value)
	}
}

// parseTunID parses a tunID Match from value.
func parseTunID(value string) (Match, error) {
	var values []uint64
	for _, s := range strings.Split(value, "/") {
		if !strings.HasPrefix(s, hexPrefix) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}

			values = append(values, uint64(v))
			continue
		}

		v, err := parseHexUint64(s)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}

	switch len(values) {
	case 1:
		return TunnelID(values[0]), nil
	case 2:
		return TunnelIDWithMask(values[0], values[1]), nil
	// Match had too many parts, e.g. "tun_id=10/10/10"
	default:
		return nil, fmt.Errorf("invalid tun_id match: %q", value)
	}
}

// parseHexUint16 parses a uint16 value from a hexadecimal string.
func parseHexUint16(value string) (uint16, error) {
	val, err := strconv.ParseUint(strings.TrimPrefix(value, hexPrefix), 16, 32)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

// parseHexUint32 parses a uint32 value from a hexadecimal string.
func parseHexUint32(value string) (uint32, error) {
	val, err := strconv.ParseUint(strings.TrimPrefix(value, hexPrefix), 16, 32)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}

// parseHexUint64 parses a uint64 value from a hexadecimal string.
func parseHexUint64(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimPrefix(value, hexPrefix), 16, 64)
}
