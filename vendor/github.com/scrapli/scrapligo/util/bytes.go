package util

import (
	"bytes"
	"regexp"
	"sync"
)

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?" +
	"\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var (
	ansiPattern     *regexp.Regexp //nolint: gochecknoglobals
	ansiPatternOnce sync.Once      //nolint: gochecknoglobals
)

func getAnsiPattern() *regexp.Regexp {
	ansiPatternOnce.Do(func() {
		ansiPattern = regexp.MustCompile(ansi)
	})

	return ansiPattern
}

// StripANSI removes ANSI escape codes from the given byte slice b.
func StripANSI(b []byte) []byte {
	return getAnsiPattern().ReplaceAll(b, []byte{})
}

// ByteIsAny checks if byte b is contained in byte slice l.
func ByteIsAny(b byte, l []byte) bool {
	for _, ss := range l {
		if b == ss {
			return true
		}
	}

	return false
}

// ByteContainsAny checks if byte slice b is contained in any byte slice in the slice of byte
// slices l.
func ByteContainsAny(b []byte, l [][]byte) bool {
	for _, ss := range l {
		if bytes.Contains(b, ss) {
			return true
		}
	}

	return false
}

// BytesRoughlyContains returns true if all bytes from the given byte slice `input` exist in the
// given `output` byte slice -- the elements must be found in order. This is basically the same as
// what you can see in @lithammer's(1) fuzzysearch `Match` function (thank you to them!) but
// converted to work for bytes and to not use a continuation block. Some examples:
//
// input 'aa', output 'b' = false
// input 'aa', output 'bba' = false
// input 'aa', output 'bbaa' = true
// input 'aba', output 'bba' = false
//
// In the context of scrapligo this is basically used for "fuzzy" matching our inputs. This lets us
// cope with devices that do things like the following srlinux banner entry output:
//
//		```
//	 --{ !* candidate shared default }--[  ]--
//	 A:srl# system banner login-banner "
//	 ...my banner
//	 ...has
//	 ...some lines
//	 ...that are neat
//	 ..."
//	 --{ !* candidate shared default }--[  ]--
//
// The "..." at the beginning of each line would historically be problematic for scrapli because in
// a very brute force/ham-fisted way we would demand to read back exactly what we sent to the device
// in the output -- so the "..." broke that. Not cool! This can be used to ensure that doesn't
// happen!
//
// Note: @lithammer's fuzzy search `Match` function here:
// https://github.com/lithammer/fuzzysearch/blob/ \
// b1f37a8c2080703d9fbd3e8989b2855c149a09e4/fuzzy/fuzzy.go#L60-L83.
func BytesRoughlyContains(input, output []byte) bool {
	if bytes.Contains(output, input) {
		// contains check is faster and simpler, if we contain the exact input then we are done
		return true
	}

	if len(output) < len(input) {
		// no point in doing a relatively costly check if there isn't enough chars to check against
		return false
	}

	for _, inputChar := range input {
		var shouldContinue bool

		shouldContinue, output = bytesRoughlyContainsIterOutputForInputChar(inputChar, output)

		if shouldContinue {
			continue
		}

		return false
	}

	return true
}

func bytesRoughlyContainsIterOutputForInputChar(
	inputChar byte,
	output []byte,
) (shouldContinue bool, newOutput []byte) {
	for idx, outputChar := range output {
		if inputChar == outputChar {
			return true, output[idx+1:]
		}
	}

	return false, output
}
