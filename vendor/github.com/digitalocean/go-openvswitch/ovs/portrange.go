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
	"math"
)

var (
	// ErrInvalidPortRange is returned when there's a port range that invalid.
	ErrInvalidPortRange = errors.New("invalid port range")
)

// An PortRange represents a range of ports expressed in 16 bit integers.  The start and
// end values of this range are inclusive.
type PortRange struct {
	Start uint16
	End   uint16
}

// A BitRange is a representation of a range of values from base value with a bitmask
// applied.
type BitRange struct {
	Value uint16
	Mask  uint16
}

// BitwiseMatch returns an array of BitRanges that represent the range of integers
// in the PortRange.
func (r *PortRange) BitwiseMatch() ([]BitRange, error) {
	if r.Start <= 0 || r.End <= 0 {
		return nil, ErrInvalidPortRange
	}
	if r.Start > r.End {
		return nil, ErrInvalidPortRange
	}

	if r.Start == r.End {
		return []BitRange{
			{Value: r.Start, Mask: 0xffff},
		}, nil
	}

	bitRanges := []BitRange{}

	// Find the largest window we can get on a binary boundary
	window := (r.End - r.Start) + 1
	bitLength := uint(math.Floor(math.Log2(float64(window))))

	rangeStart, rangeEnd := getRange(r.End, bitLength)

	// Decrement our mask until we fit inside the range we want from a binary boundary.
	for rangeEnd > r.End {
		bitLength--
		rangeStart, rangeEnd = getRange(r.End, bitLength)
	}

	current := BitRange{
		Value: rangeStart,
		Mask:  getMask(bitLength),
	}

	// The range we picked out was from the middle of our set, so we'll need to recurse on
	// the remaining values for anything less than or greater than the current
	// range.

	if r.Start != rangeStart {
		leftRemainder := PortRange{
			Start: r.Start,
			End:   rangeStart - 1,
		}

		leftRemainingBitRanges, err := leftRemainder.BitwiseMatch()
		if err != nil {
			return nil, err
		}

		bitRanges = append(bitRanges, leftRemainingBitRanges...)
	}

	// We append our current range here, so we're ordered properly.
	bitRanges = append(bitRanges, current)

	if r.End != rangeEnd {
		rightRemainder := PortRange{
			Start: rangeEnd + 1,
			End:   r.End,
		}

		rightRemainingBitRanges, err := rightRemainder.BitwiseMatch()
		if err != nil {
			return nil, err
		}

		bitRanges = append(bitRanges, rightRemainingBitRanges...)
	}

	return bitRanges, nil
}

func getMask(bitLength uint) uint16 {
	// All 1s for everything that doesn't change in the range
	return math.MaxUint16 ^ uint16((1<<bitLength)-1)
}

func getRange(end uint16, bitLength uint) (rangeStart uint16, rangeEnd uint16) {
	// Represents the upper bound of our range window (all 1s to binary boundary)
	rangeLength := uint16((1 << bitLength) - 1)

	// Zero out our mask, so we start at a binary boundary.
	rangeStart = end &^ rangeLength

	// Simply add the mask so we end at a binary boundary.
	rangeEnd = rangeStart + rangeLength

	return rangeStart, rangeEnd
}
