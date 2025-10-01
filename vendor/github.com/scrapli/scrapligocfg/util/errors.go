package util

import "errors"

var (
	// ErrIgnoredOption is an error for ignored options, typically this error can/should be ignored.
	ErrIgnoredOption = errors.New("errIgnoredOption")
	// ErrUnsupportedPlatform is an error returned when requesting an unsupported platform type.
	ErrUnsupportedPlatform = errors.New("errUnsupportedPlatform")
	// ErrPrepareError is an error for any issues encountered during the "prepare" phase.
	ErrPrepareError = errors.New("errPrepareError")
	// ErrConnectionError is an error for generic connection errors with the underlying scrapligo
	// Conn object.
	ErrConnectionError = errors.New("errConnectionError")
	// ErrCandidateError is an error for any issues related to loading candidate configurations.
	ErrCandidateError = errors.New("errCandidateError")
	// ErrBadSource is an error returned when an invalid source/target datastore is requested.
	ErrBadSource = errors.New("errBadSource")
	// ErrFilesystemError is an error returned for any filesystem related issues, including
	// insufficient space to load a candidate configuration.
	ErrFilesystemError = errors.New("errFilesystemError")
)
