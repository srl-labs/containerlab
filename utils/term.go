package utils

import "golang.org/x/term"

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}
