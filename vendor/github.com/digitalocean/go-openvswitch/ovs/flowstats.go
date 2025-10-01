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
	"strconv"
	"strings"
)

var (
	// ErrInvalidFlowStats is returned when flow statistics from 'ovs-ofctl
	// dump-aggregate' do not match the expected output format.
	ErrInvalidFlowStats = errors.New("invalid flow statistics")
)

// FlowStats contains a variety of statistics about an Open vSwitch port,
// including its port ID and numbers about packet receive and transmit
// operations.
type FlowStats struct {
	PacketCount uint64
	ByteCount   uint64
}

// UnmarshalText unmarshals a FlowStats from textual form.
func (f *FlowStats) UnmarshalText(b []byte) error {
	// Make a copy per documentation for encoding.TextUnmarshaler.
	s := string(b)

	// Constants only needed within this method, to avoid polluting the
	// package namespace with generic names
	const (
		packetCount = "packet_count"
		byteCount   = "byte_count"
		flowCount   = "flow_count"
	)

	// Find the index of packet count to find stats.
	idx := strings.Index(s, packetCount)
	if idx == -1 {
		return ErrInvalidFlowStats
	}

	// Assume the last three fields are packets, bytes, and flows, in that order.
	ss := strings.Fields(s[idx:])
	fields := []string{
		packetCount,
		byteCount,
		flowCount,
	}

	if len(ss) != len(fields) {
		return ErrInvalidFlowStats
	}

	var values []uint64
	for i := range ss {
		// Split key from its integer value.
		kv := strings.Split(ss[i], "=")
		if len(kv) != 2 {
			return ErrInvalidFlowStats
		}

		// Verify keys appear in expected order.
		if kv[0] != fields[i] {
			return ErrInvalidFlowStats
		}

		n, err := strconv.ParseUint(kv[1], 10, 64)
		if err != nil {
			return err
		}

		values = append(values, n)
	}

	*f = FlowStats{
		PacketCount: values[0],
		ByteCount:   values[1],
		// Flow count unused.
	}

	return nil
}
