package types

import "net/netip"

// IPAMProvider selects the management address allocator.
type IPAMProvider string

const (
	IPAMProviderContainerlab IPAMProvider = "containerlab"
	IPAMProviderRuntime      IPAMProvider = "runtime"
)

// MgmtIPAM configures management address allocation and duplicate detection.
type MgmtIPAM struct {
	Provider IPAMProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	DAD      *bool        `json:"dad,omitempty" yaml:"dad,omitempty"`
}

// DADEnabled reports whether duplicate address detection is enabled; nil defaults to true.
func (m MgmtIPAM) DADEnabled() bool {
	return m.DAD == nil || *m.DAD
}

// ExistingAddress is an address already assigned to a node in the management
// network, obtained from runtime inspection during apply.
type ExistingAddress struct {
	NodeName    string
	ContainerID string
	Address     netip.Addr
}

// NodeAddresses contains preferred allocation addresses, excluding explicit topology settings.
type NodeAddresses struct {
	IPv4 string `yaml:"ipv4,omitempty" json:"ipv4,omitempty"`
	IPv6 string `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
}

// AllocationOptions supplies live addresses, preferred addresses, and runtime reservations.
type AllocationOptions struct {
	Existing  []ExistingAddress
	Preferred map[string]NodeAddresses
	Reserved  []netip.Addr
}

// IPPrefixSet indexes occupied addresses and CIDRs. Traversal is bounded by
// address width (32 or 128 bits), independently of the number of stored prefixes.
// It is not safe for concurrent mutation.
type IPPrefixSet struct {
	roots [2]*ipPrefixNode
}

type ipPrefixNode struct {
	full     bool
	children [2]*ipPrefixNode
}

func ipFamily(ip netip.Addr) int {
	if ip.Is4() {
		return 0
	}
	return 1
}

func ipBit(ip netip.Addr, bit int) int {
	bytes := ip.As16()
	if ip.Is4() {
		bit += 96
	}
	return int((bytes[bit/8] >> uint(7-bit%8)) & 1)
}

func setIPBit(ip netip.Addr, bit, value int) netip.Addr {
	bytes := ip.As16()
	index := bit
	if ip.Is4() {
		index += 96
	}
	mask := byte(1 << uint(7-index%8))
	bytes[index/8] &= ^mask
	if value != 0 {
		bytes[index/8] |= mask
	}
	result := netip.AddrFrom16(bytes)
	if ip.Is4() {
		result = result.Unmap()
	}
	return result
}

// Add indexes a prefix, coalescing fully occupied sibling prefixes.
func (s *IPPrefixSet) Add(prefix netip.Prefix) {
	if !prefix.IsValid() || prefix.Addr().Is4In6() {
		return
	}
	root := &s.roots[ipFamily(prefix.Addr())]
	addIPPrefix(root, prefix.Masked(), 0)
}

func addIPPrefix(node **ipPrefixNode, prefix netip.Prefix, depth int) {
	if *node == nil {
		*node = &ipPrefixNode{}
	}
	n := *node
	if n.full {
		return
	}
	if depth == prefix.Bits() {
		n.full = true
		n.children = [2]*ipPrefixNode{}
		return
	}
	addIPPrefix(&n.children[ipBit(prefix.Addr(), depth)], prefix, depth+1)
	if n.children[0] != nil && n.children[0].full && n.children[1] != nil && n.children[1].full {
		n.full = true
		n.children = [2]*ipPrefixNode{}
	}
}

// Lookup returns an occupied prefix containing ip, if any.
func (s *IPPrefixSet) Lookup(ip netip.Addr) (netip.Prefix, bool) {
	if !ip.IsValid() || ip.Is4In6() {
		return netip.Prefix{}, false
	}
	n := s.roots[ipFamily(ip)]
	for depth := 0; n != nil; depth++ {
		if n.full {
			return netip.PrefixFrom(ip, depth).Masked(), true
		}
		if depth == ip.BitLen() {
			break
		}
		n = n.children[ipBit(ip, depth)]
	}
	return netip.Prefix{}, false
}

// NextFree returns the first unoccupied address at or after start in pool.
// Whole occupied subtrees are skipped without enumerating their addresses.
func (s *IPPrefixSet) NextFree(pool netip.Prefix, start netip.Addr) (netip.Addr, bool) {
	if !pool.IsValid() || !pool.Contains(start) {
		return netip.Addr{}, false
	}
	n := s.roots[ipFamily(start)]
	for depth := 0; depth < pool.Bits() && n != nil; depth++ {
		if n.full {
			return netip.Addr{}, false
		}
		n = n.children[ipBit(start, depth)]
	}
	return nextFreeIP(n, start, pool.Bits())
}

func nextFreeIP(n *ipPrefixNode, start netip.Addr, depth int) (netip.Addr, bool) {
	if n == nil {
		return start, true
	}
	if n.full || depth == start.BitLen() {
		return netip.Addr{}, false
	}
	bit := ipBit(start, depth)
	if ip, ok := nextFreeIP(n.children[bit], start, depth+1); ok {
		return ip, true
	}
	if bit == 1 {
		return netip.Addr{}, false
	}
	right := setIPBit(netip.PrefixFrom(start, depth).Masked().Addr(), depth, 1)
	return firstFreeIP(n.children[1], right, depth+1)
}

func firstFreeIP(n *ipPrefixNode, start netip.Addr, depth int) (netip.Addr, bool) {
	for n != nil {
		if n.full || depth == start.BitLen() {
			return netip.Addr{}, false
		}
		if n.children[0] == nil || !n.children[0].full {
			n = n.children[0]
		} else {
			start = setIPBit(start, depth, 1)
			n = n.children[1]
		}
		depth++
	}
	return start, true
}
