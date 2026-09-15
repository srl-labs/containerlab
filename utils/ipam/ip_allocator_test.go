package ipam

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"slices"
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
		var set ipPrefixSet
		used := make(map[netip.Addr]bool)
		rng := rand.New(rand.NewPCG(1, 2))
		addresses := make([]netip.Addr, 256)
		for i, ip := 0, pool.Addr(); i < 256; i, ip = i+1, ip.Next() {
			addresses[i] = ip
		}
		for step := 0; step < 150; step++ {
			prefix := netip.PrefixFrom(addresses[rng.IntN(256)], pool.Bits()+3+rng.IntN(6)).Masked()
			set.add(prefix)
			for _, ip := range addresses {
				if prefix.Contains(ip) {
					used[ip] = true
				}
			}
			for i, start := range addresses {
				_, occupied := set.lookup(start)
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
				got, ok := set.nextFree(pool, start)
				if ok != want.IsValid() || (ok && got != want) {
					t.Fatalf("step %d, %s: next %s/%t, want %s", step, start, got, ok, want)
				}
			}
		}
	}
}

func TestIPPrefixSetLargeIPv6Reservation(t *testing.T) {
	var set ipPrefixSet
	set.add(netip.MustParsePrefix("::/1"))
	got, ok := set.nextFree(netip.MustParsePrefix("::/0"), netip.MustParseAddr("::1"))
	if !ok || got != netip.MustParseAddr("8000::") {
		t.Fatalf("did not skip /1: %s %t", got, ok)
	}
	if _, ok := set.nextFree(
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParseAddr("2001:db8::"),
	); ok {
		t.Fatal("allocated inside occupied ancestor")
	}
	set.add(netip.MustParsePrefix("8000::/1"))
	if _, ok := set.nextFree(netip.MustParsePrefix("::/0"), netip.MustParseAddr("::")); ok {
		t.Fatal("failed to coalesce full root")
	}
}

func BenchmarkIPPrefixSetLookup(b *testing.B) {
	for _, count := range []int{1, 1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var set ipPrefixSet
			for i := 0; i < count; i++ {
				set.add(netip.MustParsePrefix(fmt.Sprintf("2001:db8:%x::/64", i)))
			}
			target := netip.MustParseAddr("2001:db8:ffff::1")
			b.ResetTimer()
			for b.Loop() {
				set.lookup(target)
			}
		})
	}
}

func TestIPAllocator(t *testing.T) {
	for _, subnet := range []string{"192.0.2.0/30", "2001:db8::/126"} {
		t.Run(subnet, func(t *testing.T) {
			prefix := netip.MustParsePrefix(subnet)
			a, err := NewIPAllocator(prefix, prefix, []netip.Addr{prefix.Addr().Next()})
			if err != nil {
				t.Fatal(err)
			}
			ip, err := a.Next(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !prefix.Contains(ip) || ip == prefix.Addr() || ip == prefix.Addr().Next() {
				t.Fatalf("reserved address allocated: %s", ip)
			}
			if err := a.Reserve(ip); err == nil {
				t.Fatal("duplicate reservation succeeded")
			}
			if !ip.Is4() {
				if _, err := a.Next(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := a.Next(context.Background()); err == nil {
				t.Fatal("expected exhaustion")
			}
		})
	}
}

func TestIPAllocatorNextBatch(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/29")
	allocator, err := NewIPAllocator(prefix, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := allocator.NextBatch(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("192.0.2.2"),
		netip.MustParseAddr("192.0.2.3"),
	}
	if !slices.Equal(addresses, want) {
		t.Fatalf("addresses = %v, want %v", addresses, want)
	}
	if _, err := allocator.NextBatch(context.Background(), -1); err == nil {
		t.Fatal("accepted negative batch size")
	}
}

func TestIPAllocatorNextPreferredBatch(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/29")
	allocator, err := NewIPAllocator(prefix, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := allocator.NextPreferredBatch(context.Background(), []netip.Addr{
		netip.MustParseAddr("192.0.2.5"),
		netip.MustParseAddr("192.0.2.5"),
		netip.MustParseAddr("198.51.100.1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Addr{
		netip.MustParseAddr("192.0.2.5"),
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("192.0.2.2"),
	}
	if !slices.Equal(addresses, want) {
		t.Fatalf("addresses = %v, want %v", addresses, want)
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
	if _, err := a.Next(context.Background()); err == nil {
		t.Fatal("allocated subnet boundary")
	}
}

func TestIPAllocatorDADClient(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/29")
	a, err := NewIPAllocator(prefix, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Next(context.Background(), nil); err == nil {
		t.Fatal("accepted a nil DAD client")
	}
}

func TestIPAllocatorSkipsDADConflict(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/29")
	a, err := NewIPAllocator(prefix, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	dad := &DADClient{parent: "does-not-exist", cache: make(map[netip.Addr]struct{})}
	for address := prefix.Addr().Next(); prefix.Contains(address.Next()); address = address.Next() {
		dad.cache[address] = struct{}{}
	}

	if _, err := a.Next(context.Background(), dad); err == nil {
		t.Fatal("allocated from an occupied pool")
	}
}
