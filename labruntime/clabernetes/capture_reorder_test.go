package clabernetes

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// reorderSink records what a reorder buffer writes.
type reorderSink struct {
	mu      sync.Mutex
	written []int64
	ids     []uint32
	failAt  int
}

func (s *reorderSink) write(id uint32, packet *pcapngPacket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAt > 0 && len(s.written) == s.failAt {
		return errors.New("write |1: broken pipe")
	}
	s.written = append(s.written, packet.Timestamp)
	s.ids = append(s.ids, id)

	return nil
}

func (s *reorderSink) timestamps() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.written)
}

// interleavedStreams pushes two synthetic streams of ticks, interleaved in time: stream 0 is
// delivered at once, stream 1 `delay` after its packets were stamped. Both stamp a packet every
// 2 ms, offset by 1 ms.
func interleavedStreams(t *testing.T, q *captureReorder, ticks int, delay time.Duration) {
	t.Helper()

	start := time.Now()
	type delivery struct {
		at time.Time
		id uint32
		ts int64
	}
	var deliveries []delivery
	for tick := range ticks {
		stamp := start.Add(time.Duration(tick) * 2 * time.Millisecond)
		deliveries = append(deliveries,
			delivery{at: stamp, id: 0, ts: stamp.UnixNano()},
			delivery{
				at: stamp.Add(time.Millisecond + delay), id: 1,
				ts: stamp.Add(time.Millisecond).UnixNano(),
			})
	}
	slices.SortStableFunc(deliveries, func(a, b delivery) int { return a.at.Compare(b.at) })

	for _, next := range deliveries {
		time.Sleep(time.Until(next.at))
		if err := q.push(
			next.id,
			&pcapngPacket{InterfaceID: next.id, Timestamp: next.ts},
		); err != nil {
			t.Fatal(err)
		}
	}
}

func runReorder(q *captureReorder) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		q.run(ctx)
	}()

	return func() {
		cancel()
		<-done
		q.flush()
	}
}

func TestCaptureReorderOrdersStreamsWithinTheWindow(t *testing.T) {
	t.Parallel()

	sink := &reorderSink{}
	q := newCaptureReorder(100*time.Millisecond, sink.write, nil)
	stop := runReorder(q)
	interleavedStreams(t, q, 60, 40*time.Millisecond)

	// packets go out while the capture runs, not only at the end
	time.Sleep(150 * time.Millisecond)
	if len(sink.timestamps()) == 0 {
		t.Fatal("nothing written while the streams ran")
	}
	stop()

	written := sink.timestamps()
	if len(written) != 120 || !slices.IsSorted(written) {
		t.Fatalf("written %d packets, sorted %v", len(written), slices.IsSorted(written))
	}
	if q.stats.late != 0 {
		t.Fatalf("late packets = %d", q.stats.late)
	}
}

func TestCaptureReorderWritesLatePacketsAsTheyCome(t *testing.T) {
	t.Parallel()

	sink := &reorderSink{}
	q := newCaptureReorder(20*time.Millisecond, sink.write, nil)
	stop := runReorder(q)
	// stream 1 arrives 80 ms after its stamps, far beyond the 20 ms window
	interleavedStreams(t, q, 60, 80*time.Millisecond)
	stop()

	written := sink.timestamps()
	if len(written) != 120 {
		t.Fatalf("written %d packets, want all 120", len(written))
	}
	if q.stats.late == 0 || q.stats.maxLate < 40*time.Millisecond || slices.IsSorted(written) {
		t.Fatalf("late = %d (max %s), sorted %v; want late packets written out of order",
			q.stats.late, q.stats.maxLate, slices.IsSorted(written))
	}
}

func TestCaptureReorderHoldsPacketsAfterAHold(t *testing.T) {
	t.Parallel()

	sink := &reorderSink{}
	q := newCaptureReorder(time.Millisecond, sink.write, nil)
	stop := runReorder(q)
	defer stop()

	chain := &captureChain{}
	q.hold(chain, 100)
	for _, ts := range []int64{50, 150, 100, 200} {
		if err := q.push(0, &pcapngPacket{Timestamp: ts}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(30 * time.Millisecond)
	if got := sink.timestamps(); !slices.Equal(got, []int64{50, 100}) {
		t.Fatalf("written under the hold = %v, want [50 100]", got)
	}

	// the chain writes what it held back, then releases
	if err := q.push(1, &pcapngPacket{Timestamp: 120}); err != nil {
		t.Fatal(err)
	}
	q.release(chain)
	time.Sleep(30 * time.Millisecond)
	if got := sink.timestamps(); !slices.Equal(got, []int64{50, 100, 120, 150, 200}) {
		t.Fatalf("written after the release = %v", got)
	}
}

func TestCaptureReorderStopsAtTheFirstWriteError(t *testing.T) {
	t.Parallel()

	sink := &reorderSink{failAt: 2}
	failed := 0
	q := newCaptureReorder(time.Millisecond, sink.write, func() { failed++ })
	stop := runReorder(q)

	var err error
	for ts := int64(1); ts < 50 && err == nil; ts++ {
		err = q.push(0, &pcapngPacket{Timestamp: ts})
		time.Sleep(2 * time.Millisecond)
	}
	stop()

	if err == nil || failed != 1 || len(sink.timestamps()) != 2 {
		t.Fatalf(
			"push error = %v, onFail calls %d, written %d",
			err,
			failed,
			len(sink.timestamps()),
		)
	}
}

func TestCaptureReorderWithoutWindowWritesThrough(t *testing.T) {
	t.Parallel()

	sink := &reorderSink{}
	q := newCaptureReorder(0, sink.write, nil)
	for _, ts := range []int64{30, 10, 20} {
		if err := q.push(0, &pcapngPacket{Timestamp: ts}); err != nil {
			t.Fatal(err)
		}
	}
	if got := sink.timestamps(); !slices.Equal(got, []int64{30, 10, 20}) {
		t.Fatalf("written = %v, want arrival order", got)
	}
}

func TestCaptureReorderWindowKnob(t *testing.T) {
	cases := map[string]time.Duration{
		"":      250 * time.Millisecond,
		"0":     0,
		"75ms":  75 * time.Millisecond,
		"bogus": 250 * time.Millisecond,
	}
	for value, want := range cases {
		t.Setenv(envCaptureReorderWindow, value)
		r := &Runtime{captureReorderWindow: captureReorderWindowFromEnv()}
		if got := r.captureReorderWindowLength(); got != want {
			t.Errorf("%s=%q: window %s, want %s", envCaptureReorderWindow, value, got, want)
		}
	}
}
