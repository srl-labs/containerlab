package util

import (
	"bytes"
	"sync"
)

// Queue is a simple queue structure to store and queue/requeue/dequeue bytes.
type Queue struct {
	queue     [][]byte
	depth     int
	depthChan chan int
	lock      *sync.RWMutex
}

// NewQueue returns a prepared Queue object.
func NewQueue() *Queue {
	depthChan := make(chan int, 1)
	depthChan <- 0

	return &Queue{
		depthChan: depthChan,
		lock:      &sync.RWMutex{},
	}
}

// Requeue prepends some bytes to the front of the queue.
func (q *Queue) Requeue(b []byte) {
	q.lock.Lock()
	defer q.lock.Unlock()

	n := [][]byte{b}
	q.queue = append(n, q.queue...)

	q.depth++

	<-q.depthChan
	q.depthChan <- q.depth
}

// Enqueue queues some bytes at the end of the queue.
func (q *Queue) Enqueue(b []byte) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.queue = append(q.queue, b)
	q.depth++

	<-q.depthChan
	q.depthChan <- q.depth
}

// Dequeue returns the first bytes in the queue.
func (q *Queue) Dequeue() []byte {
	// check the depth before acquiring a full read/write lock which can cause deadlocks with tons
	// of concurrent access to enqueue/deque.
	if q.getDepth() == 0 {
		return nil
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	b := q.queue[0]

	q.queue = q.queue[1:]
	q.depth--

	<-q.depthChan
	q.depthChan <- q.depth

	return b
}

// DequeueAll returns all bytes in the queue.
func (q *Queue) DequeueAll() []byte {
	if q.getDepth() == 0 {
		return nil
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	b := q.queue

	q.queue = nil

	q.depth = 0

	<-q.depthChan
	q.depthChan <- q.depth

	return bytes.Join(b, []byte{})
}

func (q *Queue) getDepth() int {
	// rather than locking/unlocking to access the q.depth, we simply grab the depth from the
	// depthChan and then put it back in and return the value we got. this should be slightly faster
	// and less cpu than locking/unlocking
	d := <-q.depthChan
	q.depthChan <- d

	return d
}

// GetDepth returns the depth of the queue.
func (q *Queue) GetDepth() int {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.depth
}
