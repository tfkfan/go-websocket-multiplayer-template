package room

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/core/entity"
	"github.com/tfkfan/gogame/internal/core/session"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/core/spatial"
)

type testEntity struct {
	entity.Base
	radius  float64
	updates int
	step    spatial.Vec2
}

func newTestEntity(id uuid.UUID, position spatial.Vec2, radius float64) *testEntity {
	return &testEntity{
		Base:   entity.Base{Id: id, Position: position},
		radius: radius,
	}
}

func (e *testEntity) Shape() spatial.Shape {
	return spatial.Circle{Center: e.Position, Radius: e.radius}
}

func (e *testEntity) Update(dt float64) {
	e.updates++
	e.Position = e.Position.Add(e.step.Scale(dt))
}

func (e *testEntity) Extent() float64 {
	return e.radius
}

type testSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	outbox *snapshot.Outbox

	mu          sync.Mutex
	written     [][]byte
	closeCode   websocket.StatusCode
	closeReason string
	once        sync.Once
}

func newTestSession() *testSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &testSession{
		ctx:    ctx,
		cancel: cancel,
		outbox: snapshot.NewOutbox(ctx.Done()),
	}
}

func (s *testSession) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *testSession) Publish(kind snapshot.SnapshotType, msg []byte) bool {
	return s.outbox.Publish(kind, msg)
}

func (s *testSession) NextOutbound() []byte {
	return s.outbox.Next()
}

func (s *testSession) Signal() <-chan struct{} {
	return s.outbox.Signal
}

func (s *testSession) Write(_ context.Context, msg []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written = append(s.written, append([]byte(nil), msg...))
	return nil
}

func (s *testSession) StartWriter(ctx context.Context) {
	s.once.Do(func() {
		go session.RunWriter(ctx, s)
	})
}

func (s *testSession) Stop() {
	s.cancel()
}

func (s *testSession) Close(code websocket.StatusCode, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCode = code
	s.closeReason = reason
}

func (s *testSession) wrote() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.written)
}

func (s *testSession) closeInfo() (websocket.StatusCode, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCode, s.closeReason
}

func (s *testSession) messages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.written...)
}

type testHooks struct {
	mu      sync.Mutex
	joined  []Member
	left    []Member
	ticks   int
	joinErr error
	onTick  func(dt float64)
}

func (h *testHooks) OnJoin(member Member) error {
	h.mu.Lock()
	h.joined = append(h.joined, member)
	err := h.joinErr
	h.mu.Unlock()
	return err
}

func (h *testHooks) OnLeave(member Member) {
	h.mu.Lock()
	h.left = append(h.left, member)
	h.mu.Unlock()
}

func (h *testHooks) OnTick(dt float64) {
	h.mu.Lock()
	h.ticks++
	callback := h.onTick
	h.mu.Unlock()
	if callback != nil {
		callback(dt)
	}
}

func (h *testHooks) counts() (joined int, left int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.joined), len(h.left)
}

func newTestRoom(t *testing.T, timeout time.Duration) (*Room, *testHooks) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	hooks := &testHooks{}
	room := New(ctx, cancel, Config{
		Id:           uuid.New(),
		Timeout:      timeout,
		TickInterval: time.Hour,
	}, hooks)
	t.Cleanup(func() {
		cancel()
		_ = room.Shutdown()
	})
	return room, hooks
}

func TestJoinRejectsDuplicateAndClosed(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	first := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: first}); err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if err := room.Join(Member{Entity: first}); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate join error = %v, want ErrExists", err)
	}
	if room.Len() != 1 {
		t.Fatalf("membership = %d, want 1", room.Len())
	}

	room.close(nil)
	if err := room.Join(Member{Entity: newTestEntity(uuid.New(), spatial.Vec2{}, 25)}); !errors.Is(err, ErrClosed) {
		t.Fatalf("join to closed room error = %v, want ErrClosed", err)
	}
}

func TestJoinRejectsInvalidMember(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	if err := room.Join(Member{}); !errors.Is(err, ErrInvalidMember) {
		t.Fatalf("join without entity error = %v, want ErrInvalidMember", err)
	}
}

func TestPublishToMissingAndSessionlessMembers(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	sessionless := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: sessionless}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	if err := room.PublishTo(sessionless.Key(), snapshot.PrivateType, []byte("x")); !errors.Is(err, ErrNoSession) {
		t.Fatalf("session-less publish error = %v, want ErrNoSession", err)
	}
	if err := room.PublishTo(uuid.NewString(), snapshot.PrivateType, []byte("x")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing member publish error = %v, want ErrNotFound", err)
	}
}

func TestBroadcastTargetsConnectedMembersOnly(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	connected := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	connectedSession := newTestSession()
	detached := newTestEntity(uuid.New(), spatial.Vec2{}, 25)

	if err := room.Join(Member{Entity: connected, Session: connectedSession}); err != nil {
		t.Fatalf("connected join failed: %v", err)
	}
	if err := room.Join(Member{Entity: detached}); err != nil {
		t.Fatalf("detached join failed: %v", err)
	}

	room.Broadcast(snapshot.BroadcastType, []byte("snapshot"))

	waitFor(t, func() bool { return connectedSession.wrote() == 1 })
	if got := string(connectedSession.messages()[0]); got != "snapshot" {
		t.Fatalf("connected member received %q, want snapshot", got)
	}
}

func TestReapRemovesStoppedSessions(t *testing.T) {
	room, hooks := newTestRoom(t, time.Minute)

	stopped := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	stoppedSession := newTestSession()
	active := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	sessionless := newTestEntity(uuid.New(), spatial.Vec2{}, 25)

	if err := room.Join(Member{Entity: stopped, Session: stoppedSession}); err != nil {
		t.Fatalf("stopped join failed: %v", err)
	}
	if err := room.Join(Member{Entity: active, Session: newTestSession()}); err != nil {
		t.Fatalf("active join failed: %v", err)
	}
	if err := room.Join(Member{Entity: sessionless}); err != nil {
		t.Fatalf("session-less join failed: %v", err)
	}

	stoppedSession.Stop()
	room.reapStopped()

	if room.Len() != 2 {
		t.Fatalf("membership = %d, want 2 after reaping one stopped session", room.Len())
	}
	if _, ok := room.Member(stopped.Key()); ok {
		t.Fatal("stopped member still present after reaping")
	}
	if _, ok := room.Member(sessionless.Key()); !ok {
		t.Fatal("session-less member was reaped")
	}
	_, left := hooks.counts()
	if left != 1 {
		t.Fatalf("leave hook calls = %d, want 1", left)
	}
}

func TestLeaveStopsSession(t *testing.T) {
	room, hooks := newTestRoom(t, time.Minute)

	session := newTestSession()
	member := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: member, Session: session}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	if err := room.Leave(member.Key()); err != nil {
		t.Fatalf("leave failed: %v", err)
	}
	if room.Len() != 0 {
		t.Fatalf("membership = %d, want 0", room.Len())
	}
	select {
	case <-session.Done():
	default:
		t.Fatal("leave did not stop the session")
	}
	if _, left := hooks.counts(); left != 1 {
		t.Fatalf("leave hook calls = %d, want 1", left)
	}
}

func TestTimeoutClosesRoom(t *testing.T) {
	room, _ := newTestRoom(t, 30*time.Millisecond)

	closed := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(context.Background(), func(*Room) {
			close(closed)
		})
	}()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("room did not close on its scheduled timeout")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("room loop did not stop after close")
	}
	if !room.Closed() {
		t.Fatal("room reported open after timeout close")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	session := newTestSession()
	member := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: member, Session: session}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	calls := 0
	room.close(func(*Room) { calls++ })
	room.close(func(*Room) { calls++ })

	if calls != 1 {
		t.Fatalf("close callback calls = %d, want 1", calls)
	}
	if room.Len() != 0 {
		t.Fatalf("membership after close = %d, want 0", room.Len())
	}
	code, reason := session.closeInfo()
	if code != websocket.StatusGoingAway || reason != "room closed" {
		t.Fatalf("session close = %v %q, want 1001 room closed", code, reason)
	}
}

func TestJoinStartsWriter(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	session := newTestSession()
	member := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: member, Session: session}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	room.Broadcast(snapshot.SystemType, []byte("join"))
	waitFor(t, func() bool { return session.wrote() == 1 })
}

func TestMembersReturnsSnapshot(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	first := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	second := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	if err := room.Join(Member{Entity: first}); err != nil {
		t.Fatalf("join failed: %v", err)
	}
	if err := room.Join(Member{Entity: second}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	members := room.Members()
	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}
	if err := room.Leave(first.Key()); err != nil {
		t.Fatalf("leave failed: %v", err)
	}
	if len(members) != 2 {
		t.Fatal("members snapshot changed after leave")
	}
}

func waitFor(t *testing.T, condition func() bool) {
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

func TestMembershipIsOrderIndependent(t *testing.T) {
	build := func(reversed bool) string {
		room, _ := newTestRoom(t, time.Minute)

		ids := []uuid.UUID{
			uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		}
		members := []Member{
			{Entity: newTestEntity(ids[0], spatial.Vec2{}, 25), Session: newTestSession()},
			{Entity: newTestEntity(ids[1], spatial.Vec2{}, 25)},
			{Entity: newTestEntity(ids[2], spatial.Vec2{}, 25)},
		}

		if reversed {
			for index := len(members) - 1; index >= 0; index-- {
				if err := room.Join(members[index]); err != nil {
					t.Fatalf("join failed: %v", err)
				}
			}
		} else {
			for _, member := range members {
				if err := room.Join(member); err != nil {
					t.Fatalf("join failed: %v", err)
				}
			}
		}

		keys := make([]string, 0, room.Len())
		for _, member := range room.Members() {
			keys = append(keys, member.Entity.Key())
		}
		sort.Strings(keys)
		return strings.Join(keys, ",")
	}

	if first, second := build(false), build(true); first != second {
		t.Fatalf("membership differs by admission order: %s and %s", first, second)
	}
}

func TestJoinAllAdmitsBatchBeforeRun(t *testing.T) {
	room, hooks := newTestRoom(t, time.Minute)

	first := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	second := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	firstSession := newTestSession()
	secondSession := newTestSession()

	if err := room.JoinAll([]Member{
		{Entity: first, Session: firstSession},
		{Entity: second, Session: secondSession},
	}); err != nil {
		t.Fatalf("batch join failed: %v", err)
	}

	if room.Len() != 2 {
		t.Fatalf("membership = %d, want 2", room.Len())
	}
	joined, left := hooks.counts()
	if joined != 2 || left != 0 {
		t.Fatalf("hooks joined=%d left=%d, want 2 and 0", joined, left)
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(runCtx, func(*Room) {})
	}()
	defer func() {
		runCancel()
		<-done
	}()
	waitFor(t, func() bool { return room.started.Load() })
}

func TestJoinAllRollsBackOnFailure(t *testing.T) {
	room, hooks := newTestRoom(t, time.Minute)

	first := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	dead := newTestEntity(uuid.New(), spatial.Vec2{}, 25)
	deadSession := newTestSession()
	deadSession.Stop()

	err := room.JoinAll([]Member{
		{Entity: first, Session: newTestSession()},
		{Entity: dead, Session: deadSession},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("batch join error = %v, want ErrNotFound", err)
	}
	if room.Len() != 0 {
		t.Fatalf("membership after rollback = %d, want 0", room.Len())
	}
	if _, ok := room.Member(first.Key()); ok {
		t.Fatal("first member survived the rollback")
	}
	joined, left := hooks.counts()
	if joined != 1 || left != 1 {
		t.Fatalf("hooks joined=%d left=%d, want 1 and 1", joined, left)
	}
}

func TestJoinAllRejectsStartedRoom(t *testing.T) {
	room, _ := newTestRoom(t, time.Minute)

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(runCtx, func(*Room) {})
	}()
	defer func() {
		runCancel()
		<-done
	}()

	waitFor(t, func() bool {
		return errors.Is(room.JoinAll(nil), ErrRoomStarted)
	})
}
