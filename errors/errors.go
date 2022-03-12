package errors

import "errors"

var (
	// ErrFileNotFound is returned when a file is not found
	ErrFileNotFound = errors.New("file not found")
)
