package envsubst

import (
	"os"

	"github.com/hellt/envsubst/parse"
)

// String returns the parsed template string after processing it.
// If the parser encounters invalid input, it returns an error describing the failure.
func String(s string) (string, error) {
	return StringRestricted(s, false, false)
}

// StringRestricted returns the parsed template string after processing it.
// If the parser encounters invalid input, or a restriction is violated, it returns
// an error describing the failure.
// Errors on first failure or returns a collection of failures if failOnFirst is false
func StringRestricted(s string, noUnset, noEmpty bool) (string, error) {
	return StringRestrictedNoDigit(s, noUnset, noEmpty, false)
}

// Like StringRestricted but additionally allows to ignore env variables which start with a digit.
func StringRestrictedNoDigit(s string, noUnset, noEmpty bool, noDigit bool) (string, error) {
	return StringNoReplace(s, noUnset, noEmpty, noDigit, false)
}

// StringNoReplace is like StringRestrictedNoDigit but additionally allows to leave not found variables unreplaced
//
// e.g. P4s$w0rd123! to remain P4s$w0rd123!
// as opposed to P4s! IF no EnvVar is found to match
func StringNoReplace(s string, noUnset, noEmpty, noDigit, noReplace bool) (string, error) {
	return parse.New("string", os.Environ(),
		&parse.Restrictions{NoUnset: noUnset, NoEmpty: noEmpty, NoDigit: noDigit, NoReplace: noReplace}).Parse(s)
}

// Bytes returns the bytes represented by the parsed template after processing it.
// If the parser encounters invalid input, it returns an error describing the failure.
func Bytes(b []byte) ([]byte, error) {
	return BytesRestricted(b, false, false)
}

// BytesRestricted returns the bytes represented by the parsed template after processing it.
// If the parser encounters invalid input, or a restriction is violated, it returns
// an error describing the failure.
func BytesRestricted(b []byte, noUnset, noEmpty bool) ([]byte, error) {
	return BytesRestrictedNoDigit(b, noUnset, noEmpty, false)
}

// Like BytesRestricted but additionally allows to ignore env variables which start with a digit.
func BytesRestrictedNoDigit(b []byte, noUnset, noEmpty bool, noDigit bool) ([]byte, error) {
	return BytesRestrictedNoReplace(b, noUnset, noEmpty, noDigit, false)
}

// BytesRestrictedNoReplace is like BytesRestricted but additionally allows to leave not found variables unreplaced
//
// e.g. P4s$w0rd123! to remain P4s$w0rd123!
// as opposed to P4s! IF no EnvVar is found to match
func BytesRestrictedNoReplace(b []byte, noUnset, noEmpty, noDigit, noReplace bool) ([]byte, error) {
	s, err := parse.New("bytes", os.Environ(),
		&parse.Restrictions{NoUnset: noUnset, NoEmpty: noEmpty, NoDigit: noDigit, NoReplace: noReplace}).Parse(string(b))
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// ReadFile call io.ReadFile with the given file name.
// If the call to io.ReadFile failed it returns the error; otherwise it will
// call envsubst.Bytes with the returned content.
func ReadFile(filename string) ([]byte, error) {
	return ReadFileRestricted(filename, false, false)
}

// ReadFileRestricted calls io.ReadFile with the given file name.
// If the call to io.ReadFile failed it returns the error; otherwise it will
// call envsubst.Bytes with the returned content.
func ReadFileRestricted(filename string, noUnset, noEmpty bool) ([]byte, error) {
	return ReadFileRestrictedNoDigit(filename, noUnset, noEmpty, false)
}

// Like ReadFileRestricted but additionally allows to ignore env variables which start with a digit.
func ReadFileRestrictedNoDigit(filename string, noUnset, noEmpty bool, noDigit bool) ([]byte, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return BytesRestrictedNoDigit(b, noUnset, noEmpty, noDigit)
}

// Like BytesRestrictedNoDigit but additionally allows to leave not found variables unreplaced
//
// e.g. P4s$w0rd123! to remain P4s$w0rd123!
// as opposed to P4s! IF no EnvVar is found to match
func ReadFileRestrictedNoReplace(filename string, noUnset, noEmpty, noDigit, noReplace bool) ([]byte, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return BytesRestrictedNoReplace(b, noUnset, noEmpty, noDigit, noReplace)
}
