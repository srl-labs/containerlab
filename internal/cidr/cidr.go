package cidr

import (
	"fmt"
	"math/big"
	"net/netip"
)

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
