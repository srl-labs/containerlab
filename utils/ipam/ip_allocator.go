package ipam

import (
	"context"
	"fmt"
	"net/netip"
)

// ipPrefixSet indexes occupied addresses and CIDRs for one address family.
// Traversal is bounded by address width, independently of the number of stored prefixes.
type ipPrefixSet struct {
	root *ipPrefixNode
}

type ipPrefixNode struct {
	full     bool
	children [2]*ipPrefixNode
}

// IPAllocator assigns addresses from one subnet. It is not safe for concurrent use.
type IPAllocator struct {
	subnet netip.Prefix
	pool   netip.Prefix
	used   ipPrefixSet
}

// CanonicalPrefix normalizes a prefix while preserving invalid input.
func CanonicalPrefix(value string) string {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String()
	}
	return value
}

// CanonicalIP normalizes an address with or without a prefix while preserving invalid input.
func CanonicalIP(value string) string {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().String()
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return address.String()
	}
	return value
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
func (s *ipPrefixSet) add(prefix netip.Prefix) {
	if !prefix.IsValid() || prefix.Addr().Is4In6() {
		return
	}
	addIPPrefix(&s.root, prefix.Masked(), 0)
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
func (s *ipPrefixSet) lookup(ip netip.Addr) (netip.Prefix, bool) {
	if !ip.IsValid() || ip.Is4In6() {
		return netip.Prefix{}, false
	}
	n := s.root
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
func (s *ipPrefixSet) nextFree(pool netip.Prefix, start netip.Addr) (netip.Addr, bool) {
	if !pool.IsValid() || !pool.Contains(start) {
		return netip.Addr{}, false
	}
	n := s.root
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

// NewIPAllocator creates an allocator, excluding subnet boundary addresses.
// Reserved addresses (for example gateways) may be outside the allocation pool.
func NewIPAllocator(subnet, pool netip.Prefix, reserved []netip.Addr) (*IPAllocator, error) {
	if !isValidAllocationPool(subnet, pool) {
		return nil, fmt.Errorf("allocation pool %s must be contained in subnet %s", pool, subnet)
	}
	a := &IPAllocator{
		subnet: subnet.Masked(),
		pool:   pool.Masked(),
	}
	a.used.add(netip.PrefixFrom(a.subnet.Addr(), a.subnet.Addr().BitLen()))
	if subnet.Addr().Is4() {
		last := a.subnet.Addr().As4()
		for bit := subnet.Bits(); bit < 32; bit++ {
			last[bit/8] |= 1 << uint(7-bit%8)
		}
		a.used.add(netip.PrefixFrom(netip.AddrFrom4(last), 32))
	}
	for _, ip := range reserved {
		if !subnet.Contains(ip) {
			return nil, fmt.Errorf("reserved address %s is outside %s", ip, subnet)
		}
		a.used.add(netip.PrefixFrom(ip, ip.BitLen()))
	}
	return a, nil
}

func dadClient(clients []*DADClient) (*DADClient, error) {
	if len(clients) == 0 {
		return nil, nil
	}
	if len(clients) != 1 || clients[0] == nil {
		return nil, fmt.Errorf("expected at most one DAD client")
	}
	return clients[0], nil
}

func isValidAllocationPool(subnet, pool netip.Prefix) bool {
	if !subnet.IsValid() || !pool.IsValid() {
		return false
	}
	if subnet.Addr().Is4In6() || pool.Addr().Is4In6() {
		return false
	}
	if subnet.Addr().BitLen() != pool.Addr().BitLen() {
		return false
	}
	if pool.Bits() < subnet.Bits() {
		return false
	}
	return subnet.Contains(pool.Masked().Addr())
}

// Reserve excludes an explicit address.
func (a *IPAllocator) Reserve(ip netip.Addr) error {
	if !ip.IsValid() || ip.Is4In6() {
		return fmt.Errorf("invalid address %s", ip)
	}
	if !a.subnet.Contains(ip) {
		return fmt.Errorf("address %s is outside subnet %s", ip, a.subnet)
	}
	if _, occupied := a.used.lookup(ip); occupied {
		return fmt.Errorf("address %s is already reserved in %s", ip, a.subnet)
	}
	a.used.add(netip.PrefixFrom(ip, ip.BitLen()))
	return nil
}

// Next reserves and returns the first free address accepted by duplicate detection.
func (a *IPAllocator) Next(ctx context.Context, clients ...*DADClient) (netip.Addr, error) {
	dad, err := dadClient(clients)
	if err != nil {
		return netip.Addr{}, err
	}
	addresses, err := a.nextBatch(ctx, 1, dad)
	if err != nil {
		return netip.Addr{}, err
	}
	return addresses[0], nil
}

// NextPreferredBatch reserves available preferred addresses and replaces unavailable preferences
// from the pool.
func (a *IPAllocator) NextPreferredBatch(
	ctx context.Context,
	preferred []netip.Addr,
	clients ...*DADClient,
) ([]netip.Addr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dad, err := dadClient(clients)
	if err != nil {
		return nil, err
	}

	addresses := make([]netip.Addr, len(preferred))
	candidates := make([]netip.Addr, 0, len(preferred))
	candidateIndexes := make([]int, 0, len(preferred))
	for i, address := range preferred {
		if !a.pool.Contains(address) {
			continue
		}
		if _, occupied := a.used.lookup(address); occupied {
			continue
		}
		a.used.add(netip.PrefixFrom(address, address.BitLen()))
		candidates = append(candidates, address)
		candidateIndexes = append(candidateIndexes, i)
	}

	available := make([]bool, len(candidates))
	if dad == nil {
		for i := range available {
			available[i] = true
		}
	} else {
		available, err = dad.ProbeBatch(ctx, candidates)
		if err != nil {
			return nil, err
		}
	}
	for i, ok := range available {
		if ok {
			addresses[candidateIndexes[i]] = candidates[i]
		}
	}

	missing := make([]int, 0, len(addresses))
	for i, address := range addresses {
		if !address.IsValid() {
			missing = append(missing, i)
		}
	}
	replacements, err := a.nextBatch(ctx, len(missing), dad)
	if err != nil {
		return nil, err
	}
	for i, index := range missing {
		addresses[index] = replacements[i]
	}
	return addresses, nil
}

func (a *IPAllocator) nextBatch(
	ctx context.Context,
	count int,
	dad *DADClient,
) ([]netip.Addr, error) {
	addresses := make([]netip.Addr, 0, count)
	for len(addresses) < count {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		candidates := make([]netip.Addr, count-len(addresses))
		for i := range candidates {
			ip, ok := a.used.nextFree(a.pool, a.pool.Addr())
			if !ok {
				return nil, fmt.Errorf("IP pool %s exhausted", a.pool)
			}
			a.used.add(netip.PrefixFrom(ip, ip.BitLen()))
			candidates[i] = ip
		}

		if dad == nil {
			addresses = append(addresses, candidates...)
			continue
		}

		available, err := dad.ProbeBatch(ctx, candidates)
		if err != nil {
			return nil, err
		}
		for i, ok := range available {
			if ok {
				addresses = append(addresses, candidates[i])
			}
		}
	}
	return addresses, nil
}
