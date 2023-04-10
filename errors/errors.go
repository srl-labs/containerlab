package errors

import "errors"

// ErrFileNotFound is returned when a file is not found.
var ErrFileNotFound = errors.New("file not found")

// ErrIncorrectInput is returned when the user input is incorrect.
var ErrIncorrectInput = errors.New("incorrect input")
