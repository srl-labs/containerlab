package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GenerateIPv6ULASubnet creates a random /64 ULA (Unique Local Address) IPv6 subnet in the fd00::/8 range.
func GenerateIPv6ULASubnet() (string, error) {
	var ula strings.Builder

	ula.WriteString("fd00:")

	bytes := make([]byte, 2)
	for i := 0; i < 3; i++ {
		// Generate a random 16-bit hex field
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}

		ula.WriteString(hex.EncodeToString(bytes))
		ula.WriteString(":")
	}

	ula.WriteString(":/64")

	return ula.String(), nil
}
