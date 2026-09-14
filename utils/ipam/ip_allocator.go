package ipam

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"net/netip"
	"sync"

	"github.com/srl-labs/containerlab/types"
)

// a generic IP allocator
type IPAllocator struct {
	mu     sync.Mutex
	subnet netip.Prefix
	pool   netip.Prefix
	used   types.IPPrefixSet
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
