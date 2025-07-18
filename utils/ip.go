package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
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

// CIDRToDDN converts CIDR mask to a Dotted Decimal Notation
// ie CIDR: 24 -> DDN: 255.255.255.0
// The result is a string.
func CIDRToDDN(length int) string {
	// check mask length is valid
	if length < 0 || length > 32 {
		log.Errorf("Invalid prefix length: %d", length)
		return ""
	}

	mask := net.CIDRMask(length, 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}
