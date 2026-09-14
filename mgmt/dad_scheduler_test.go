package mgmt

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func TestDADSchedulerRateAndWindows(t *testing.T) {
	const count = 80
	interval, observation := time.Millisecond, 20*time.Millisecond
	var mu sync.Mutex
	sends := map[netip.Addr][]time.Time{}
	var all []time.Time
	s := newDADScheduler(func(ip netip.Addr) error {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		all = append(all, now)
		sends[ip] = append(sends[ip], now)
		return nil
	}, func() error { return nil }, interval, observation)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Go(func() {
			ip := netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
			if i%2 == 1 {
				ip = netip.AddrFrom16([16]byte{0x20, 1, 0xd, 0xb8, 15: byte(i + 1)})
			}
			if err := s.Check(ctx, ip); err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			times := sends[ip]
			if len(times) != 3 || time.Since(times[2]) < observation {
				t.Errorf("accepted without three observation windows: %s %v", ip, times)
			}
		})
	}
	wg.Wait()
	if len(all) != count*3 {
		t.Fatalf("sent %d packets", len(all))
	}
	for i := dadSendBurst; i < len(all); i++ {
		if all[i].Sub(all[i-dadSendBurst]) < interval*dadSendBurst {
			t.Fatal("aggregate rate exceeded")
		}
	}
	for ip, times := range sends {
		for i := 1; i < len(times); i++ {
			if times[i].Sub(times[i-1]) < observation {
				t.Fatalf("probe spacing violated for %s", ip)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) != 0 || len(s.queue) != 0 {
		t.Fatal("completed requests retained")
	}
}

func TestDADSchedulerCancellationAndLateConflict(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.2")
	sent := make(chan netip.Addr, 10)
	s := newDADScheduler(func(ip netip.Addr) error { sent <- ip; return nil }, func() error { return nil }, time.Millisecond, 20*time.Millisecond)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkCtx, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- s.Check(checkCtx, ip) }()
	select {
	case <-sent:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	s.mu.Lock()
	remaining := len(s.pending) + len(s.queue)
	s.mu.Unlock()
	if remaining != 0 {
		t.Fatal("cancelled request retained")
	}
	// A late response during the final observation window must still reject.
	go func() { result <- s.Check(ctx, ip.Next()) }()
	for i := 0; i < 3; i++ {
		select {
		case <-sent:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	s.conflict(ip.Next(), ErrDuplicateAddress)
	if err := <-result; !errors.Is(err, ErrDuplicateAddress) {
		t.Fatalf("late conflict: %v", err)
	}
}

func TestDADSchedulerErrorsAndDrain(t *testing.T) {
	for _, stage := range []string{"send", "receive", "late-drain"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("socket failure")
			ip := netip.MustParseAddr("2001:db8::2")
			var s *dadScheduler
			s = newDADScheduler(func(netip.Addr) error {
				if stage == "send" {
					return failure
				}
				return nil
			}, func() error {
				if stage == "late-drain" {
					s.conflict(ip, ErrDuplicateAddress)
					return nil
				}
				return failure
			}, time.Millisecond, time.Millisecond)
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			want := failure
			if stage == "late-drain" {
				want = ErrDuplicateAddress
			}
			if err := s.Check(ctx, ip); !errors.Is(err, want) {
				t.Fatalf("%s: %v", stage, err)
			}
		})
	}
}

func TestDADSchedulerRestartsAfterObservationLoss(t *testing.T) {
	var sends, drains int
	s := newDADScheduler(
		func(netip.Addr) error {
			sends++
			return nil
		},
		func() error {
			drains++
			if drains == 1 {
				return errDADObservationLost
			}
			return nil
		},
		time.Millisecond,
		time.Millisecond,
	)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Check(ctx, netip.MustParseAddr("192.0.2.2")); err != nil {
		t.Fatal(err)
	}
	if sends != 6 || drains != 2 {
		t.Fatalf("sends=%d drains=%d, want 6 and 2", sends, drains)
	}
}

func TestDADSchedulerSendDoesNotBlockConflicts(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.2")
	entered := make(chan struct{})
	release := make(chan struct{})
	s := newDADScheduler(
		func(netip.Addr) error {
			close(entered)
			<-release
			return nil
		},
		func() error { return nil },
		time.Millisecond,
		time.Second,
	)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- s.Check(ctx, ip) }()
	<-entered
	conflicted := make(chan struct{})
	go func() {
		s.conflict(ip, ErrDuplicateAddress)
		close(conflicted)
	}()
	select {
	case <-conflicted:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("socket write held the scheduler mutex")
	}
	close(release)
	if err := <-result; !errors.Is(err, ErrDuplicateAddress) {
		t.Fatalf("conflict result: %v", err)
	}
}
