package capture

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// maxReorderBytes bounds the packet data the reorder buffer holds. Past it, the oldest packets
// are written early, which can put them out of order.
const maxReorderBytes = 64 << 20

// Reorder writes the packets of several capture sources in timestamp order.
//
// Each source (a node's capture stream, or a local interface socket) delivers its packets with
// its own delay. The buffer holds every packet for `window` after it arrived and writes the held
// packets in timestamp order, so a packet that was stamped earlier but delivered up to `window`
// later still goes out first. A packet stamped before the last packet written arrived too late
// to be ordered: it is written as it comes and counted, and the count goes to the debug log when
// the capture ends.
//
// A source can place a hold at the earliest timestamp it may still write (Hold); packets stamped
// after a hold wait for it to be released (Release). The c9s runtime holds while it switches
// between chained capture segments.
type Reorder struct {
	write  func(id uint32, packet *Packet) error
	onFail func()
	window time.Duration
	now    func() time.Time

	mu     sync.Mutex
	queue  reorderQueue
	bytes  int
	seq    uint64
	holds  map[any]int64
	err    error
	wake   chan struct{}
	stats  reorderStats
	cursor int64 // timestamp of the latest packet written in order
}

type reorderStats struct {
	late    int
	maxLate time.Duration
	forced  int
}

// reorderedPacket is a packet waiting in the buffer.
type reorderedPacket struct {
	id      uint32
	packet  *Packet
	arrived time.Time
	seq     uint64
}

// reorderQueue is a min-heap by timestamp, then arrival.
type reorderQueue []reorderedPacket

func (q reorderQueue) Len() int { return len(q) }

func (q reorderQueue) Less(i, j int) bool {
	if q[i].packet.Timestamp != q[j].packet.Timestamp {
		return q[i].packet.Timestamp < q[j].packet.Timestamp
	}

	return q[i].seq < q[j].seq
}

func (q reorderQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *reorderQueue) Push(
	x any,
) {
	*q = append(*q, x.(reorderedPacket))
} //nolint:forcetypeassert // heap contract

func (q *reorderQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]

	return item
}

// NewReorder returns a buffer that writes through write; onFail is called once a write
// failed. A window of zero or less writes every packet as it arrives.
func NewReorder(
	window time.Duration,
	write func(id uint32, packet *Packet) error,
	onFail func(),
) *Reorder {
	return &Reorder{
		write: write, onFail: onFail, window: window, now: time.Now,
		holds: map[any]int64{}, wake: make(chan struct{}, 1),
	}
}

// push hands a packet to the buffer. It returns the error of a failed write.
func (q *Reorder) Push(id uint32, packet *Packet) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.err != nil {
		return q.err
	}
	if q.window <= 0 {
		return q.writeLocked(id, packet)
	}
	if q.cursor != 0 && packet.Timestamp < q.cursor {
		q.stats.late++
		q.stats.maxLate = max(q.stats.maxLate, time.Duration(q.cursor-packet.Timestamp))
		if q.stats.late == 1 {
			log.Debug("Capture packet arrived after the reorder window, writing it out of order",
				"late_by", time.Duration(q.cursor-packet.Timestamp), "window", q.window)
		}

		return q.writeLocked(id, packet)
	}

	q.seq++
	heap.Push(&q.queue, reorderedPacket{id: id, packet: packet, arrived: q.now(), seq: q.seq})
	q.bytes += len(packet.Data)
	for q.bytes > maxReorderBytes && q.queue.Len() > 0 {
		q.stats.forced++
		if err := q.popLocked(); err != nil {
			return err
		}
	}

	select {
	case q.wake <- struct{}{}:
	default:
	}

	return nil
}

// hold keeps packets stamped after timestamp in the buffer until release(key).
func (q *Reorder) Hold(key any, timestamp int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.holds[key] = timestamp
}

// release removes the hold of key.
func (q *Reorder) Release(key any) {
	q.mu.Lock()
	_, held := q.holds[key]
	delete(q.holds, key)
	q.mu.Unlock()

	if held {
		select {
		case q.wake <- struct{}{}:
		default:
		}
	}
}

// run writes due packets until ctx ends.
func (q *Reorder) Run(ctx context.Context) {
	if q.window <= 0 {
		return
	}

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	for {
		next := q.writeDue()
		wait := time.Hour
		if !next.IsZero() {
			wait = max(next.Sub(q.now()), time.Millisecond)
		}
		resetTimer(timer, wait)

		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		case <-timer.C:
		}
	}
}

// writeDue writes the packets that waited for the window and are not behind a hold, in
// timestamp order. It returns when the next packet is due, or zero when that depends on a new
// packet or a released hold.
func (q *Reorder) writeDue() time.Time {
	q.mu.Lock()
	defer q.mu.Unlock()

	held, limit := false, int64(0)
	for _, timestamp := range q.holds {
		if !held || timestamp < limit {
			held, limit = true, timestamp
		}
	}

	now := q.now()
	for q.queue.Len() > 0 && q.err == nil {
		head := q.queue[0]
		if held && head.packet.Timestamp > limit {
			return time.Time{}
		}
		if due := head.arrived.Add(q.window); due.After(now) {
			return due
		}
		if q.popLocked() != nil {
			return time.Time{}
		}
	}

	return time.Time{}
}

// flush writes every remaining packet in timestamp order and logs what could not be ordered.
func (q *Reorder) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.queue.Len() > 0 && q.err == nil {
		if q.popLocked() != nil {
			break
		}
	}
	q.queue, q.bytes = nil, 0

	if q.stats.late > 0 || q.stats.forced > 0 {
		log.Debug("Capture wrote packets out of timestamp order",
			"late_packets", q.stats.late, "max_late_by", q.stats.maxLate,
			"window", q.window, "buffer_overflow_packets", q.stats.forced)
	}
}

func (q *Reorder) popLocked() error {
	item := heap.Pop(&q.queue).(reorderedPacket) //nolint:forcetypeassert // heap contract
	q.bytes -= len(item.packet.Data)
	q.cursor = max(q.cursor, item.packet.Timestamp)

	return q.writeLocked(item.id, item.packet)
}

func (q *Reorder) writeLocked(id uint32, packet *Packet) error {
	if err := q.write(id, packet); err != nil {
		if q.err == nil {
			q.err = err
			if q.onFail != nil {
				q.onFail()
			}
		}

		return err
	}

	return nil
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}
