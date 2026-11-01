package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tfkfan/gogame/internal/core/snapshot"
)

type fakeSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	outbox *snapshot.Outbox

	mu           sync.Mutex
	written      [][]byte
	writeErr     error
	closeCode    websocket.StatusCode
	closeReason  string
	wroteMessage chan struct{}
}

func newFakeSession() *fakeSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeSession{
		ctx:          ctx,
		cancel:       cancel,
		outbox:       snapshot.NewOutbox(ctx.Done()),
		wroteMessage: make(chan struct{}, 16),
	}
}

func (s *fakeSession) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *fakeSession) Publish(kind snapshot.SnapshotType, msg []byte) bool {
	return s.outbox.Publish(kind, msg)
}

func (s *fakeSession) NextOutbound() []byte {
	return s.outbox.Next()
}

func (s *fakeSession) Signal() <-chan struct{} {
	return s.outbox.Signal
}

func (s *fakeSession) Write(_ context.Context, msg []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeErr != nil {
		return s.writeErr
	}
	s.written = append(s.written, append([]byte(nil), msg...))
	select {
	case s.wroteMessage <- struct{}{}:
	default:
	}
	return nil
}

func (s *fakeSession) Stop() {
	s.cancel()
}

func (s *fakeSession) Close(code websocket.StatusCode, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCode = code
	s.closeReason = reason
}

func (s *fakeSession) messages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.written...)
}

func TestWriterDrainsInNumericOrder(t *testing.T) {
	sess := newFakeSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sess.Publish(snapshot.BroadcastType, []byte("broadcast"))
	sess.Publish(snapshot.SystemType, []byte("system"))

	go RunWriter(ctx, sess)

	waitForMessages(t, sess, 2)
	got := sess.messages()
	if string(got[0]) != "system" || string(got[1]) != "broadcast" {
		t.Fatalf("writer order = [%q %q], want system then broadcast", got[0], got[1])
	}
}

func TestWriterWaitsForSignal(t *testing.T) {
	sess := newFakeSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go RunWriter(ctx, sess)

	sess.Publish(snapshot.PrivateType, []byte("later"))
	waitForMessages(t, sess, 1)
	if got := string(sess.messages()[0]); got != "later" {
		t.Fatalf("written message = %q, want later", got)
	}
}

func TestWriterStopsOnContextAndSessionClose(t *testing.T) {
	sess := newFakeSession()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunWriter(ctx, sess)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not stop after the room context closed")
	}

	second := newFakeSession()
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		RunWriter(context.Background(), second)
	}()
	second.Stop()
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not stop after the session closed")
	}
}

func TestWriterClosesSessionOnWriteFailure(t *testing.T) {
	sess := newFakeSession()
	sess.writeErr = errors.New("write failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go RunWriter(ctx, sess)
	sess.Publish(snapshot.BroadcastType, []byte("payload"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sess.mu.Lock()
		code := sess.closeCode
		reason := sess.closeReason
		sess.mu.Unlock()
		if code != 0 {
			if code != websocket.StatusInternalError || reason != "write failed" {
				t.Fatalf("close = %v %q, want 1011 write failed", code, reason)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("writer did not close the session after a write failure")
}

func waitForMessages(t *testing.T, sess *fakeSession, count int) {
	t.Helper()

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for len(sess.messages()) < count {
		select {
		case <-sess.wroteMessage:
		case <-deadline.C:
			t.Fatalf("writer delivered %d messages, want %d", len(sess.messages()), count)
		}
	}
}
