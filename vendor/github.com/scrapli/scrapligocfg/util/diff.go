package util

import (
	"strings"

	"github.com/carlmontanari/difflibgo/difflibgo"
)

const (
	// DiffSubtraction is the symbol for lines removed in a diff output.
	DiffSubtraction = "- "
	// DiffAddition is the symbol for lines added in a diff output.
	DiffAddition = "+ "
	// DiffUnknown is the symbol for unknown lines in a diff output.
	DiffUnknown = "? "
)

// GetDiffLines returns a slice of diff lines (lines with diff symbols as outlined in constants) for
// the given strings.
func GetDiffLines(a, b string) []string {
	d := difflibgo.Differ{}

	return d.Compare(
		strings.Split(a, "\n"),
		strings.Split(b, "\n"),
	)
}
