package utils

import (
	"strings"
)

func SanitiseInterfaceName(ifaceName string) string {
	var sb strings.Builder  // Efficient way to build strings
	sb.Grow(len(ifaceName)) // Allocate enough memory to avoid reallocation

	for _, char := range ifaceName {
		switch char {
		case '/', ' ':
			sb.WriteRune('-') // Replace with '-'
		default:
			sb.WriteRune(char) // Keep the original character
		}
	}

	return sb.String()
}
