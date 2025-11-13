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

// ShortID trims the supplied identifier to the first 12 characters, matching Docker's short ID format.
func ShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}

	return id
}
