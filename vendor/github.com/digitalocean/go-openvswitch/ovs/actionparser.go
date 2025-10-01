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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// An actionParser is a parser for OVS flow actions.
type actionParser struct {
	r *bufio.Reader
	s stack
}

// newActionParser creates a new actionParser which wraps the input
// io.Reader.
func newActionParser(r io.Reader) *actionParser {
	return &actionParser{
		r: bufio.NewReader(r),
		s: make(stack, 0),
	}
}

// eof is a sentinel rune for end of file.
var eof = rune(0)

// read reads a single rune from the wrapped io.Reader.  It returns eof
// if no more runes are present.
func (p *actionParser) read() rune {
	ch, _, err := p.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// Parse parses a slice of Actions using the wrapped io.Reader.  The raw
// action strings are also returned for inspection if needed.
func (p *actionParser) Parse() ([]Action, []string, error) {
	var actions []Action
	var raw []string

	for {
		a, r, err := p.parseAction()
		if err != nil {
			// No more actions remain
			if err == io.EOF {
				break
			}

			return nil, nil, err
		}

		actions = append(actions, a)
		raw = append(raw, r)
	}

	return actions, raw, nil
}

// parseAction parses a single Action and its raw text from the wrapped
// io.Reader.
func (p *actionParser) parseAction() (Action, string, error) {
	// Track runes encountered
	var buf bytes.Buffer

	for {
		ch := p.read()

		// If comma encountered and no open parentheses, at end of this
		// action string
		if ch == ',' && p.s.len() == 0 {
			break
		}

		// If EOF encountered, at end of string
		if ch == eof {
			// If no items in buffer, end of this action
			if buf.Len() == 0 {
				return nil, "", io.EOF
			}

			// Parse action from buffer
			break
		}

		// Track open and closing parentheses using a stack to ensure
		// that they are appropriately matched
		switch ch {
		case '(':
			p.s.push()
		case ')':
			p.s.pop()
		}

		_, _ = buf.WriteRune(ch)
	}

	// Found an unmatched set of parentheses
	if p.s.len() > 0 {
		return nil, "", fmt.Errorf("invalid action: %q", buf.String())
	}

	s := buf.String()
	act, err := parseAction(s)
	return act, s, err
}

// A stack is a basic stack with elements that have no value.
type stack []struct{}

// len returns the current length of the stack.
func (s *stack) len() int {
	return len(*s)
}

// push adds an element to the stack.
func (s *stack) push() {
	*s = append(*s, struct{}{})
}

// pop removes an element from the stack.
func (s *stack) pop() {
	*s = (*s)[:s.len()-1]
}

var (
	// resubmitRe is the regex used to match the resubmit action
	// with port and table specified
	resubmitRe = regexp.MustCompile(`resubmit\((\d*),(\d*)\)`)

	// resubmitPortRe is the regex used to match the resubmit action
	// when only a port is specified
	resubmitPortRe = regexp.MustCompile(`resubmit:(\d+)`)

	// ctRe is the regex used to match the ct action with its
	// parameter list.
	ctRe = regexp.MustCompile(`ct\((\S+)\)`)

	// loadRe is the regex used to match the load action
	// with its parameters.
	loadRe = regexp.MustCompile(`load:(\S+)->(\S+)`)

	// setFieldRe is the regex used to match the set_field action
	// with its parameters.
	setFieldRe = regexp.MustCompile(`set_field:(\S+)->(\S+)`)
)

// TODO(mdlayher): replace parsing regex with arguments parsers

// parseAction creates an Action function from the input string.
func parseAction(s string) (Action, error) {
	// Simple actions which match a basic string
	switch strings.ToLower(s) {
	case actionDrop:
		return Drop(), nil
	case actionFlood:
		return Flood(), nil
	case actionInPort:
		return InPort(), nil
	case actionLocal:
		return Local(), nil
	case actionNormal:
		return Normal(), nil
	case actionStripVLAN:
		return StripVLAN(), nil
	}

	// ActionCT, with its arguments
	if ss := ctRe.FindAllStringSubmatch(s, 1); len(ss) > 0 && len(ss[0]) == 2 {
		// Results are:
		//  - full string
		//  - arguments list
		return ConnectionTracking(ss[0][1]), nil
	}

	// ActionModDataLinkDestination, with its hardware address.
	if strings.HasPrefix(s, patModDataLinkDestination[:len(patModDataLinkDestination)-2]) {
		var addr string
		n, err := fmt.Sscanf(s, patModDataLinkDestination, &addr)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			mac, err := net.ParseMAC(addr)
			if err != nil {
				return nil, err
			}

			return ModDataLinkDestination(mac), nil
		}
	}

	// ActionModDataLinkSource, with its hardware address.
	if strings.HasPrefix(s, patModDataLinkSource[:len(patModDataLinkSource)-2]) {
		var addr string
		n, err := fmt.Sscanf(s, patModDataLinkSource, &addr)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			mac, err := net.ParseMAC(addr)
			if err != nil {
				return nil, err
			}

			return ModDataLinkSource(mac), nil
		}
	}

	// ActionModNetworkDestination, with it hardware address
	if strings.HasPrefix(s, patModNetworkDestination[:len(patModNetworkDestination)-2]) {
		var ip string
		n, err := fmt.Sscanf(s, patModNetworkDestination, &ip)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			ip4 := net.ParseIP(ip).To4()
			if ip4 == nil {
				return nil, fmt.Errorf("invalid IPv4 address: %s", ip)
			}

			return ModNetworkDestination(ip4), nil
		}
	}

	// ActionModNetworkSource, with it hardware address
	if strings.HasPrefix(s, patModNetworkSource[:len(patModNetworkSource)-2]) {
		var ip string
		n, err := fmt.Sscanf(s, patModNetworkSource, &ip)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			ip4 := net.ParseIP(ip).To4()
			if ip4 == nil {
				return nil, fmt.Errorf("invalid IPv4 address: %s", ip)
			}

			return ModNetworkSource(ip4), nil
		}
	}

	// ActionModTransportDestinationPort, with its port.
	if strings.HasPrefix(s, patModTransportDestinationPort[:len(patModTransportDestinationPort)-2]) {
		var port uint16
		n, err := fmt.Sscanf(s, patModTransportDestinationPort, &port)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return ModTransportDestinationPort(port), nil
		}
	}

	// ActionModTransportSourcePort, with its port.
	if strings.HasPrefix(s, patModTransportSourcePort[:len(patModTransportSourcePort)-2]) {
		var port uint16
		n, err := fmt.Sscanf(s, patModTransportSourcePort, &port)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return ModTransportSourcePort(port), nil
		}
	}

	// ActionModVLANVID, with its VLAN ID
	if strings.HasPrefix(s, patModVLANVID[:len(patModVLANVID)-2]) {
		var vlan int
		n, err := fmt.Sscanf(s, patModVLANVID, &vlan)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return ModVLANVID(vlan), nil
		}
	}

	// ActionConjunction, with it's id, dimension number, and dimension size
	if strings.HasPrefix(s, patConjunction[:len(patConjunction)-10]) {
		var id, dimensionNumber, dimensionSize int
		n, err := fmt.Sscanf(s, patConjunction, &id, &dimensionNumber, &dimensionSize)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return Conjunction(id, dimensionNumber, dimensionSize), nil
		}
	}

	// ActionOutput, with its port number
	if strings.HasPrefix(s, patOutput[:len(patOutput)-2]) {
		var port int
		n, err := fmt.Sscanf(s, patOutput, &port)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return Output(port), nil
		}
	}

	// ActionResubmit, with both port number and table number
	if ss := resubmitRe.FindAllStringSubmatch(s, 1); len(ss) > 0 && len(ss[0]) == 3 {
		var (
			port  int
			table int

			err error
		)

		// Results are:
		//  - full string
		//  - port in parenthesis
		//  - table in parenthesis

		if s := ss[0][1]; s != "" {
			port, err = strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
		}
		if s := ss[0][2]; s != "" {
			table, err = strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
		}

		return Resubmit(port, table), nil
	}

	// ActionResubmitPort, with only a port number
	if ss := resubmitPortRe.FindAllStringSubmatch(s, 1); len(ss) > 0 && len(ss[0]) == 2 {
		port, err := strconv.Atoi(ss[0][1])
		if err != nil {
			return nil, err
		}

		return ResubmitPort(port), nil
	}

	if ss := loadRe.FindAllStringSubmatch(s, 2); len(ss) > 0 && len(ss[0]) == 3 {
		// Results are:
		//  - full string
		//  - value
		//  - field
		return Load(ss[0][1], ss[0][2]), nil
	}

	if ss := setFieldRe.FindAllStringSubmatch(s, 2); len(ss) > 0 && len(ss[0]) == 3 {
		// Results are:
		//  - full string
		//  - value
		//  - field
		return SetField(ss[0][1], ss[0][2]), nil
	}

	return nil, fmt.Errorf("no action matched for %q", s)
}
