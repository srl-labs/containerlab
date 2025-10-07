package utils

import (
	"strings"
	"unicode"
)

// StripNonPrintChars removes non-printable characters from the string.
func StripNonPrintChars(s string) string {
	return strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
}
