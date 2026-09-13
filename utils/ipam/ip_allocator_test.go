package ipam

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
)

func TestIPAllocator(t *testing.T) {
	for _, subnet := range []string{"192.0.2.0/30", "2001:db8::/126"} {
		t.Run(subnet, func(t *testing.T) {
			prefix := netip.MustParsePrefix(subnet)
			a, err := NewIPAllocator(prefix, prefix, []netip.Addr{prefix.Addr().Next()})
			if err != nil {
				t.Fatal(err)
			}
			ip, err := a.Allocate(context.Background(), "node", nil)
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
			again, err := b.Allocate(context.Background(), "node", nil)
			if err != nil || ip != again {
				t.Fatalf("unstable allocation: %s %v", again, err)
			}
			if !ip.Is4() {
				if _, err := a.Allocate(context.Background(), "other", nil); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := a.Allocate(context.Background(), "full", nil); err == nil {
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
	if _, err := a.Allocate(context.Background(), "full", nil); err == nil {
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
		ip, err := a.Allocate(context.Background(), "node", func(ip netip.Addr) bool {
			if seen[ip] {
				t.Fatalf("repeated rejected candidate %s", ip)
			}
			seen[ip] = true
			return len(seen) == 3
		})
		if err != nil || !seen[ip] || len(seen) != 3 {
			t.Fatalf("callback allocation: %s %v", ip, err)
		}
		if _, err := a.Allocate(context.Background(), "full", func(netip.Addr) bool { return false }); err == nil {
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
	_, err = a.Allocate(ctx, "node", func(netip.Addr) bool { calls++; cancel(failure); return false })
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
