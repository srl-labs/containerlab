package mgmt

import (
	"container/heap"
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"
)

// dadSendBurst limits packets per wakeup to amortize timer overhead.
const dadSendBurst = 8

var errDADClosed = errors.New("DAD scheduler closed")

type dadRequest struct {
	ip                     netip.Addr
	next                   time.Time
	attempts, index, users int
	done                   chan struct{}
	err                    error
}
type dadQueue []*dadRequest

func (q dadQueue) Len() int           { return len(q) }
func (q dadQueue) Less(i, j int) bool { return q[i].next.Before(q[j].next) }
func (q dadQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}
func (q *dadQueue) Push(x any) {
	r := x.(*dadRequest)
	r.index = len(*q)
	*q = append(*q, r)
}
func (q *dadQueue) Pop() any {
	old := *q
	r := old[len(old)-1]
	old[len(old)-1] = nil
	*q = old[:len(old)-1]
	r.index = -1
	return r
}

// dadScheduler uses a timer heap for probe sends and observation deadlines.
// The aggregate send rate is bounded independently of the pending address count.
type dadScheduler struct {
	ctx                   context.Context
	cancel                context.CancelCauseFunc
	mu                    sync.Mutex
	pending               map[netip.Addr]*dadRequest
	queue                 dadQueue
	wake                  chan struct{}
	done                  chan struct{}
	interval, observation time.Duration
	nextSend              time.Time
	burstLeft             int
	send                  func(netip.Addr) error
	drain                 func() error
}

func newDADScheduler(send func(netip.Addr) error, drain func() error, interval, observation time.Duration) *dadScheduler {
	ctx, cancel := context.WithCancelCause(context.Background())
	s := &dadScheduler{
		ctx:         ctx,
		cancel:      cancel,
		pending:     make(map[netip.Addr]*dadRequest),
		wake:        make(chan struct{}, 1),
		done:        make(chan struct{}),
		interval:    interval,
		burstLeft:   dadSendBurst,
		observation: observation,
		send:        send,
		drain:       drain,
	}
	go s.run()
	return s
}
func (s *dadScheduler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *dadScheduler) Check(ctx context.Context, ip netip.Addr) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if err := context.Cause(s.ctx); err != nil {
		s.mu.Unlock()
		return err
	}
	r := s.pending[ip]
	if r == nil {
		r = &dadRequest{ip: ip, next: time.Now(), done: make(chan struct{})}
		s.pending[ip] = r
		heap.Push(&s.queue, r)
	}
	r.users++
	s.mu.Unlock()
	s.notify()
	defer func() {
		s.mu.Lock()
		r.users--
		if r.users == 0 && s.pending[ip] == r {
			s.finish(r, context.Cause(ctx))
		}
		s.mu.Unlock()
		s.notify()
	}()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case <-r.done:
		if err := context.Cause(s.ctx); err != nil {
			return err
		}
		return r.err
	}
}

// finish and heap changes require mu. Removing the map entry prevents late
// replies or cancellations from completing a request twice.
func (s *dadScheduler) finish(r *dadRequest, err error) {
	if s.pending[r.ip] != r {
		return
	}
	delete(s.pending, r.ip)
	if r.index >= 0 {
		heap.Remove(&s.queue, r.index)
	}
	r.err = err
	close(r.done)
}
func (s *dadScheduler) conflict(ip netip.Addr, err error) {
	s.mu.Lock()
	if r := s.pending[ip]; r != nil {
		s.finish(r, err)
	}
	s.mu.Unlock()
	s.notify()
}
func (s *dadScheduler) restartPending() {
	s.mu.Lock()
	now := time.Now()
	for _, r := range s.pending {
		r.attempts = 0
		r.next = now
	}
	heap.Init(&s.queue)
	s.mu.Unlock()
	s.notify()
}
func (s *dadScheduler) Close() error {
	s.cancel(errDADClosed)
	<-s.done
	if err := context.Cause(s.ctx); !errors.Is(err, errDADClosed) {
		return err
	}
	return nil
}
func (s *dadScheduler) run() {
	defer close(s.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		if len(s.queue) == 0 {
			s.mu.Unlock()
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		r := s.queue[0]
		due := r.next
		if r.attempts < 3 && s.nextSend.After(due) {
			due = s.nextSend
		}
		if delay := time.Until(due); delay > 0 {
			s.mu.Unlock()
			timer.Reset(delay)
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			case <-timer.C:
			}
			timer.Stop()
			continue
		}
		if r.attempts == 3 {
			// Drain already received packets before declaring an address free. The
			// receiver and this drain share a lock, including conflict dispatch.
			s.mu.Unlock()
			if err := s.drain(); err != nil {
				if errors.Is(err, errDADObservationLost) {
					s.restartPending()
					continue
				}
				s.cancel(err)
				return
			}
			s.mu.Lock()
			if s.pending[r.ip] == r && r.attempts == 3 {
				s.finish(r, nil)
			}
			s.mu.Unlock()
			continue
		}
		// Only this goroutine sends. Space bounded bursts from actual send
		// completion; scheduler pauses must not produce catch-up bursts.
		s.mu.Unlock()
		if err := s.send(r.ip); err != nil {
			s.cancel(err)
			return
		}
		now := time.Now()
		s.mu.Lock()
		if s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		s.burstLeft--
		if s.burstLeft == 0 {
			s.nextSend = now.Add(s.interval * dadSendBurst)
			s.burstLeft = dadSendBurst
		}
		if s.pending[r.ip] != r {
			s.mu.Unlock()
			continue
		}
		r.attempts++
		r.next = now.Add(s.observation)
		heap.Fix(&s.queue, r.index)
		s.mu.Unlock()
	}
}
