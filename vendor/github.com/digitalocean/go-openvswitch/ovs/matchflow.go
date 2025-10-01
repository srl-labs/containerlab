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
	"strconv"
	"strings"
)

const (
	// AnyTable is a special table value to match flows in any table.
	AnyTable = -1
)

var (
	// errEmptyMatchFlow is returned when a MatchFlow has no arguments.
	errEmptyMatchFlow = errors.New("match flow is empty")
)

// TODO(mdlayher): it would be nice if MatchFlow was just a Flow.

// A MatchFlow is an OpenFlow flow intended for flow deletion.  It can be marshaled to its textual
// form for use with Open vSwitch.
type MatchFlow struct {
	Protocol Protocol
	InPort   int
	Matches  []Match
	Table    int

	// Cookie indicates a cookie value to use when matching flows.
	Cookie uint64

	// CookieMask is a mask used alongside Cookie to enable matching flows
	// which match a mask.  If CookieMask is not set, Cookie will be matched
	// exactly.
	CookieMask uint64
}

var _ error = &MatchFlowError{}

// A MatchFlowError is an error encountered while marshaling or unmarshaling
// a MatchFlow.
type MatchFlowError struct {
	// Str indicates the string, if any, that caused the flow to
	// fail while unmarshaling.
	Str string

	// Err indicates the error that halted flow marshaling or unmarshaling.
	Err error
}

// Error returns the string representation of a MatchFlowError.
func (e *MatchFlowError) Error() string {
	if e.Str == "" {
		return e.Err.Error()
	}

	return fmt.Sprintf("flow error due to string %q: %v",
		e.Str, e.Err)
}

// MarshalText marshals a MatchFlow into its textual form.
func (f *MatchFlow) MarshalText() ([]byte, error) {
	matches, err := f.marshalMatches()
	if err != nil {
		return nil, err
	}

	var b []byte

	if f.Protocol != "" {
		b = append(b, f.Protocol...)
		b = append(b, ',')
	}

	if f.InPort != 0 {
		b = append(b, inPort+"="...)

		// Special case, InPortLOCAL is converted to the literal string LOCAL
		if f.InPort == PortLOCAL {
			b = append(b, portLOCAL...)
		} else {
			b = strconv.AppendInt(b, int64(f.InPort), 10)
		}
		b = append(b, ',')
	}

	if len(matches) > 0 {
		b = append(b, strings.Join(matches, ",")...)
		b = append(b, ',')
	}

	if f.Cookie > 0 {
		// Hexadecimal cookies and masks are much easier to read.
		b = append(b, cookie+"="...)
		b = append(b, paddedHexUint64(f.Cookie)...)
		b = append(b, '/')

		if f.CookieMask == 0 {
			b = append(b, "-1"...)
		} else {
			b = append(b, paddedHexUint64(f.CookieMask)...)
		}

		b = append(b, ',')
	}

	if f.Table != AnyTable {
		b = append(b, table+"="...)
		b = strconv.AppendInt(b, int64(f.Table), 10)
	}

	b = bytes.Trim(b, ",")

	if len(b) == 0 {
		return nil, &MatchFlowError{
			Err: errEmptyMatchFlow,
		}
	}

	return b, nil
}

// marshalMatches marshals all Matches in a MatchFlow to their text form.
func (f *MatchFlow) marshalMatches() ([]string, error) {
	fns := make([]func() ([]byte, error), 0, len(f.Matches))
	for _, fn := range f.Matches {
		fns = append(fns, fn.MarshalText)
	}

	return f.marshalFunctions(fns)
}

// marshalFunctions marshals a slice of functions to their text form.
func (f *MatchFlow) marshalFunctions(fns []func() ([]byte, error)) ([]string, error) {
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
