package ipam

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"net/netip"
	"sync"
)

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

// a generic IP allocator
type IPAllocator struct {
	mu     sync.Mutex
	subnet netip.Prefix
	pool   netip.Prefix
	used   IPPrefixSet
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

// NewIPAllocator creates an allocator, excluding subnet boundary addresses.
// Reserved addresses (for example gateways) may be outside the allocation pool.
func NewIPAllocator(subnet, pool netip.Prefix, reserved []netip.Addr) (*IPAllocator, error) {
	if !isValidAllocationPool(subnet, pool) {
		return nil, fmt.Errorf("allocation pool %s must be contained in subnet %s", pool, subnet)
	}
	a := &IPAllocator{subnet: subnet.Masked(), pool: pool.Masked()}
	a.used.Add(netip.PrefixFrom(a.subnet.Addr(), a.subnet.Addr().BitLen()))
	if subnet.Addr().Is4() {
		last := a.subnet.Addr().As4()
		for bit := subnet.Bits(); bit < 32; bit++ {
			last[bit/8] |= 1 << uint(7-bit%8)
		}
		a.used.Add(netip.PrefixFrom(netip.AddrFrom4(last), 32))
	}
	for _, ip := range reserved {
		if !subnet.Contains(ip) {
			return nil, fmt.Errorf("reserved address %s is outside %s", ip, subnet)
		}
		a.used.Add(netip.PrefixFrom(ip, ip.BitLen()))
	}
	return a, nil
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

// Reserve excludes an explicit address, reporting duplicates and boundaries.
func (a *IPAllocator) Reserve(ip netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, occupied := a.used.Lookup(ip)
	if !a.subnet.Contains(ip) || occupied {
		return fmt.Errorf("invalid, reserved or duplicate address %s in %s", ip, a.subnet)
	}
	a.used.Add(netip.PrefixFrom(ip, ip.BitLen()))
	return nil
}

// ReservePrefix excludes an occupied CIDR without enumerating its addresses.
func (a *IPAllocator) ReservePrefix(prefix netip.Prefix) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if prefix.IsValid() && prefix.Overlaps(a.subnet) {
		a.used.Add(prefix)
	}
}

// candidate reserves the next hashed candidate, skipping entire occupied prefixes.
func (a *IPAllocator) candidate(key string) (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	size := new(big.Int).Lsh(big.NewInt(1), uint(a.pool.Addr().BitLen()-a.pool.Bits()))
	base := new(big.Int).SetBytes(a.pool.Addr().AsSlice())
	hash := sha256.Sum256([]byte(key))
	offset := new(big.Int).Mod(new(big.Int).SetBytes(hash[:]), size)
	value := new(big.Int).Add(base, offset)
	ipBytes := value.FillBytes(make([]byte, a.pool.Addr().BitLen()/8))
	start, _ := netip.AddrFromSlice(ipBytes)
	ip, ok := a.used.NextFree(a.pool, start)
	if !ok {
		ip, ok = a.used.NextFree(a.pool, a.pool.Addr())
	}
	if !ok {
		return netip.Addr{}, fmt.Errorf("IP pool %s exhausted allocating %q", a.pool, key)
	}
	a.used.Add(netip.PrefixFrom(ip, ip.BitLen()))
	return ip, nil
}

// AllocateBatch reserves candidates in key order before checking them concurrently.
// Only rejected candidates are retried, in their original order. Each candidate
// search is bounded by address width; total work also depends on rejection count.
// The accept callback must support concurrent calls and may call ReservePrefix.
func (a *IPAllocator) AllocateBatch(ctx context.Context, keys []string, concurrency int, accept func(netip.Addr) bool) ([]netip.Addr, error) {
	addresses := make([]netip.Addr, len(keys))
	pending := make([]int, len(keys))
	for i := range keys {
		pending[i] = i
	}
	for len(pending) > 0 {
		candidates := make([]netip.Addr, len(pending))
		for i, index := range pending {
			if err := context.Cause(ctx); err != nil {
				return nil, err
			}
			ip, err := a.candidate(keys[index])
			if err != nil {
				return nil, err
			}
			candidates[i] = ip
		}
		accepted, err := CheckIPAddresses(ctx, candidates, concurrency, accept)
		if err != nil {
			return nil, err
		}
		retry := pending[:0]
		for i, index := range pending {
			if accepted[i] {
				addresses[index] = candidates[i]
			} else {
				retry = append(retry, index)
			}
		}
		pending = retry
	}
	return addresses, context.Cause(ctx)
}

// CheckIPAddresses invokes a bool predicate with bounded concurrency, returning
// results in input order. Nil accepts all addresses. Cancellation waits for all
// active callbacks to return; callbacks should observe the caller's context.
func CheckIPAddresses(ctx context.Context, addresses []netip.Addr, concurrency int, accept func(netip.Addr) bool) ([]bool, error) {
	results := make([]bool, len(addresses))
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency == 1 || accept == nil {
		for i, ip := range addresses {
			if err := context.Cause(ctx); err != nil {
				return results, err
			}
			results[i] = accept == nil || accept(ip)
		}
		return results, context.Cause(ctx)
	}
	concurrency = min(concurrency, len(addresses))
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Go(func() {
			for i := worker; i < len(addresses); i += concurrency {
				if ctx.Err() != nil {
					return
				}
				results[i] = accept == nil || accept(addresses[i])
			}
		})
	}
	wg.Wait()
	return results, context.Cause(ctx)
}
