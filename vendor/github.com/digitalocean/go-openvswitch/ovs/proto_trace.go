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
	"regexp"
	"strings"
)

var (
	// ErrInvalidProtoTrace is returned when the output from
	// ovs-appctl ofproto/trace is in an unexpected format
	ErrInvalidProtoTrace = errors.New("invalid ofproto/trace output")

	datapathActionsRegexp = regexp.MustCompile(`Datapath actions: (.*)`)
	initialFlowRegexp     = regexp.MustCompile(`Flow: (.*)`)
	finalFlowRegexp       = regexp.MustCompile(`Final flow: (.*)`)
	megaFlowRegexp        = regexp.MustCompile(`Megaflow: (.*)`)
	traceStartRegexp      = regexp.MustCompile(`bridge\("(.*)"\)`)
	traceFlowRegexp       = regexp.MustCompile(` *[[:digit:]]+[.] ([[:alpha:]].*)`)
	traceActionRegexp     = regexp.MustCompile(` +([[:alpha:]].*)`)
	recircIDRegexp        = regexp.MustCompile(`recirc_id=`)
	recircRegexp          = regexp.MustCompile(`recirc\(`)
	ctCommentRegexp       = regexp.MustCompile(`^\s+->`)
	ctThawRegexp          = regexp.MustCompile(`thaw`)
	ctResumeFromRegexp    = regexp.MustCompile(`Resuming from table`)
	ctResumeWithRegexp    = regexp.MustCompile(`resume conntrack with`)
	tunNative             = regexp.MustCompile(`native tunnel`)

	pushVLANPattern = `push_vlan(vid=[0-9]+,pcp=[0-9]+)`
)

const (
	popvlan   = "popvlan"
	pushvlan  = "pushvlan"
	drop      = "drop"
	localPort = 65534
)

// DataPathActions is a text unmarshaler for data path actions in ofproto/trace output
type DataPathActions interface {
	encoding.TextUnmarshaler
}

// NewDataPathActions returns an implementation of DataPathActions
func NewDataPathActions(actions string) DataPathActions {
	return &dataPathActions{
		actions: actions,
	}
}

type dataPathActions struct {
	actions string
}

func (d *dataPathActions) UnmarshalText(b []byte) error {
	d.actions = string(b)
	return nil
}

// DataPathFlows represents the initial/final flows passed/returned from ofproto/trace
type DataPathFlows struct {
	Protocol Protocol
	Matches  []Match
}

// UnmarshalText unmarshals the initial/final data path flows from ofproto/trace output
func (df *DataPathFlows) UnmarshalText(b []byte) error {
	matches := strings.Split(string(b), ",")

	if len(matches) == 0 {
		return errors.New("error unmarshalling text, no comma delimiter found")
	}

	for _, match := range matches {
		switch Protocol(match) {
		case ProtocolARP, ProtocolICMPv4, ProtocolICMPv6,
			ProtocolIPv4, ProtocolIPv6, ProtocolTCPv4,
			ProtocolTCPv6, ProtocolUDPv4, ProtocolUDPv6:
			df.Protocol = Protocol(match)
			continue
		}

		// We can safely skip these keywords
		switch {
		case match == "eth":
			continue
		case match == "unchanged":
			continue
		case recircIDRegexp.MatchString(match):
			continue
		}

		kv := strings.Split(match, "=")
		if len(kv) != 2 {
			return fmt.Errorf("unexpected match format for match %q", match)
		}

		switch strings.TrimSpace(kv[0]) {
		case inPort:
			// Parse in_port=LOCAL into a new match.
			if strings.TrimSpace(kv[1]) == portLOCAL {
				df.Matches = append(df.Matches, InPortMatch(localPort))
				continue
			}
		}

		m, err := parseMatch(kv[0], kv[1])
		if err != nil {
			return err
		}
		// The keyword will be skipped if unknown,
		// don't add a nil value
		if m != nil {
			df.Matches = append(df.Matches, m)
		}
	}

	return nil
}

// ProtoTrace is a type representing output from ovs-app-ctl ofproto/trace
type ProtoTrace struct {
	CommandStr      string
	InputFlow       *DataPathFlows
	FinalFlow       *DataPathFlows
	DataPathActions DataPathActions
	FlowActions     []string
}

// UnmarshalText unmarshals ProtoTrace text into a ProtoTrace type.
// Not implemented yet.
func (pt *ProtoTrace) UnmarshalText(b []byte) error {
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if matches, matched := checkMatch(datapathActionsRegexp, line); matched {

			if recircRegexp.MatchString(line) {
				pt.FlowActions = append(pt.FlowActions, "recirc")
			}

			// first index is always the left most match, following
			// are the actual matches
			pt.DataPathActions = &dataPathActions{
				actions: matches[1],
			}

			continue
		}

		if matches, matched := checkMatch(initialFlowRegexp, line); matched {
			flow := &DataPathFlows{}
			err := flow.UnmarshalText([]byte(matches[1]))
			if err != nil {
				return err
			}

			pt.InputFlow = flow
			continue
		}

		if matches, matched := checkMatch(finalFlowRegexp, line); matched {
			flow := &DataPathFlows{}
			err := flow.UnmarshalText([]byte(matches[1]))
			if err != nil {
				return err
			}

			pt.FinalFlow = flow
			continue
		}

		if _, matched := checkMatch(megaFlowRegexp, line); matched {
			continue
		}

		if _, matched := checkMatch(traceStartRegexp, line); matched {
			continue
		}

		if _, matched := checkMatch(traceFlowRegexp, line); matched {
			continue
		}

		// We can safely skip these keywords
		switch {
		case ctCommentRegexp.MatchString(line):
			continue
		case ctThawRegexp.MatchString(line):
			continue
		case ctResumeFromRegexp.MatchString(line):
			continue
		case ctResumeWithRegexp.MatchString(line):
			continue
		case tunNative.MatchString(line):
			continue
		}

		if matches, matched := checkMatch(traceActionRegexp, line); matched {
			pt.FlowActions = append(pt.FlowActions, matches[1])
			continue
		}
	}

	return nil
}

func checkMatch(re *regexp.Regexp, s string) ([]string, bool) {
	matches := re.FindStringSubmatch(s)
	if len(matches) == 0 {
		return matches, false
	}

	return matches, true
}
