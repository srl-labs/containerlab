package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
)

// GenerateIPv6ULASubnet creates a random /64 ULA (Unique Local Address) IPv6 subnet in the fd00::/8
// range.
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

// GetRoutableAddresses returns a list of routable IPv4 and IPv6 addresses on the system.
// It excludes loopback, link-local, and other special-use addresses.
func GetRoutableAddresses() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var routableAddrs []string
	for _, addr := range addrs {
		// Parse the address
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}

		// Skip loopback addresses
		if ip.IsLoopback() {
			continue
		}

		// Skip link-local addresses
		if ip.IsLinkLocalUnicast() {
			continue
		}

		// Skip multicast and other special addresses
		if ip.IsMulticast() || ip.IsUnspecified() {
			continue
		}

		// For IPv4, skip private addresses like 127.x.x.x, 169.254.x.x
		if ip.To4() != nil {
			// Skip 169.254.x.x (link-local)
			if ip.To4()[0] == 169 && ip.To4()[1] == 254 {
				continue
			}
		}

		// For IPv6, skip unique local addresses (fc00::/7) if we want only global addresses
		// But include them for now as they might be routable within the network
		routableAddrs = append(routableAddrs, ip.String())
	}

	return routableAddrs, nil
}

// LastHostIPInSubnet returns the last host IP address in a subnet
// (excludes the broadcast address for IPv4)
func LastHostIPInSubnet(ipnet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)

	// Set all host bits to 1 to get the last IP
	for i := 0; i < len(ip); i++ {
		ip[i] |= ^ipnet.Mask[i]
	}

	// For IPv4, decrement by 1 to avoid broadcast address
	// For IPv6, the last address is usable (no broadcast)
	if ip.To4() != nil {
		// Decrement the IP by 1
		for i := len(ip) - 1; i >= 0; i-- {
			if ip[i] > 0 {
				ip[i]--
				break
			}
			ip[i] = 255
		}
	}

	return ip
}
