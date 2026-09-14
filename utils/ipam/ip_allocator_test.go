package ipam

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"reflect"
	"testing"
)

func TestCanonicalPrefix(t *testing.T) {
	for input, want := range map[string]string{
		"192.0.2.42/24":   "192.0.2.0/24",
		"2001:0db8::1/64": "2001:db8::/64",
		"":                "",
		"invalid":         "invalid",
	} {
		if got := CanonicalPrefix(input); got != want {
			t.Errorf("CanonicalPrefix(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestCanonicalIP(t *testing.T) {
	for input, want := range map[string]string{
		"192.0.2.1":       "192.0.2.1",
		"192.0.2.1/24":    "192.0.2.1",
		"2001:0db8::1/64": "2001:db8::1",
		"":                "",
		"invalid":         "invalid",
	} {
		if got := CanonicalIP(input); got != want {
			t.Errorf("CanonicalIP(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestIPPrefixSetMatchesExhaustiveSearch(t *testing.T) {
	for _, text := range []string{"192.0.2.0/24", "2001:db8::/120"} {
		pool := netip.MustParsePrefix(text)
		var set IPPrefixSet
		used := make(map[netip.Addr]bool)
		rng := rand.New(rand.NewPCG(1, 2))
		addresses := make([]netip.Addr, 256)
		for i, ip := 0, pool.Addr(); i < 256; i, ip = i+1, ip.Next() {
			addresses[i] = ip
		}
		for step := 0; step < 150; step++ {
			prefix := netip.PrefixFrom(addresses[rng.IntN(256)], pool.Bits()+3+rng.IntN(6)).Masked()
			set.Add(prefix)
			for _, ip := range addresses {
				if prefix.Contains(ip) {
					used[ip] = true
				}
			}
			for i, start := range addresses {
				_, occupied := set.Lookup(start)
				if occupied != used[start] {
					t.Fatalf("lookup mismatch at %s", start)
				}
				want := netip.Addr{}
				for _, ip := range addresses[i:] {
					if !used[ip] {
						want = ip
						break
					}
				}
				got, ok := set.NextFree(pool, start)
				if ok != want.IsValid() || (ok && got != want) {
					t.Fatalf("step %d, %s: next %s/%t, want %s", step, start, got, ok, want)
				}
			}
		}
	}
}

func TestIPPrefixSetLargeIPv6Reservation(t *testing.T) {
	var set IPPrefixSet
	set.Add(netip.MustParsePrefix("::/1"))
	got, ok := set.NextFree(netip.MustParsePrefix("::/0"), netip.MustParseAddr("::1"))
	if !ok || got != netip.MustParseAddr("8000::") {
		t.Fatalf("did not skip /1: %s %t", got, ok)
	}
	if _, ok := set.NextFree(
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParseAddr("2001:db8::"),
	); ok {
		t.Fatal("allocated inside occupied ancestor")
	}
	set.Add(netip.MustParsePrefix("8000::/1"))
	if _, ok := set.NextFree(netip.MustParsePrefix("::/0"), netip.MustParseAddr("::")); ok {
		t.Fatal("failed to coalesce full root")
	}
}

func BenchmarkIPPrefixSetLookup(b *testing.B) {
	for _, count := range []int{1, 1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var set IPPrefixSet
			for i := 0; i < count; i++ {
				set.Add(netip.MustParsePrefix(fmt.Sprintf("2001:db8:%x::/64", i)))
			}
			target := netip.MustParseAddr("2001:db8:ffff::1")
			b.ResetTimer()
			for b.Loop() {
				set.Lookup(target)
			}
		})
	}
}

func allocateOne(
	a *IPAllocator,
	ctx context.Context,
	key string,
	accept func(netip.Addr) bool,
) (netip.Addr, error) {
	addresses, err := a.AllocateBatch(ctx, []string{key}, 1, accept)
	if err != nil {
		return netip.Addr{}, err
	}
	return addresses[0], nil
}

func TestIPAllocator(t *testing.T) {
	for _, subnet := range []string{"192.0.2.0/30", "2001:db8::/126"} {
		t.Run(subnet, func(t *testing.T) {
			prefix := netip.MustParsePrefix(subnet)
			a, err := NewIPAllocator(prefix, prefix, []netip.Addr{prefix.Addr().Next()})
			if err != nil {
				t.Fatal(err)
			}
			ip, err := allocateOne(a, context.Background(), "node", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !prefix.Contains(ip) || ip == prefix.Addr() || ip == prefix.Addr().Next() {
				t.Fatalf("reserved address allocated: %s", ip)
			}
			if err := a.Reserve(ip); err == nil {
				t.Fatal("duplicate reservation succeeded")
			}
			b, err := NewIPAllocator(prefix, prefix, []netip.Addr{prefix.Addr().Next()})
			if err != nil {
				t.Fatal(err)
			}
			again, err := allocateOne(b, context.Background(), "node", nil)
			if err != nil || ip != again {
				t.Fatalf("unstable allocation: %s %v", again, err)
			}
			if !ip.Is4() {
				if _, err := allocateOne(a, context.Background(), "other", nil); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := allocateOne(a, context.Background(), "full", nil); err == nil {
				t.Fatal("expected exhaustion")
			}
		})
	}
}

func TestIPAllocatorInvalidPools(t *testing.T) {
	for _, pool := range []string{"192.0.3.0/24", "192.0.0.0/16", "2001:db8::/64", "::ffff:192.0.2.0/120"} {
		if _, err := NewIPAllocator(
			netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParsePrefix(pool),
			nil,
		); err == nil {
			t.Fatalf("accepted pool %s", pool)
		}
	}
	p := netip.MustParsePrefix("192.0.2.2/32")
	a, err := NewIPAllocator(p, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := allocateOne(a, context.Background(), "full", nil); err == nil {
		t.Fatal("allocated subnet boundary")
	}
}

func TestIPAllocatorBoolCallback(t *testing.T) {
	for _, subnet := range []string{"192.0.2.0/29", "2001:db8::/125"} {
		prefix := netip.MustParsePrefix(subnet)
		a, err := NewIPAllocator(prefix, prefix, nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[netip.Addr]bool{}
		ip, err := allocateOne(a, context.Background(), "node", func(ip netip.Addr) bool {
			if seen[ip] {
				t.Fatalf("repeated rejected candidate %s", ip)
			}
			seen[ip] = true
			return len(seen) == 3
		})
		if err != nil || !seen[ip] || len(seen) != 3 {
			t.Fatalf("callback allocation: %s %v", ip, err)
		}
		if _, err := allocateOne(a, context.Background(), "full", func(netip.Addr) bool { return false }); err == nil {
			t.Fatal("expected exhaustion")
		}
	}
}

func TestIPAllocatorCallbackCancellation(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/64")
	a, err := NewIPAllocator(prefix, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	failure := errors.New("probe failed")
	calls := 0
	_, err = allocateOne(a, ctx, "node", func(netip.Addr) bool { calls++; cancel(failure); return false })
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("failed to abort retry: %v, %d calls", err, calls)
	}
}

func TestIPAllocatorBatchDeterministic(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	keys := make([]string, 70)
	for i := range keys {
		keys[i] = "same-hash"
	}
	var want []netip.Addr
	for _, concurrency := range []int{1, 4, 64} {
		a, err := NewIPAllocator(prefix, prefix, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := a.AllocateBatch(context.Background(), keys, concurrency, func(ip netip.Addr) bool {
			return ip.As4()[3]%3 != 0
		})
		if err != nil {
			t.Fatal(err)
		}
		if want == nil {
			want = got
		} else if !reflect.DeepEqual(want, got) {
			t.Fatal("completion order changed allocation")
		}
	}
}
