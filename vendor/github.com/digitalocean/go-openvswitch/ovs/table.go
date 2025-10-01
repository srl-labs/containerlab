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
	// ErrInvalidTable is returned when tables from 'ovs-ofctl dump-tables'
	// do not match the expected output format.
	ErrInvalidTable = errors.New("invalid openflow table")
)

// A Table is an Open vSwitch table.
type Table struct {
	ID      int
	Name    string
	Wild    string
	Max     int
	Active  int
	Lookup  uint64
	Matched uint64
}

// UnmarshalText unmarshals a Table from textual form as output by
// 'ovs-ofctl dump-tables':
//  0: classifier: wild=0x3fffff, max=1000000, active=0
//                 lookup=0, matched=0
func (t *Table) UnmarshalText(b []byte) error {
	// Make a copy per documentation for encoding.TextUnmarshaler.
	s := string(b)

	ss := strings.Fields(s)
	if len(ss) != 7 && len(ss) != 8 {
		return ErrInvalidTable
	}

	// ID has trailing colon which must be removed
	id, err := strconv.ParseInt(strings.TrimSuffix(ss[0], ":"), 10, 0)
	if err != nil {
		return err
	}
	t.ID = int(id)

	// Numeric fields start at index 2 normally, but if the table name
	// does not contain a trailing colon (OVS tables that aren't "classifier"),
	// it will start at index 3
	idx := 2
	if !strings.HasSuffix(ss[1], ":") {
		idx = 3
	}
	// Name has trailing colon which must be removed
	t.Name = strings.TrimSuffix(ss[1], ":")

	out := make([]uint64, 0, 4)
	for i, str := range ss[idx:] {
		// Strip trailing commas from key/value fields
		str = strings.TrimSuffix(str, ",")

		pair := strings.Split(str, "=")
		if len(pair) != 2 {
			return ErrInvalidTable
		}

		if i == 0 {
			t.Wild = pair[1]
			continue
		}

		n, err := strconv.ParseUint(pair[1], 10, 64)
		if err != nil {
			return err
		}

		out = append(out, n)
	}

	t.Max = int(out[0])
	t.Active = int(out[1])
	t.Lookup = out[2]
	t.Matched = out[3]

	return nil
}
