package shared

import (
	"context"
	"sync"
	"time"
)

type Scheduler struct {
	mu      sync.Mutex
	tasks   *Heap[*timeout]
	seq     uint64
	pool    []*timeout
	running sync.WaitGroup
	timer   *time.Timer
	wake    chan struct{}
	done    chan struct{}
	exited  chan struct{}
	stopped bool
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		tasks: NewHeap(func(a, b *timeout) bool {
			if a.due.Equal(b.due) {
				return a.seq < b.seq
			}
			return a.due.Before(b.due)
		}),
		timer:  time.NewTimer(time.Hour),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		exited: make(chan struct{}),
	}
	s.timer.Stop()
	go s.run()
	return s
}

func (s *Scheduler) Schedule(timeout time.Duration, callback func()) {
	if timeout < 0 {
		timeout = 0
	}
	s.push(time.Now().Add(timeout), 0, callback)
}

func (s *Scheduler) ScheduleAt(at time.Time, callback func()) {
	s.push(at, 0, callback)
}

func (s *Scheduler) SchedulePeriodic(initialDelay, period time.Duration, callback func()) {
	if initialDelay < 0 {
		initialDelay = 0
	}
	if period < 0 {
		period = 0
	}
	s.push(time.Now().Add(initialDelay), period, callback)
}

func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	first := !s.stopped
	if first {
		s.stopped = true
		for {
			t, ok := s.tasks.Pop()
			if !ok {
				break
			}
			s.recycle(t)
		}
	}
	s.mu.Unlock()

	if first {
		close(s.done)
	}
	<-s.exited

	done := make(chan struct{})
	go func() {
		s.running.Wait()
		close(done)
	}()

	if ctx == nil {
		<-done
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		select {
		case <-done:
			return nil
		default:
			return ctx.Err()
		}
	}
}

func (s *Scheduler) run() {
	defer close(s.exited)
	defer s.stopTimer()

	for {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}

		next, hasNext := s.tasks.Peek()
		var wait time.Duration
		if hasNext {
			wait = time.Until(next.due)
			if wait < 0 {
				wait = 0
			}
		}
		s.stopTimer()
		if hasNext {
			s.timer.Reset(wait)
		}
		s.mu.Unlock()

		if !hasNext {
			select {
			case <-s.wake:
			case <-s.done:
				return
			}
			continue
		}

		select {
		case <-s.timer.C:
			s.fire()
		case <-s.wake:
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) fire() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}

	for {
		t, ok := s.tasks.Peek()
		if !ok || t.due.After(now) {
			break
		}
		s.tasks.Pop()

		callback := t.callback
		if t.period > 0 {
			t.due = nextDue(t.due, t.period, now)
			s.tasks.Push(t)
		} else {
			s.recycle(t)
		}
		if callback != nil {
			s.running.Go(func() {
				callback()
			})
		}
	}
}

func (s *Scheduler) push(due time.Time, period time.Duration, callback func()) {
	if callback == nil {
		return
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.seq++
	t := s.take()
	t.due = due
	t.period = period
	t.seq = s.seq
	t.callback = callback
	s.tasks.Push(t)
	next, _ := s.tasks.Peek()
	wake := next == t
	s.mu.Unlock()

	if wake {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

func (s *Scheduler) stopTimer() {
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
}

func (s *Scheduler) take() *timeout {
	n := len(s.pool)
	if n == 0 {
		return &timeout{}
	}
	t := s.pool[n-1]
	s.pool[n-1] = nil
	s.pool = s.pool[:n-1]
	return t
}

func (s *Scheduler) recycle(t *timeout) {
	t.due = time.Time{}
	t.period = 0
	t.callback = nil
	s.pool = append(s.pool, t)
}

func nextDue(prev time.Time, period time.Duration, now time.Time) time.Time {
	next := prev.Add(period)
	if next.After(now) {
		return next
	}
	skip := now.Sub(next)/period + 1
	return next.Add(skip * period)
}

type timeout struct {
	due      time.Time
	period   time.Duration
	seq      uint64
	callback func()
}
