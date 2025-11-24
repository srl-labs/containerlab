package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/netip"
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

// HostBig takes a parent CIDR range and turns it into a host IP address with
// the given host number.
//
// For example, 10.3.0.0/16 with a host number of 2 gives 10.3.0.2.
//
// Copied from github.com/hairyhenderson/gomplate internal/cidr package.
func HostBig(base netip.Prefix, num *big.Int) (netip.Addr, error) {
	parentLen := base.Bits()
	addrLen := base.Addr().BitLen()

	hostLen := addrLen - parentLen

	maxHostNum := big.NewInt(int64(1))

	//nolint:gosec // G115 doesn't apply here
	maxHostNum.Lsh(maxHostNum, uint(hostLen))
	maxHostNum.Sub(maxHostNum, big.NewInt(1))

	num2 := big.NewInt(num.Int64())
	if num.Cmp(big.NewInt(0)) == -1 {
		num2.Neg(num)
		num2.Sub(num2, big.NewInt(int64(1)))
		num.Sub(maxHostNum, num2)
	}

	if num2.Cmp(maxHostNum) == 1 {
		return netip.Addr{}, fmt.Errorf(
			"prefix of %d does not accommodate a host numbered %d",
			parentLen,
			num,
		)
	}

	return insertNumIntoIP(base.Masked().Addr(), num, addrLen), nil
}

func ipToInt(ip netip.Addr) (*big.Int, int) {
	val := &big.Int{}
	val.SetBytes(ip.AsSlice())

	return val, ip.BitLen()
}

func intToIP(ipInt *big.Int, bits int) netip.Addr {
	ipBytes := ipInt.Bytes()
	ret := make([]byte, bits/8)
	// Pack our IP bytes into the end of the return array,
	// since big.Int.Bytes() removes front zero padding.
	for i := 1; i <= len(ipBytes); i++ {
		ret[len(ret)-i] = ipBytes[len(ipBytes)-i]
	}

	addr, ok := netip.AddrFromSlice(ret)
	if !ok {
		panic("invalid IP address")
	}

	return addr
}

func insertNumIntoIP(ip netip.Addr, bigNum *big.Int, prefixLen int) netip.Addr {
	ipInt, totalBits := ipToInt(ip)

	//nolint:gosec // G115 isn't relevant here
	bigNum.Lsh(bigNum, uint(totalBits-prefixLen))
	ipInt.Or(ipInt, bigNum)
	return intToIP(ipInt, totalBits)
}
