package room

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/core/entity"
	coreroom "github.com/tfkfan/gogame/internal/core/room"
	"github.com/tfkfan/gogame/internal/core/session"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/game/player"
)

type testEvent struct {
	Type int             `json:"type"`
	Data json.RawMessage `json:"data"`
}

type fakeSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	outbox *snapshot.Outbox

	mu           sync.Mutex
	written      [][]byte
	closeCode    websocket.StatusCode
	closeReason  string
	wroteMessage chan struct{}
	gate         chan struct{}
	once         sync.Once
}

func newFakeSession() *fakeSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeSession{
		ctx:          ctx,
		cancel:       cancel,
		outbox:       snapshot.NewOutbox(ctx.Done()),
		wroteMessage: make(chan struct{}, 64),
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
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	s.written = append(s.written, append([]byte(nil), msg...))
	s.mu.Unlock()
	select {
	case s.wroteMessage <- struct{}{}:
	default:
	}
	return nil
}

func (s *fakeSession) StartWriter(ctx context.Context) {
	s.once.Do(func() {
		go session.RunWriter(ctx, s)
	})
}

func (s *fakeSession) Stop() {
	s.cancel()
}

func (s *fakeSession) Close(code websocket.StatusCode, reason string) {
	s.mu.Lock()
	s.closeCode = code
	s.closeReason = reason
	s.mu.Unlock()
}

func (s *fakeSession) eventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.written)
}

func (s *fakeSession) events(t *testing.T) []testEvent {
	t.Helper()

	s.mu.Lock()
	raw := append([][]byte(nil), s.written...)
	s.mu.Unlock()

	events := make([]testEvent, 0, len(raw))
	for _, msg := range raw {
		var event testEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("invalid server event %q: %v", msg, err)
		}
		events = append(events, event)
	}
	return events
}

func (s *fakeSession) eventsOfType(t *testing.T, messageType int) []testEvent {
	t.Helper()

	matched := make([]testEvent, 0)
	for _, event := range s.events(t) {
		if event.Type == messageType {
			matched = append(matched, event)
		}
	}
	return matched
}

func newTestRoom(t *testing.T, cfg *config.Config) *GameRoom {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	room := NewGameRoom(ctx, cancel, cfg)
	t.Cleanup(func() {
		cancel()
		_ = room.Shutdown()
	})
	return room
}

func joinPlayer(t *testing.T, room *GameRoom, id uuid.UUID, controller player.Controller) (*player.Player, *fakeSession) {
	t.Helper()

	p := player.New(id, player.DefaultProfile(), controller)
	sess := newFakeSession()
	if err := room.Join(coreroom.Member{Entity: p, Session: sess}); err != nil {
		t.Fatalf("join failed: %v", err)
	}
	return p, sess
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before the deadline")
}

func memberOf(e entity.Entity) coreroom.Member {
	return coreroom.Member{Entity: e}
}
