package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateIPv6ULASubnet creates a random /64 IPv6 subnet from the ULA (Unique Local Address) range
func GenerateIPv6ULASubnet() (string, error) {
	ula := "fd00:"
	for i := 0; i < 3; i++ {
		// Generate a random 16-bit hex field
		bytes := make([]byte, 2)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		ula += hex.EncodeToString(bytes) + ":"
	}
	ula += ":/64"
	return ula, nil
}
