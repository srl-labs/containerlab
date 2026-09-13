package types

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"testing"
)

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
