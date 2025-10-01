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
	"encoding"
	"errors"
	"fmt"
	"net"
	"strconv"
)

var (
	// errCTNoArguments is returned when no arguments are passed to ActionCT.
	errCTNoArguments = errors.New("no arguments for connection tracking")

	// errInvalidIPv6Label is returned when an input IPv6 label is out of
	// range. It should only use the first 20 bits of the 32 bit field.
	errInvalidIPv6Label = errors.New("IPv6 label must only use 20 bits")

	// errInvalidARPOP is returned when an input ARP OP is out of
	// range. It should be in the range 1-4.
	errInvalidARPOP = errors.New("ARP OP must in the range 1-4")

	// errInvalidVLANVID is returned when an input VLAN VID is out of range
	// for a valid VLAN VID.
	errInvalidVLANVID = errors.New("VLAN VID must be between 0 and 4095")

	// errInvalidVLANVIDPCP is returned when an input VLAN PCP is out of range
	// for a valid VLAN PCP.
	errInvalidVLANPCP = errors.New("VLAN PCP must be between 0 and 7")

	// errOutputNegativePort is returned when Output is called with a
	// negative integer port.
	errOutputNegativePort = errors.New("output port number must not be negative")

	// errResubmitPortTableZero is returned when Resubmit is called with
	// both port and table value set to zero.
	errResubmitPortTableZero = errors.New("both port and table are zero for action resubmit")

	// errLoadSetFieldZero is returned when Load or SetField is called with value and/or
	// field set to empty strings.
	errLoadSetFieldZero = errors.New("value and/or field for action load or set_field are empty")

	// errResubmitPortInvalid is returned when ResubmitPort is given a port number that is
	// invalid per the openflow spec.
	errResubmitPortInvalid = errors.New("resubmit port must be between 0 and 65279 inclusive")

	// errTooManyDimensions is returned when the specified dimension exceeds the total dimension
	// in a conjunction action.
	errDimensionTooLarge = errors.New("dimension number exceeds total number of dimensions")

	// errMoveEmpty is returned when Move is called with src and/or dst set to the empty string.
	errMoveEmpty = errors.New("src and/or dst field for action move are empty")

	// errOutputFieldEmpty is returned when OutputField is called with field set to the empty string.
	errOutputFieldEmpty = errors.New("field for action output (output:field syntax) is empty")

	// errLearnedNil is returned when Learn is called with a nil *LearnedFlow.
	errLearnedNil = errors.New("learned flow for action learn is nil")
)

// Action strings in lower case, as those are compared to the lower case letters
// in parseAction().
const (
	actionAll       = "all"
	actionDrop      = "drop"
	actionFlood     = "flood"
	actionInPort    = "in_port"
	actionLocal     = "local"
	actionNormal    = "normal"
	actionStripVLAN = "strip_vlan"
)

// An Action is a type which can be marshaled into an OpenFlow action. Actions can be
// used with Flows to perform operations when the Flow matches an input packet.
//
// Actions must also implement fmt.GoStringer for code generation purposes.
type Action interface {
	encoding.TextMarshaler
	fmt.GoStringer
}

// A textAction is an Action which is referred to by a name only, with no arguments.
type textAction struct {
	action string
}

// MarshalText implements Action.
func (a *textAction) MarshalText() ([]byte, error) {
	return []byte(a.action), nil
}

// GoString implements Action.
func (a *textAction) GoString() string {
	switch a.action {
	case actionAll:
		return "ovs.All()"
	case actionDrop:
		return "ovs.Drop()"
	case actionFlood:
		return "ovs.Flood()"
	case actionInPort:
		return "ovs.InPort()"
	case actionLocal:
		return "ovs.Local()"
	case actionNormal:
		return "ovs.Normal()"
	case actionStripVLAN:
		return "ovs.StripVLAN()"
	default:
		return fmt.Sprintf("// BUG(mdlayher): unimplemented OVS text action: %q", a.action)
	}
}

// All outputs the packet on all switch ports except
// the port on which it was received.
func All() Action {
	return &textAction{
		action: actionAll,
	}
}

// Drop immediately discards the packet.  It must be the only Action
// specified when used.
func Drop() Action {
	return &textAction{
		action: actionDrop,
	}
}

// Flood outputs the packet on all switch ports other than the port on which it
// was received, which have flooding enabled.
func Flood() Action {
	return &textAction{
		action: actionFlood,
	}
}

// InPort outputs the packet on the port from which it was received.
func InPort() Action {
	return &textAction{
		action: actionInPort,
	}
}

// Local outputs the packet on the local port, which corresponds to
// the network device that has the same name as the bridge.
func Local() Action {
	return &textAction{
		action: actionLocal,
	}
}

// Normal subjects the packet to the device's normal L2/L3 processing.
func Normal() Action {
	return &textAction{
		action: actionNormal,
	}
}

// StripVLAN strips the VLAN tag from a packet, if one is present.
func StripVLAN() Action {
	return &textAction{
		action: actionStripVLAN,
	}
}

// printf-style patterns for marshaling and unmarshaling actions.
const (
	patConnectionTracking          = "ct(%s)"
	patConjunction                 = "conjunction(%d,%d/%d)"
	patModDataLinkDestination      = "mod_dl_dst:%s"
	patModDataLinkSource           = "mod_dl_src:%s"
	patModNetworkDestination       = "mod_nw_dst:%s"
	patModNetworkSource            = "mod_nw_src:%s"
	patModTransportDestinationPort = "mod_tp_dst:%d"
	patModTransportSourcePort      = "mod_tp_src:%d"
	patModVLANVID                  = "mod_vlan_vid:%d"
	patOutput                      = "output:%d"
	patOutputField                 = "output:%s"
	patResubmitPort                = "resubmit:%s"
	patResubmitPortTable           = "resubmit(%s,%s)"
	patLearn                       = "learn(%s)"
)

// ConnectionTracking sends a packet through the host's connection tracker.
func ConnectionTracking(args string) Action {
	return &ctAction{
		args: args,
	}
}

// A ctAction is an Action which is used by ConneectionTracking.
type ctAction struct {
	// TODO(mdlayher): implement arguments type for ct() actions
	args string
}

// MarshalText implements Action.
func (a *ctAction) MarshalText() ([]byte, error) {
	if a.args == "" {
		return nil, errCTNoArguments
	}

	return bprintf(patConnectionTracking, a.args), nil
}

// GoString implements Action.
func (a *ctAction) GoString() string {
	return fmt.Sprintf("ovs.ConnectionTracking(%q)", a.args)
}

// ModDataLinkDestination modifies the data link destination of a packet.
func ModDataLinkDestination(addr net.HardwareAddr) Action {
	return &modDataLinkAction{
		srcdst: destination,
		addr:   addr,
	}
}

// ModDataLinkSource modifies the data link source of a packet.
func ModDataLinkSource(addr net.HardwareAddr) Action {
	return &modDataLinkAction{
		srcdst: source,
		addr:   addr,
	}
}

// A modDataLinkAction is an Action which is used by
// ModDataLink{Source,Destination}.
type modDataLinkAction struct {
	srcdst string
	addr   net.HardwareAddr
}

// MarshalText implements Action.
func (a *modDataLinkAction) MarshalText() ([]byte, error) {
	if len(a.addr) != ethernetAddrLen {
		return nil, fmt.Errorf("hardware address must be %d octets, but got %d",
			ethernetAddrLen, len(a.addr))
	}

	if a.srcdst == source {
		return bprintf(patModDataLinkSource, a.addr.String()), nil
	}

	return bprintf(patModDataLinkDestination, a.addr.String()), nil
}

// GoString implements Action.
func (a *modDataLinkAction) GoString() string {
	if a.srcdst == source {
		return fmt.Sprintf("ovs.ModDataLinkSource(%s)", hwAddrGoString(a.addr))
	}

	return fmt.Sprintf("ovs.ModDataLinkDestination(%s)", hwAddrGoString(a.addr))
}

// ModNetworkDestination modifies the destination IPv4 address of a packet.
func ModNetworkDestination(ip net.IP) Action {
	return &modNetworkAction{
		srcdst: destination,
		ip:     ip.To4(),
	}
}

// ModNetworkSource modifies the source IPv4 address of a packet.
func ModNetworkSource(ip net.IP) Action {
	return &modNetworkAction{
		srcdst: source,
		ip:     ip.To4(),
	}
}

// A modNetworkAction is an Action which is used by
// ModNetwork{Source,Destination}.
type modNetworkAction struct {
	srcdst string
	ip     net.IP
}

// MarshalText implements Action.
func (a *modNetworkAction) MarshalText() ([]byte, error) {
	if a.ip == nil {
		return nil, errors.New("invalid IPv4 address for ModNetwork action")
	}

	if a.srcdst == source {
		return bprintf(patModNetworkSource, a.ip.String()), nil
	}

	return bprintf(patModNetworkDestination, a.ip.String()), nil
}

// GoString implements Action.
func (a *modNetworkAction) GoString() string {
	if a.srcdst == source {
		return fmt.Sprintf("ovs.ModNetworkSource(%s)", ipv4GoString(a.ip))
	}

	return fmt.Sprintf("ovs.ModNetworkDestination(%s)", ipv4GoString(a.ip))
}

// ModTransportDestinationPort modifies the destination port of a packet.
func ModTransportDestinationPort(port uint16) Action {
	return &modTransportPortAction{
		srcdst: destination,
		port:   port,
	}
}

// ModTransportSourcePort modifies the source port of a packet.
func ModTransportSourcePort(port uint16) Action {
	return &modTransportPortAction{
		srcdst: source,
		port:   port,
	}
}

// A modTransportPortAction is an Action which is used by
// ModTransport{Source,Destination}Port.
type modTransportPortAction struct {
	srcdst string
	port   uint16
}

// MarshalText implements Action.
func (a *modTransportPortAction) MarshalText() ([]byte, error) {
	if a.srcdst == source {
		return bprintf(patModTransportSourcePort, a.port), nil
	}

	return bprintf(patModTransportDestinationPort, a.port), nil
}

// GoString implements Action.
func (a *modTransportPortAction) GoString() string {
	if a.srcdst == source {
		return fmt.Sprintf("ovs.ModTransportSourcePort(%d)", a.port)
	}

	return fmt.Sprintf("ovs.ModTransportDestinationPort(%d)", a.port)
}

// ModVLANVID modifies the VLAN ID (VID) on a packet.  It adds a VLAN
// tag if one is not already present.  vid must be a valid VLAN VID, within
// the range of 0 to 4095.
func ModVLANVID(vid int) Action {
	return &modVLANVIDAction{
		vid: vid,
	}
}

// A modVLANVIDAction is an Action which is used by ModVLANVID.
type modVLANVIDAction struct {
	vid int
}

// MarshalText implements Action.
func (a *modVLANVIDAction) MarshalText() ([]byte, error) {
	if !validVLANVID(a.vid) {
		return nil, errInvalidVLANVID
	}

	return bprintf(patModVLANVID, a.vid), nil
}

// GoString implements Action.
func (a *modVLANVIDAction) GoString() string {
	return fmt.Sprintf("ovs.ModVLANVID(%d)", a.vid)
}

// Output outputs the packet to the specified switch port.  Use
// InPortLocal to output the packet to the LOCAL port.  port must either
// be a non-negative integer.
func Output(port int) Action {
	return &outputAction{
		port: port,
	}
}

// An outputAction is an Action which is used by Output.
type outputAction struct {
	port int
}

// MarshalText implements Action.
func (a *outputAction) MarshalText() ([]byte, error) {
	if a.port < 0 {
		return nil, errOutputNegativePort
	}

	return bprintf(patOutput, a.port), nil
}

// GoString implements Action.
func (a *outputAction) GoString() string {
	return fmt.Sprintf("ovs.Output(%d)", a.port)
}

// OutputField outputs the packet to the switch port described by the specified field.
// For example, when the `field` value is "in_port", the packet is output to the port
// it came in on.
func OutputField(field string) Action {
	return &outputFieldAction{
		field: field,
	}
}

// An outputFieldAction is an Action which is used by OutputField.
type outputFieldAction struct {
	field string
}

// MarshalText implements Action.
func (a *outputFieldAction) MarshalText() ([]byte, error) {
	if a.field == "" {
		return nil, errOutputFieldEmpty
	}

	return bprintf(patOutputField, a.field), nil
}

// GoString implements Action.
func (a *outputFieldAction) GoString() string {
	return fmt.Sprintf("ovs.OutputField(%q)", a.field)
}

// Conjunction associates a flow with a certain conjunction ID to match on more than
// one dimension across multiple set matches.
func Conjunction(id int, dimensionNumber int, dimensionSize int) Action {
	return &conjunctionAction{
		id:              id,
		dimensionNumber: dimensionNumber,
		dimensionSize:   dimensionSize,
	}
}

// A conjuctionAction is an Action which is used by Conjunction.
type conjunctionAction struct {
	id              int
	dimensionNumber int
	dimensionSize   int
}

// MarshalText implements Action.
func (a *conjunctionAction) MarshalText() ([]byte, error) {
	if a.dimensionNumber > a.dimensionSize {
		return nil, errDimensionTooLarge
	}

	return bprintf(patConjunction, a.id, a.dimensionNumber, a.dimensionSize), nil
}

// GoString implements Action.
func (a *conjunctionAction) GoString() string {
	return fmt.Sprintf("ovs.Conjunction(%d, %d, %d)", a.id, a.dimensionNumber, a.dimensionSize)
}

// Resubmit resubmits a packet for further processing by matching
// flows with the specified port and table.  If port or table are zero,
// they are set to empty in the output Action.  If both are zero, an
// error is returned.
func Resubmit(port int, table int) Action {
	return &resubmitAction{
		port:  port,
		table: table,
	}
}

// A resubmitAction is an Action which is used by ConneectionTracking.
type resubmitAction struct {
	port  int
	table int
}

// ResubmitPort resubmits a packet into the current table with its context modified
// to look like it originated from the specified openflow port ID.
func ResubmitPort(port int) Action {
	return &resubmitPortAction{
		port: port,
	}
}

// A resubmitPortAction is an Action which is used by ConneectionTracking.
type resubmitPortAction struct {
	port int
}

// MarshalText implements Action.
func (a *resubmitPortAction) MarshalText() ([]byte, error) {
	// Largest valid port ID is 0xfffffeff per openflow spec.
	if a.port < 0 || a.port > 0xfffffeff {
		return nil, errResubmitPortInvalid
	}

	p := strconv.Itoa(a.port)

	return bprintf(patResubmitPort, p), nil
}

// GoString implements Action.
func (a *resubmitPortAction) GoString() string {
	return fmt.Sprintf("ovs.ResubmitPort(%d)", a.port)
}

// MarshalText implements Action.
func (a *resubmitAction) MarshalText() ([]byte, error) {
	if a.port == 0 && a.table == 0 {
		return nil, errResubmitPortTableZero
	}

	p := ""
	if a.port != 0 {
		p = strconv.Itoa(a.port)
	}

	t := ""
	if a.table != 0 {
		t = strconv.Itoa(a.table)
	}

	return bprintf(patResubmitPortTable, p, t), nil
}

// GoString implements Action.
func (a *resubmitAction) GoString() string {
	return fmt.Sprintf("ovs.Resubmit(%d, %d)", a.port, a.table)
}

// SetField overwrites the specified field with the specified value.
// If either string is empty, an error is returned.
func SetField(value string, field string) Action {
	return &loadSetFieldAction{
		value: value,
		field: field,
		typ:   actionSetField,
	}
}

// Load loads the specified value into the specified field.
// If either string is empty, an error is returned.
func Load(value string, field string) Action {
	return &loadSetFieldAction{
		value: value,
		field: field,
		typ:   actionLoad,
	}
}

// Specifies whether SetField or Load was called to construct a
// loadSetFieldAction.
const (
	actionSetField = iota
	actionLoad
)

// A loadSetFieldAction is an Action which is used by Load and SetField.
type loadSetFieldAction struct {
	value string
	field string
	typ   int
}

// MarshalText implements Action.
func (a *loadSetFieldAction) MarshalText() ([]byte, error) {
	if a.value == "" || a.field == "" {
		return nil, errLoadSetFieldZero
	}

	if a.typ == actionLoad {
		return bprintf("load:%s->%s", a.value, a.field), nil
	}

	return bprintf("set_field:%s->%s", a.value, a.field), nil
}

// GoString implements Action.
func (a *loadSetFieldAction) GoString() string {
	if a.typ == actionLoad {
		return fmt.Sprintf("ovs.Load(%q, %q)", a.value, a.field)
	}

	return fmt.Sprintf("ovs.SetField(%q, %q)", a.value, a.field)
}

// SetTunnel sets the tunnel id, e.g. VNI if vxlan is the tunnel protocol.
func SetTunnel(tunnelID uint64) Action {
	return &setTunnelAction{
		tunnelID: tunnelID,
	}
}

// A setTunnelAction is an Action used by SetTunnel.
type setTunnelAction struct {
	tunnelID uint64
}

// GoString implements Action.
func (a *setTunnelAction) GoString() string {
	return fmt.Sprintf("ovs.SetTunnel(%#x)", a.tunnelID)
}

// MarshalText implements Action.
func (a *setTunnelAction) MarshalText() ([]byte, error) {
	return bprintf("set_tunnel:%#x", a.tunnelID), nil
}

// Move sets the value of the destination field to the value of the source field.
func Move(src, dst string) Action {
	return &moveAction{
		src: src,
		dst: dst,
	}
}

// A moveAction is an Action used by Move.
type moveAction struct {
	src string
	dst string
}

// GoString implements Action.
func (a *moveAction) GoString() string {
	return fmt.Sprintf("ovs.Move(%q, %q)", a.src, a.dst)
}

// MarshalText implements Action.
func (a *moveAction) MarshalText() ([]byte, error) {
	if a.src == "" || a.dst == "" {
		return nil, errMoveEmpty
	}

	return bprintf("move:%s->%s", a.src, a.dst), nil
}

// Learn dynamically installs a LearnedFlow.
func Learn(learned *LearnedFlow) Action {
	return &learnAction{
		learned: learned,
	}
}

// A learnAction is an Action used by Learn.
type learnAction struct {
	learned *LearnedFlow
}

// GoString implements Action.
func (a *learnAction) GoString() string {
	return fmt.Sprintf("ovs.Learn(%#v)", a.learned)
}

// MarshalText implements Action.
func (a *learnAction) MarshalText() ([]byte, error) {
	if a.learned == nil {
		return nil, errLearnedNil
	}

	l, err := a.learned.MarshalText()
	if err != nil {
		return nil, err
	}

	return bprintf(patLearn, l), nil
}

// validARPOP indicates if an ARP OP is out of range. It should be in the range
// 1-4.
func validARPOP(op uint16) bool {
	return 1 <= op && op <= 4
}

// validIPv6Label indicates if an IPv6 label is out of range. It should only
// use the first 20 bits of the 32 bit field.
func validIPv6Label(label uint32) bool {
	return (label & 0xfff00000) == 0x00000000
}

// validVLANVID indicates if a VLAN VID falls within the valid range
// for a VLAN VID.
func validVLANVID(vid int) bool {
	return vid >= 0x000 && vid <= 0xfff
}

// validVLANVPCP indicates if a VLAN VID falls within the valid range
// for a VLAN VID.
func validVLANPCP(pcp int) bool {
	return pcp >= 0 && pcp <= 7
}
