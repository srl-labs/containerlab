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
	"encoding"
	"errors"
	"fmt"
	"io"
)

var (
	// errMultipleValues is returned when a function should only retrieve a
	// single value, but multiple values are found instead.
	errMultipleValues = errors.New("multiple values returned")

	// errNotCommitted is returned when a flow bundle transaction is not
	// committed, and thus, the contents of the bundle are discarded.
	errNotCommitted = errors.New("flow bundle not committed, discarding flows")
)

// An OpenFlowService is used in a Client to execute 'ovs-ofctl' commands.
type OpenFlowService struct {
	// Wrapped Client for ExecFunc and debugging.
	c *Client
}

// AddFlow adds a Flow to a bridge attached to Open vSwitch.
func (o *OpenFlowService) AddFlow(bridge string, flow *Flow) error {
	fb, err := flow.MarshalText()
	if err != nil {
		return err
	}

	args := []string{"add-flow"}
	args = append(args, o.c.ofctlFlags...)
	args = append(args, []string{bridge, string(fb)}...)

	_, err = o.exec(args...)
	return err
}

// A FlowTransaction is a transaction used when adding or deleting
// multiple flows using an Open vSwitch flow bundle.
type FlowTransaction struct {
	flows     []flowDirective
	committed bool
	err       error
}

// A flowDirective is a directive and flow string pair, used to perform
// multiple operations within a Transaction.
type flowDirective struct {
	directive string
	flow      string
}

// Possible flowDirective directive values.
const (
	dirAdd    = "add"
	dirDelete = "delete"
)

// Add pushes zero or more Flows on to the transaction, to be added by
// Open vSwitch.  If any of the flows are invalid, Add becomes a no-op
// and the error will be surfaced when Commit is called.
func (tx *FlowTransaction) Add(flows ...*Flow) {
	if tx.err != nil {
		return
	}

	tms := make([]encoding.TextMarshaler, 0, len(flows))
	for _, f := range flows {
		tms = append(tms, f)
	}

	tx.push(dirAdd, tms...)
}

// Delete pushes zero or more MatchFlows on to the transaction, to be deleted
// by Open vSwitch.  If any of the flows are invalid, Delete becomes a no-op
// and the error will be surfaced when Commit is called.
func (tx *FlowTransaction) Delete(flows ...*MatchFlow) {
	if tx.err != nil {
		return
	}

	tms := make([]encoding.TextMarshaler, 0, len(flows))
	for _, f := range flows {
		tms = append(tms, f)
	}

	tx.push(dirDelete, tms...)
}

// push pushes zero or more encoding.TextMarshalers on to the transaction
// (typically a Flow or MatchFlow).
func (tx *FlowTransaction) push(directive string, flows ...encoding.TextMarshaler) {
	for _, f := range flows {
		fb, err := f.MarshalText()
		if err != nil {
			tx.err = err
			return
		}

		tx.flows = append(tx.flows, flowDirective{
			directive: directive,
			flow:      string(fb),
		})
	}
}

// Commit finalizes an AddFlowTransaction, returning any errors that may
// have occurred while adding flows.  Commit must be called at the end
// of a successful transaction, but may return an error if one was encountered
// during a call to Add.
func (tx *FlowTransaction) Commit() error {
	if tx.err != nil {
		return tx.err
	}

	tx.committed = true
	return nil
}

// Discard discards the contents of an AddFlowTransaction, returning an
// error wrapping the input error.  Discard should be called if any
// operations fail in the middle of an AddFlowTransaction function.
func (tx *FlowTransaction) Discard(err error) error {
	tx.flows = make([]flowDirective, 0)
	return fmt.Errorf("discarding add flow transaction: %v", err)
}

// AddFlowBundle creates an Open vSwitch flow bundle and enables adding and
// removing flows to and from the specified bridge using a FlowTransaction.
// This function enables atomic addition and deletion of flows to and from
// Open vSwitch.
func (o *OpenFlowService) AddFlowBundle(bridge string, fn func(tx *FlowTransaction) error) error {
	// Flows will be added to and read from an in-memory buffer.  The buffer's
	// contents are piped to 'ovs-ofctl' using stdin.
	buf := bytes.NewBuffer(nil)

	tx := &FlowTransaction{}
	if err := fn(tx); err != nil {
		// Errors from "tx.Commit()" or "tx.Discard()" will be returned here.
		return err
	}

	// Require an explicit "tx.Commit()" to make changes.
	if !tx.committed {
		return errNotCommitted
	}

	for _, flow := range tx.flows {
		// Syntax for adding a flow in the file is:
		// "add priority=10,ip,actions=drop\n"
		s := fmt.Sprintf("%s %s\n", flow.directive, flow.flow)
		if _, err := io.WriteString(buf, s); err != nil {
			return err
		}
	}

	args := []string{"--bundle", "add-flow"}
	args = append(args, o.c.ofctlFlags...)
	// Read from stdin.
	args = append(args, bridge, "-")

	return o.pipe(buf, args...)
}

// DelFlows removes flows that match MatchFlow from a bridge attached to Open vSwitch.
//
// If flow is nil, all flows will be deleted from the specified bridge.
func (o *OpenFlowService) DelFlows(bridge string, flow *MatchFlow) error {
	if flow == nil {
		// This means we'll flush the entire flows
		// from the specifided bridge.
		_, err := o.exec("del-flows", bridge)
		return err
	}
	fb, err := flow.MarshalText()
	if err != nil {
		return err
	}

	_, err = o.exec("del-flows", bridge, string(fb))
	return err
}

// ModPort modifies the specified characteristics for the specified port.
func (o *OpenFlowService) ModPort(bridge string, port string, action PortAction) error {
	_, err := o.exec("mod-port", bridge, string(port), string(action))
	return err
}

// DumpPort retrieves statistics about the specified port attached to the
// specified bridge.
func (o *OpenFlowService) DumpPort(bridge string, port string) (*PortStats, error) {
	stats, err := o.dumpPorts(bridge, port)
	if err != nil {
		return nil, err
	}
	if len(stats) != 1 {
		return nil, errMultipleValues
	}

	return stats[0], nil
}

// DumpPorts retrieves statistics about all ports attached to the specified
// bridge.
func (o *OpenFlowService) DumpPorts(bridge string) ([]*PortStats, error) {
	return o.dumpPorts(bridge, "")
}

// DumpTables retrieves statistics about all tables for the specified bridge.
// If a table has no active flows and has not been used for a lookup or matched
// by an incoming packet, it is filtered from the output.
func (o *OpenFlowService) DumpTables(bridge string) ([]*Table, error) {
	out, err := o.exec("dump-tables", bridge)
	if err != nil {
		return nil, err
	}

	var tables []*Table
	err = parseEach(out, dumpTablesPrefix, func(b []byte) error {
		t := new(Table)
		if err := t.UnmarshalText(b); err != nil {
			return err
		}

		// Ignore empty tables
		if t.Active == 0 && t.Lookup == 0 && t.Matched == 0 {
			return nil
		}

		tables = append(tables, t)
		return nil
	})

	return tables, err
}

// DumpFlows retrieves statistics about all flows for the specified bridge.
// If a table has no active flows and has not been used for a lookup or matched
// by an incoming packet, it is filtered from the output.
func (o *OpenFlowService) DumpFlows(bridge string) ([]*Flow, error) {
	out, err := o.exec("dump-flows", bridge)
	if err != nil {
		return nil, err
	}

	var flows []*Flow
	err = parseEachLine(out, dumpFlowsPrefix, func(b []byte) error {
		// Do not attempt to parse NXST_FLOW messages.
		if bytes.HasPrefix(b, dumpFlowsPrefix) {
			return nil
		}

		f := new(Flow)
		if err := f.UnmarshalText(b); err != nil {
			return err
		}

		flows = append(flows, f)
		return nil
	})

	return flows, err
}

// DumpAggregate retrieves statistics about the specified flow attached to the
// specified bridge.
func (o *OpenFlowService) DumpAggregate(bridge string, flow *MatchFlow) (*FlowStats, error) {
	stats, err := o.dumpAggregate(bridge, flow)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

var (
	// dumpPortsPrefix is a sentinel value returned at the beginning of
	// the output from 'ovs-ofctl dump-ports'.
	dumpPortsPrefix = []byte("OFPST_PORT reply")

	// dumpTablesPrefix is a sentinel value returned at the beginning of
	// the output from 'ovs-ofctl dump-tables'.
	dumpTablesPrefix = []byte("OFPST_TABLE reply")

	// dumpFlowsPrefix is a sentinel value returned at the beginning of
	// the output from 'ovs-ofctl dump-flows'.
	dumpFlowsPrefix = []byte("NXST_FLOW reply")

	// dumpAggregatePrefix is a sentinel value returned at the beginning of
	// the output from "ovs-ofctl dump-aggregate"
	dumpAggregatePrefix = []byte("NXST_AGGREGATE reply")
)

// dumpPorts calls 'ovs-ofctl dump-ports' with the specified arguments and
// parses the output into zero or more PortStats structs.
func (o *OpenFlowService) dumpPorts(bridge string, port string) ([]*PortStats, error) {
	args := []string{
		"dump-ports",
		bridge,
	}

	args = append(o.c.ofctlFlags, args...)

	// Attach port argument only if non-empty.
	if port != "" {
		args = append(args, string(port))
	}

	out, err := o.exec(args...)
	if err != nil {
		return nil, err
	}

	var stats []*PortStats
	err = parseEach(out, dumpPortsPrefix, func(b []byte) error {
		s := new(PortStats)
		if err := s.UnmarshalText(b); err != nil {
			return err
		}

		stats = append(stats, s)
		return nil
	})

	return stats, err
}

// dumpAggregate calls 'ovs-ofctl dump-aggregate' with the specified arguments and
// parses the output into zero or more FlowStat structs.
func (o *OpenFlowService) dumpAggregate(bridge string, flow *MatchFlow) (*FlowStats, error) {

	flowText, err := flow.MarshalText()
	if err != nil {
		return nil, err
	}

	args := []string{
		"dump-aggregate",
		bridge,
		string(flowText),
	}

	args = append(o.c.ofctlFlags, args...)

	out, err := o.exec(args...)
	if err != nil {
		return nil, err
	}

	var stats FlowStats
	if err := stats.UnmarshalText(out); err != nil {
		return nil, err
	}

	return &stats, err
}

// parseEachLine parses ovs-ofctl output from the input buffer, ensuring it has the
// specified prefix, and invoking the input function on each line scanned,
// so more complex structures can be parsed.
func parseEachLine(in []byte, prefix []byte, fn func(b []byte) error) error {
	// First line must not be empty
	scanner := bufio.NewScanner(bytes.NewReader(in))
	scanner.Split(bufio.ScanLines)
	if !scanner.Scan() {
		return io.ErrUnexpectedEOF
	}

	// First line must contain prefix returned by OVS
	if !bytes.HasPrefix(scanner.Bytes(), prefix) {
		return io.ErrUnexpectedEOF
	}

	// Scan every line to retrieve information needed to unmarshal
	// a single Flow struct.
	for scanner.Scan() {
		b := make([]byte, len(scanner.Bytes()))
		copy(b, scanner.Bytes())
		if err := fn(b); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// parseEach parses ovs-ofctl output from the input buffer, ensuring it has the
// specified prefix, and invoking the input function on each two lines scanned,
// so more complex structures can be parsed.
func parseEach(in []byte, prefix []byte, fn func(b []byte) error) error {
	// First line must not be empty
	scanner := bufio.NewScanner(bytes.NewReader(in))
	scanner.Split(bufio.ScanLines)
	if !scanner.Scan() {
		return io.ErrUnexpectedEOF
	}

	// First line must contain prefix returned by OVS
	if !bytes.HasPrefix(scanner.Bytes(), prefix) {
		return io.ErrUnexpectedEOF
	}

	// OVS with OpenFlow 1.3+ returns an additional line with more metadata
	// which must be ignored.  A banner appears containing "(OF1.x)" which we
	// detect here to discover if the last line should be discarded.
	hasDuration := bytes.Contains(scanner.Bytes(), []byte("(OF1."))
	// Detect if CUSTOM Statistics is present, to skip additional lines
	hasCustomStats := bytes.Contains(in, []byte("CUSTOM"))

	// Scan every two lines to retrieve information needed to unmarshal
	// a single PortStats struct.
	for scanner.Scan() {
		b := make([]byte, len(scanner.Bytes()))
		copy(b, scanner.Bytes())

		// Must always scan two lines
		if !scanner.Scan() {
			return io.ErrUnexpectedEOF
		}
		b = append(b, scanner.Bytes()...)

		if hasDuration {
			// Discard the third line of information if applicable.
			if !scanner.Scan() {
				return io.ErrUnexpectedEOF
			}
			//Discard 4th & 5th lines if Custom stats are present
			if hasCustomStats {
				if !scanner.Scan() {
					return io.ErrUnexpectedEOF
				}
				if !scanner.Scan() {
					return io.ErrUnexpectedEOF
				}
			}
		}

		if err := fn(b); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// exec executes an ExecFunc using 'ovs-ofctl'.
func (o *OpenFlowService) exec(args ...string) ([]byte, error) {
	return o.c.exec("ovs-ofctl", args...)
}

// pipe executes a PipeFunc using 'ovs-ofctl'.
func (o *OpenFlowService) pipe(stdin io.Reader, args ...string) error {
	return o.c.pipe(stdin, "ovs-ofctl", args...)
}
