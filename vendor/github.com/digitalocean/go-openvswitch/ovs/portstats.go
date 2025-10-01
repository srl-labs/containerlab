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
	// ErrInvalidPortStats is returned when port statistics from 'ovs-ofctl
	// dump-ports' do not match the expected output format.
	ErrInvalidPortStats = errors.New("invalid port statistics")
)

// PortStats contains a variety of statistics about an Open vSwitch port,
// including its port ID and numbers about packet receive and transmit
// operations.
type PortStats struct {
	// PortID specifies the OVS port ID which this PortStats refers to.
	PortID int

	// Received and Transmitted contain information regarding the number
	// of received and transmitted packets, bytes, etc.
	// OVS stores all of these counters as uint64 values.
	Received    PortStatsReceive
	Transmitted PortStatsTransmit
}

// PortStatsReceive contains information regarding the number of received
// packets, bytes, etc.
type PortStatsReceive struct {
	Packets uint64
	Bytes   uint64
	Dropped uint64
	Errors  uint64
	Frame   uint64
	Over    uint64
	CRC     uint64
}

// PortStatsTransmit contains information regarding the number of transmitted
// packets, bytes, etc.
type PortStatsTransmit struct {
	Packets    uint64
	Bytes      uint64
	Dropped    uint64
	Errors     uint64
	Collisions uint64
}

// UnmarshalText unmarshals a PortStats from textual form as output by
// 'ovs-ofctl dump-ports':
//    port  1: rx pkts=0, bytes=0, drop=0, errs=0, frame=0, over=0, crc=0
//             tx pkts=0, bytes=0, drop=0, errs=0, coll=0
func (p *PortStats) UnmarshalText(b []byte) error {
	// Make a copy per documentation for encoding.TextUnmarshaler.
	s := string(b)

	// Constants only needed within this method, to avoid polluting the
	// package namespace with generic names
	const (
		port = "port"
		rx   = "rx"
		tx   = "tx"

		pkts   = "pkts"
		sbytes = "bytes"
		drop   = "drop"
		errs   = "errs"
		frame  = "frame"
		over   = "over"
		crc    = "crc"
		coll   = "coll"
	)

	// The output of 'ovs-ofctl dump-ports' should always be split into
	// 16 pieces.  If the output format changes, this function will
	// no longer work.
	ss := strings.Fields(s)
	if len(ss) != 16 {
		return ErrInvalidPortStats
	}

	// Check for sentinel values which always occur at fixed
	// indices.
	if ss[0] != port {
		return ErrInvalidPortStats
	}
	if ss[2] != rx {
		return ErrInvalidPortStats
	}
	if ss[10] != tx {
		return ErrInvalidPortStats
	}

	// Port ID has a trailing colon that needs to be removed, and may be
	// value LOCAL
	portID := strings.TrimSuffix(ss[1], ":")
	if portID == portLOCAL {
		p.PortID = PortLOCAL
	} else {
		id, err := strconv.ParseInt(portID, 10, 0)
		if err != nil {
			return err
		}
		p.PortID = int(id)
	}

	// Iterate all remaining strings to capture all output uint64
	// values for the PortStats struct.
	out := map[string]map[string]uint64{
		rx: make(map[string]uint64, 7),
		tx: make(map[string]uint64, 5),
	}
	var prefix string
	for _, str := range ss[2:] {
		str = strings.TrimSuffix(str, ",")

		// Assign output map entries based on prefix encountered
		switch str {
		case rx:
			prefix = rx
			continue
		case tx:
			prefix = tx
			continue
		}

		pair := strings.Split(str, "=")
		if len(pair) != 2 {
			return ErrInvalidPortStats
		}

		// When using tunnel interfaces, some counters have a value of '?'.
		// In this case, just return 0 for the specified statistic.
		if pair[1] == "?" {
			out[prefix][pair[0]] = 0
			continue
		}

		n, err := strconv.ParseUint(pair[1], 10, 64)
		if err != nil {
			return err
		}

		out[prefix][pair[0]] = n
	}

	// Store all output uint64 values in their proper locations.
	p.Received.Packets = out[rx][pkts]
	p.Received.Bytes = out[rx][sbytes]
	p.Received.Dropped = out[rx][drop]
	p.Received.Errors = out[rx][errs]
	p.Received.Frame = out[rx][frame]
	p.Received.Over = out[rx][over]
	p.Received.CRC = out[rx][crc]
	p.Transmitted.Packets = out[tx][pkts]
	p.Transmitted.Bytes = out[tx][sbytes]
	p.Transmitted.Dropped = out[tx][drop]
	p.Transmitted.Errors = out[tx][errs]
	p.Transmitted.Collisions = out[tx][coll]

	return nil
}
