package shared

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestSchedule(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	defer s.Shutdown(context.Background())

	fired := make(chan time.Time, 1)
	start := time.Now()
	s.Schedule(20*time.Millisecond, func() {
		fired <- time.Now()
	})

	select {
	case at := <-fired:
		if waited := at.Sub(start); waited < 15*time.Millisecond {
			t.Fatalf("callback fired too early: %v", waited)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback did not fire")
	}

	select {
	case <-fired:
		t.Fatal("callback fired more than once")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestScheduleAt(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	defer s.Shutdown(context.Background())

	at := time.Now().Add(30 * time.Millisecond)
	fired := make(chan time.Time, 1)
	s.ScheduleAt(at, func() {
		fired <- time.Now()
	})

	select {
	case got := <-fired:
		if got.Before(at) {
			t.Fatalf("callback fired %v before the deadline", at.Sub(got))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback did not fire")
	}
}

func TestScheduleAtPast(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	defer s.Shutdown(context.Background())

	fired := make(chan struct{}, 1)
	s.ScheduleAt(time.Now().Add(-time.Hour), func() {
		fired <- struct{}{}
	})

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("callback did not fire")
	}
}

func TestSchedulePeriodic(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	defer s.Shutdown(context.Background())

	ticks := make(chan time.Time, 8)
	s.SchedulePeriodic(5*time.Millisecond, 10*time.Millisecond, func() {
		select {
		case ticks <- time.Now():
		default:
		}
	})

	start := time.Now()
	for i := range 4 {
		select {
		case <-ticks:
		case <-time.After(2 * time.Second):
			t.Fatalf("received %d ticks, want 4", i)
		}
	}
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Fatalf("ticks fired too fast: %v", elapsed)
	}
}

func TestScheduleFromCallback(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	defer s.Shutdown(context.Background())

	rescheduled := make(chan struct{}, 1)
	s.Schedule(10*time.Millisecond, func() {
		s.Schedule(10*time.Millisecond, func() {
			rescheduled <- struct{}{}
		})
	})

	select {
	case <-rescheduled:
	case <-time.After(2 * time.Second):
		t.Fatal("callback scheduled from a callback did not fire")
	}
}

func TestScheduleAfterShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	var called atomic.Bool
	callback := func() {
		called.Store(true)
	}
	s.Schedule(0, callback)
	s.ScheduleAt(time.Now(), callback)
	s.SchedulePeriodic(0, time.Millisecond, callback)

	time.Sleep(50 * time.Millisecond)
	if called.Load() {
		t.Fatal("callback scheduled after Shutdown ran")
	}
}

func TestShutdownFromCallback(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	done := make(chan struct{})
	s.Schedule(10*time.Millisecond, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = s.Shutdown(ctx)
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown from a callback deadlocked")
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown after callback completion failed: %v", err)
	}
}

func TestShutdownWaitsForCallbacks(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	started := make(chan struct{})
	release := make(chan struct{})
	s.Schedule(0, func() {
		close(started)
		<-release
	})
	<-started

	result := make(chan error, 1)
	go func() {
		result <- s.Shutdown(context.Background())
	}()

	select {
	case err := <-result:
		t.Fatalf("Shutdown returned before the callback finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return")
	}
}

func TestShutdownDeadline(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	s.Schedule(0, func() {
		close(started)
		<-release
		close(finished)
	})
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if err := s.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}

	close(release)
	<-finished
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown after callback completion failed: %v", err)
	}
}

func TestShutdownDropsPending(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	var called atomic.Bool
	s.Schedule(100*time.Millisecond, func() {
		called.Store(true)
	})
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if called.Load() {
		t.Fatal("pending callback ran after Shutdown")
	}
}

func TestShutdownIdle(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown failed: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown failed: %v", err)
	}
}

func TestScheduleManyTasks(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	var count atomic.Int64
	const total = 2000
	for range total {
		s.Schedule(0, func() {
			count.Add(1)
		})
	}

	deadline := time.Now().Add(5 * time.Second)
	for count.Load() < total && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := count.Load(); got != total {
		t.Fatalf("ran %d callbacks, want %d", got, total)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestShutdownDropsManyPending(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewScheduler()

	for range 1000 {
		s.Schedule(time.Hour, func() {})
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}
