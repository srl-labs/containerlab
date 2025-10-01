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
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// PortLOCAL is a special in_port value which refers to the local port
	// of an OVS bridge.
	PortLOCAL = -1
)

// Possible errors which may be encountered while marshaling or unmarshaling
// a Flow.
var (
	errActionsWithDrop       = errors.New("Flow actions include drop, but multiple actions specified")
	errInvalidActions        = errors.New("invalid actions for Flow")
	errNoActions             = errors.New("no actions defined for Flow")
	errNotEnoughElements     = errors.New("not enough elements for valid Flow")
	errPriorityNotFirst      = errors.New("priority field is not first in Flow")
	errInvalidLearnedActions = errors.New("invalid actions for LearnedFlow")
)

// A Protocol is an OpenFlow protocol designation accepted by Open vSwitch.
type Protocol string

// Protocol constants which can be used in OVS flow configurations.
const (
	ProtocolARP    Protocol = "arp"
	ProtocolICMPv4 Protocol = "icmp"
	ProtocolICMPv6 Protocol = "icmp6"
	ProtocolIPv4   Protocol = "ip"
	ProtocolIPv6   Protocol = "ipv6"
	ProtocolTCPv4  Protocol = "tcp"
	ProtocolTCPv6  Protocol = "tcp6"
	ProtocolUDPv4  Protocol = "udp"
	ProtocolUDPv6  Protocol = "udp6"
)

// A Flow is an OpenFlow flow meant for adding flows to a software bridge.  It can be marshaled
// to and from its textual form for use with Open vSwitch.
type Flow struct {
	Priority    int
	Protocol    Protocol
	InPort      int
	Matches     []Match
	Table       int
	IdleTimeout int
	Cookie      uint64
	Actions     []Action
}

// A LearnedFlow is defined as part of the Learn action.
type LearnedFlow struct {
	Priority    int
	InPort      int
	Matches     []Match
	Table       int
	IdleTimeout int
	Cookie      uint64
	Actions     []Action

	DeleteLearned  bool
	FinHardTimeout int
	HardTimeout    int
	Limit          int
}

var _ error = &FlowError{}

// A FlowError is an error encountered while marshaling or unmarshaling
// a Flow.
type FlowError struct {
	// Str indicates the string, if any, that caused the flow to
	// fail while unmarshaling.
	Str string

	// Err indicates the error that halted flow marshaling or unmarshaling.
	Err error
}

// Error returns the string representation of a FlowError.
func (e *FlowError) Error() string {
	if e.Str == "" {
		return e.Err.Error()
	}

	return fmt.Sprintf("flow error due to string %q: %v",
		e.Str, e.Err)
}

// Constants and variables used repeatedly to reduce errors in code.
const (
	priorityString = "priority="

	priority    = "priority"
	cookie      = "cookie"
	keyActions  = "actions"
	idleTimeout = "idle_timeout"
	inPort      = "in_port"
	table       = "table"
	duration    = "duration"
	nPackets    = "n_packets"
	nBytes      = "n_bytes"
	hardAge     = "hard_age"
	idleAge     = "idle_age"

	// Variables used in LearnedFlows only.
	deleteLearned  = "delete_learned"
	finHardTimeout = "fin_hard_timeout"
	hardTimeout    = "hard_timeout"
	limit          = "limit"

	portLOCAL = "LOCAL"
)

var (
	priorityBytes = []byte(priorityString)
)

// MarshalText marshals a Flow into its textual form.
func (f *Flow) MarshalText() ([]byte, error) {
	if len(f.Actions) == 0 {
		return nil, &FlowError{
			Err: errNoActions,
		}
	}

	actions, err := marshalActions(f.Actions)
	if err != nil {
		return nil, err
	}

	matches, err := marshalMatches(f.Matches)
	if err != nil {
		return nil, err
	}

	// Action "drop" must only be specified by itself
	if len(actions) > 1 {
		for _, a := range actions {
			if a == actionDrop {
				return nil, &FlowError{
					Err: errActionsWithDrop,
				}
			}
		}
	}

	b := make([]byte, len(priorityBytes))
	copy(b, priorityBytes)

	b = strconv.AppendInt(b, int64(f.Priority), 10)

	if f.Protocol != "" {
		b = append(b, ',')
		b = append(b, f.Protocol...)
	}

	if f.InPort != 0 {
		b = append(b, ","+inPort+"="...)

		// Special case, InPortLOCAL is converted to the literal string LOCAL
		if f.InPort == PortLOCAL {
			b = append(b, portLOCAL...)
		} else {
			b = strconv.AppendInt(b, int64(f.InPort), 10)
		}
	}

	if len(matches) > 0 {
		b = append(b, ","+strings.Join(matches, ",")...)
	}

	b = append(b, ","+table+"="...)
	b = strconv.AppendInt(b, int64(f.Table), 10)

	b = append(b, ","+idleTimeout+"="...)
	b = strconv.AppendInt(b, int64(f.IdleTimeout), 10)

	if f.Cookie > 0 {
		// Hexadecimal cookies are much easier to read.
		b = append(b, ","+cookie+"="...)
		b = append(b, paddedHexUint64(f.Cookie)...)
	}

	b = append(b, ","+keyActions+"="+strings.Join(actions, ",")...)

	return b, nil
}

// MarshalText marshals a LearnedFlow into its textual form.
func (f *LearnedFlow) MarshalText() ([]byte, error) {
	if len(f.Actions) == 0 {
		return nil, &FlowError{
			Err: errNoActions,
		}
	}

	// A learned flow can have a limited set of actions, namely `load` and `output:field`.
	for _, a := range f.Actions {
		switch a.(type) {
		case *loadSetFieldAction:
			if a.(*loadSetFieldAction).typ != actionLoad {
				return nil, errInvalidLearnedActions
			}
		case *outputFieldAction:
		default:
			return nil, errInvalidLearnedActions
		}
	}

	actions, err := marshalActions(f.Actions)
	if err != nil {
		return nil, err
	}

	matches, err := marshalMatches(f.Matches)
	if err != nil {
		return nil, err
	}

	b := make([]byte, len(priorityBytes))
	copy(b, priorityBytes)

	b = strconv.AppendInt(b, int64(f.Priority), 10)

	if f.InPort != 0 {
		b = append(b, ","+inPort+"="...)

		// Special case, InPortLOCAL is converted to the literal string LOCAL
		if f.InPort == PortLOCAL {
			b = append(b, portLOCAL...)
		} else {
			b = strconv.AppendInt(b, int64(f.InPort), 10)
		}
	}

	if len(matches) > 0 {
		b = append(b, ","+strings.Join(matches, ",")...)
	}

	b = append(b, ","+table+"="...)
	b = strconv.AppendInt(b, int64(f.Table), 10)

	b = append(b, ","+idleTimeout+"="...)
	b = strconv.AppendInt(b, int64(f.IdleTimeout), 10)

	b = append(b, ","+finHardTimeout+"="...)
	b = strconv.AppendInt(b, int64(f.FinHardTimeout), 10)

	b = append(b, ","+hardTimeout+"="...)
	b = strconv.AppendInt(b, int64(f.HardTimeout), 10)

	if f.Limit > 0 {
		// On older version of OpenVSwitch, the limit option doesn't exist.
		// Make sure we don't use it if the value is not set.
		b = append(b, ","+limit+"="...)
		b = strconv.AppendInt(b, int64(f.Limit), 10)
	}

	if f.DeleteLearned {
		b = append(b, ","+deleteLearned...)
	}

	if f.Cookie > 0 {
		// Hexadecimal cookies are much easier to read.
		b = append(b, ","+cookie+"="...)
		b = append(b, paddedHexUint64(f.Cookie)...)
	}

	b = append(b, ","+strings.Join(actions, ",")...)

	return b, nil
}

// UnmarshalText unmarshals flow text into a Flow.
func (f *Flow) UnmarshalText(b []byte) error {
	// Make a copy per documentation for encoding.TextUnmarshaler.
	// A string is easier to work with in this case.
	s := string(b)

	// Must have one and only one actions=... field in the flow.
	ss := strings.Split(s, keyActions+"=")
	if len(ss) != 2 || ss[1] == "" {
		return &FlowError{
			Err: errNoActions,
		}
	}
	if len(ss) < 2 {
		return &FlowError{
			Err: errNotEnoughElements,
		}
	}
	matchers, actions := strings.TrimSpace(ss[0]), strings.TrimSpace(ss[1])

	// Handle matchers first.
	ss = strings.Split(matchers, ",")
	f.Matches = make([]Match, 0)
	for i := 0; i < len(ss); i++ {
		if !strings.Contains(ss[i], "=") {
			// that means this will be a protocol field.
			if ss[i] != "" {
				f.Protocol = Protocol(ss[i])
			}
			continue
		}

		// All remaining comma-separated values should be in key=value format,
		// but parsing "actions" is done later because actions can use the form:
		//  actions=foo,bar,baz
		kv := strings.Split(ss[i], "=")
		if len(kv) != 2 {
			continue
		}
		kv[1] = strings.TrimSpace(kv[1])

		switch strings.TrimSpace(kv[0]) {
		case priority:
			// Parse priority into struct field.
			pri, err := strconv.ParseInt(kv[1], 10, 0)
			if err != nil {
				return &FlowError{
					Str: kv[1],
					Err: err,
				}
			}
			f.Priority = int(pri)
			continue
		case cookie:
			// Parse cookie into struct field.
			cookie, err := strconv.ParseUint(kv[1], 0, 64)
			if err != nil {
				return &FlowError{
					Str: kv[1],
					Err: err,
				}
			}
			f.Cookie = cookie
			continue
		case keyActions:
			// Parse actions outside this loop.
			continue
		case inPort:
			// Parse in_port into struct field.
			s := kv[1]
			if strings.TrimSpace(s) == portLOCAL {
				f.InPort = PortLOCAL
				continue
			}

			port, err := strconv.ParseInt(s, 10, 0)
			if err != nil {
				return &FlowError{
					Str: s,
					Err: err,
				}
			}
			f.InPort = int(port)
			continue
		case idleTimeout:
			// Parse idle_timeout into struct field.
			timeout, err := strconv.ParseInt(kv[1], 10, 0)
			if err != nil {
				return &FlowError{
					Str: kv[1],
					Err: err,
				}
			}
			f.IdleTimeout = int(timeout)
			continue
		case table:
			// Parse table into struct field.
			table, err := strconv.ParseInt(kv[1], 10, 0)
			if err != nil {
				return &FlowError{
					Str: kv[1],
					Err: err,
				}
			}
			f.Table = int(table)
			continue
		case duration, nPackets, nBytes, hardAge, idleAge:
			// ignore those fields.
			continue
		}

		// All arbitrary key/value pairs that
		// don't match the case above.
		match, err := parseMatch(kv[0], kv[1])
		if err != nil {
			return err
		}
		// The keyword will be skipped if unknown,
		// don't add a nil value
		if match != nil {
			f.Matches = append(f.Matches, match)
		}
	}

	// Parse all actions from the flow.
	p := newActionParser(strings.NewReader(actions))
	out, raw, err := p.Parse()
	if err != nil {
		return &FlowError{
			Str: actions,
			Err: errInvalidActions,
		}
	}
	f.Actions = out

	// Action "drop" must only be specified by itself.
	if len(raw) > 1 {
		for _, a := range raw {
			if a == actionDrop {
				return &FlowError{
					Err: errActionsWithDrop,
				}
			}
		}
	}

	return nil
}

// MatchFlow converts Flow into MatchFlow.
func (f *Flow) MatchFlow() *MatchFlow {
	return &MatchFlow{
		Protocol: f.Protocol,
		InPort:   f.InPort,
		Matches:  f.Matches,
		Table:    f.Table,
		Cookie:   f.Cookie,
	}
}

// marshalActions marshals all provided Actions to their text form.
func marshalActions(aa []Action) ([]string, error) {
	fns := make([]func() ([]byte, error), 0, len(aa))
	for _, fn := range aa {
		fns = append(fns, fn.MarshalText)
	}

	return marshalFunctions(fns)
}

// marshalMatches marshals all provided Matches to their text form.
func marshalMatches(mm []Match) ([]string, error) {
	fns := make([]func() ([]byte, error), 0, len(mm))
	for _, fn := range mm {
		fns = append(fns, fn.MarshalText)
	}

	return marshalFunctions(fns)
}

// marshalFunctions marshals a slice of functions to their text form.
func marshalFunctions(fns []func() ([]byte, error)) ([]string, error) {
	out := make([]string, 0, len(fns))
	for _, fn := range fns {
		o, err := fn()
		if err != nil {
			return nil, err
		}

		out = append(out, string(o))
	}

	return out, nil
}

// paddedHexUint64 formats and displays a uint64 as a hexadecimal string
// prefixed with 0x and zero-padded up to 16 characters in length.
func paddedHexUint64(i uint64) string {
	return fmt.Sprintf("%#016x", i)
}
