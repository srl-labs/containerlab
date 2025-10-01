package util

import "strings"

// StringContainsAny checks if string s is in the string slice l, this function returns a bool
// indicating inclusion or not.
func StringContainsAny(s string, l []string) bool {
	for _, ss := range l {
		if strings.Contains(s, ss) {
			return true
		}
	}

	return false
}

// StringContainsAnySubStrs checks if a string s is in the string slice l and returns the first
// encountered substring. If no match is encountered an empty string is returned.
func StringContainsAnySubStrs(s string, l []string) string {
	for _, ss := range l {
		if strings.Contains(s, ss) {
			return ss
		}
	}

	return ""
}
