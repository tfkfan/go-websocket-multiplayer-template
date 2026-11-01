package server

import "sync"

type pendingQueue struct {
	mu      sync.Mutex
	entries []queuedPending
	wake    chan struct{}
	paused  bool
}

func newPendingQueue() *pendingQueue {
	return &pendingQueue{wake: make(chan struct{}, 1)}
}

func (q *pendingQueue) Enqueue(entry queuedPending) {
	q.mu.Lock()
	q.entries = append(q.entries, entry)
	q.paused = false
	q.mu.Unlock()
	q.signal()
}

func (q *pendingQueue) Remove(key string) bool {
	q.mu.Lock()
	removed := false
	for index, entry := range q.entries {
		if entry.pending.Key() != key {
			continue
		}
		q.entries = append(q.entries[:index], q.entries[index+1:]...)
		removed = true
		break
	}
	q.mu.Unlock()

	if removed {
		q.signal()
	}
	return removed
}

func (q *pendingQueue) TakeBatch(capacity int) []queuedPending {
	q.mu.Lock()
	q.prune()
	if q.paused || capacity < 1 || len(q.entries) < capacity {
		q.mu.Unlock()
		return nil
	}

	batch := append([]queuedPending(nil), q.entries[:capacity]...)
	q.entries = append([]queuedPending(nil), q.entries[capacity:]...)
	q.mu.Unlock()

	q.signal()
	return batch
}

func (q *pendingQueue) Requeue(entries []queuedPending) {
	if len(entries) == 0 {
		return
	}

	q.mu.Lock()
	q.entries = append(append([]queuedPending(nil), entries...), q.entries...)
	q.paused = true
	q.mu.Unlock()
	q.signal()
}

func (q *pendingQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.entries)
}

func (q *pendingQueue) Wait() <-chan struct{} {
	return q.wake
}

func (q *pendingQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *pendingQueue) prune() {
	kept := q.entries[:0]
	for _, entry := range q.entries {
		if isClosed(entry.pending) {
			continue
		}
		kept = append(kept, entry)
	}
	q.entries = kept
}

func isClosed(pending Pending) bool {
	select {
	case <-pending.Done():
		return true
	default:
		return false
	}
}
