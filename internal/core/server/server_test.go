package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type fakeRoom struct {
	id     uuid.UUID
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}
	once   sync.Once
}

func (r *fakeRoom) RoomID() uuid.UUID {
	return r.id
}

func (r *fakeRoom) RunRoom(ctx context.Context, onClose func()) {
	select {
	case <-ctx.Done():
	case <-r.closed:
	}
	onClose()
}

func (r *fakeRoom) Close() {
	r.once.Do(func() {
		close(r.closed)
	})
}

type fakePending struct {
	key  string
	done chan struct{}
	once sync.Once
}

func newFakePending(key string) *fakePending {
	return &fakePending{key: key, done: make(chan struct{})}
}

func (p *fakePending) Key() string {
	return p.key
}

func (p *fakePending) Done() <-chan struct{} {
	return p.done
}

func (p *fakePending) Close() {
	p.once.Do(func() {
		close(p.done)
	})
}

type fakeDefinition struct {
	mu       sync.Mutex
	rooms    map[uuid.UUID]*fakeRoom
	created  int
	admitErr error
	read     bool
	lastErr  error

	capacity     int
	pendings     map[string]*fakePending
	placed       [][]string
	placeErr     error
	placeOnce    int
	placeGate    chan struct{}
	placeStarted chan struct{}
	placeOnceRun sync.Once

	slowStarted chan struct{}
	slowRelease chan struct{}
	slowOnce    sync.Once
}

func newFakeDefinition() *fakeDefinition {
	return &fakeDefinition{
		rooms:        make(map[uuid.UUID]*fakeRoom),
		pendings:     make(map[string]*fakePending),
		capacity:     2,
		slowStarted:  make(chan struct{}),
		slowRelease:  make(chan struct{}),
		placeStarted: make(chan struct{}),
	}
}

func (d *fakeDefinition) Capacity() int {
	return d.capacity
}

func (d *fakeDefinition) NewPending(_ context.Context, _ *http.Request, _ *websocket.Conn) (Pending, error) {
	pending := newFakePending(uuid.NewString())
	d.mu.Lock()
	d.pendings[pending.Key()] = pending
	d.mu.Unlock()
	return pending, nil
}

func (d *fakeDefinition) Place(_ Room, pending []Pending) error {
	keys := make([]string, len(pending))
	for index, item := range pending {
		keys[index] = item.Key()
	}

	d.placeOnceRun.Do(func() {
		close(d.placeStarted)
	})
	if d.placeGate != nil {
		<-d.placeGate
	}

	d.mu.Lock()
	d.placed = append(d.placed, keys)
	err := d.placeErr
	if d.placeOnce > 0 {
		d.placeOnce--
		if d.placeOnce == 0 {
			d.placeErr = nil
		}
	}
	d.mu.Unlock()
	return err
}

func (d *fakeDefinition) Read(pending Pending, _ Room) error {
	<-pending.Done()
	return ErrPendingClosed
}

func (d *fakeDefinition) placements() [][]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	copied := make([][]string, len(d.placed))
	for index, entry := range d.placed {
		copied[index] = append([]string(nil), entry...)
	}
	return copied
}

func (d *fakeDefinition) setPlaceErr(err error, occurrences int) {
	d.mu.Lock()
	d.placeErr = err
	d.placeOnce = occurrences
	d.mu.Unlock()
}

func (d *fakeDefinition) Mount(mux *http.ServeMux, host *Matchmaker) {
	mux.HandleFunc("/room", host.CreateRoom)
	mux.HandleFunc("/ws", host.Connect)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		d.slowOnce.Do(func() {
			close(d.slowStarted)
		})
		<-d.slowRelease
		w.WriteHeader(http.StatusOK)
	})
}

func (d *fakeDefinition) NewRoom(ctx context.Context, cancel context.CancelFunc) (Room, error) {
	room := &fakeRoom{id: uuid.New(), ctx: ctx, cancel: cancel, closed: make(chan struct{})}
	d.mu.Lock()
	d.rooms[room.id] = room
	d.created++
	d.mu.Unlock()
	return room, nil
}

func (d *fakeDefinition) RoomFromRequest(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.URL.Query().Get("room"))
}

func (d *fakeDefinition) Admit(_ *http.Request, ws *websocket.Conn, room Room) error {
	d.mu.Lock()
	admitErr := d.admitErr
	read := d.read
	d.mu.Unlock()

	if admitErr != nil {
		d.recordError(admitErr)
		return admitErr
	}

	target := room.(*fakeRoom)
	if read {
		for {
			if _, _, err := ws.Read(context.Background()); err != nil {
				d.recordError(err)
				return err
			}
		}
	}

	<-target.ctx.Done()
	err := target.ctx.Err()
	d.recordError(err)
	return err
}

func (d *fakeDefinition) setAdmitErr(err error) {
	d.mu.Lock()
	d.admitErr = err
	d.mu.Unlock()
}

func (d *fakeDefinition) setRead(read bool) {
	d.mu.Lock()
	d.read = read
	d.mu.Unlock()
}

func (d *fakeDefinition) recordError(err error) {
	d.mu.Lock()
	d.lastErr = err
	d.mu.Unlock()
}

func (d *fakeDefinition) lastError() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastErr
}

func (d *fakeDefinition) roomCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.rooms)
}

func newMatchmakerUnderTest(t *testing.T) (*Matchmaker, *fakeDefinition) {
	t.Helper()

	def := newFakeDefinition()
	host, errc, err := New(Options{
		Addr:              "127.0.0.1:0",
		AssetsDir:         t.TempDir(),
		AssetContentTypes: map[string]string{".wasm": "application/wasm"},
		WebSocketEndpoint: "/ws",
	}, def)
	if err != nil {
		t.Fatalf("host construction failed: %v", err)
	}
	t.Cleanup(func() {
		_ = host.Shutdown(context.Background())
		select {
		case <-errc:
		default:
		}
	})
	return host, def
}

func (h *Matchmaker) httpURL(path string) string {
	return "http://" + h.Addr().String() + path
}

func (h *Matchmaker) wsURL(roomID uuid.UUID) string {
	return "ws://" + h.Addr().String() + "/ws?room=" + roomID.String()
}

func createRoom(t *testing.T, host *Matchmaker) (uuid.UUID, string) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Post(host.httpURL("/room"), "", nil)
	if err != nil {
		t.Fatalf("room creation failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read room response failed: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("room creation status = %d, want 200", response.StatusCode)
	}
	id, err := uuid.Parse(string(body))
	if err != nil {
		t.Fatalf("room id %q is not a UUID: %v", body, err)
	}
	return id, string(body)
}

func dialRoom(t *testing.T, host *Matchmaker, id uuid.UUID) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, host.wsURL(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn
}

func readCloseCode(t *testing.T, conn *websocket.Conn) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Fatal("read succeeded, want a close error")
	}
	return int(websocket.CloseStatus(err))
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
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

func TestCreateRoomRegistersAndReturnsUUID(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	id, body := createRoom(t, host)
	if body == "" {
		t.Fatal("room creation returned an empty body")
	}
	if _, ok := host.load(id); !ok {
		t.Fatal("created room is not registered")
	}
	if host.roomCount() != 1 || def.roomCount() != 1 {
		t.Fatalf("registry and definition counts = %d and %d, want 1", host.roomCount(), def.roomCount())
	}
}

func TestCreateRoomRejectsNonPost(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(host.httpURL("/room"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.StatusCode)
	}
	if def.created != 0 {
		t.Fatalf("rooms created = %d, want 0", def.created)
	}
}

func TestConcurrentRoomCreationIsSafe(t *testing.T) {
	host, _ := newMatchmakerUnderTest(t)

	const rooms = 25
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, rooms)
	for i := 0; i < rooms; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 2 * time.Second}
			response, err := client.Post(host.httpURL("/room"), "", nil)
			if err != nil {
				return
			}
			defer func() {
				_ = response.Body.Close()
			}()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return
			}
			id, err := uuid.Parse(string(body))
			if err == nil {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[uuid.UUID]struct{}, rooms)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate room id %s", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != rooms {
		t.Fatalf("created %d rooms, want %d", len(seen), rooms)
	}
	if host.roomCount() != rooms {
		t.Fatalf("registry count = %d, want %d", host.roomCount(), rooms)
	}
}

func TestClosedRoomLeavesRegistry(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	id, _ := createRoom(t, host)

	def.mu.Lock()
	room := def.rooms[id]
	def.mu.Unlock()
	if room == nil {
		t.Fatal("created room missing from the definition")
	}
	room.Close()

	waitFor(t, func() bool {
		_, ok := host.load(id)
		return !ok
	})
}

func TestDefinitionRoutesMount(t *testing.T) {
	host, _ := newMatchmakerUnderTest(t)

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(host.httpURL("/ping"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(body) != "pong" {
		t.Fatalf("ping body = %q, want pong", body)
	}
}

func TestUnknownRoomClosesWith1014(t *testing.T) {
	host, _ := newMatchmakerUnderTest(t)

	conn := dialRoom(t, host, uuid.New())
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	if code := readCloseCode(t, conn); code != 1014 {
		t.Fatalf("close code = %d, want 1014", code)
	}
}

func TestAdmissionErrorClosesWith1014(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)
	def.setAdmitErr(errors.New("admission rejected"))

	id, _ := createRoom(t, host)
	conn := dialRoom(t, host, id)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	if code := readCloseCode(t, conn); code != 1014 {
		t.Fatalf("close code = %d, want 1014", code)
	}
}

func TestCanceledRoomClosesWith1001(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	id, _ := createRoom(t, host)
	def.mu.Lock()
	room := def.rooms[id]
	def.mu.Unlock()
	if room == nil {
		t.Fatal("created room missing from the definition")
	}

	conn := dialRoom(t, host, id)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	room.cancel()
	if code := readCloseCode(t, conn); code != 1001 {
		t.Fatalf("close code = %d, want 1001", code)
	}
}

func TestNormalClientClosePassesThrough(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)
	def.setRead(true)

	id, _ := createRoom(t, host)
	conn := dialRoom(t, host, id)

	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("client close failed: %v", err)
	}

	waitFor(t, func() bool { return def.lastError() != nil })
	if code := websocket.CloseStatus(def.lastError()); code != websocket.StatusNormalClosure {
		t.Fatalf("admission saw close code %v, want normal closure", code)
	}
}

func TestOversizedFrameClosesWith1009(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)
	def.setRead(true)

	id, _ := createRoom(t, host)
	conn := dialRoom(t, host, id)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	ctx := testContext(t)
	payload := make([]byte, 40000)
	_ = conn.Write(ctx, websocket.MessageText, payload)

	if code := readCloseCode(t, conn); code != 1009 {
		t.Fatalf("close code = %d, want 1009", code)
	}
}

func TestShutdownCancelsRooms(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	id, _ := createRoom(t, host)
	def.mu.Lock()
	room := def.rooms[id]
	def.mu.Unlock()
	if room == nil {
		t.Fatal("created room missing from the definition")
	}

	if err := host.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	select {
	case <-room.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("host shutdown did not cancel the room context")
	}
	waitFor(t, func() bool {
		_, ok := host.load(id)
		return !ok
	})
}

func TestShutdownReportsDeadline(t *testing.T) {
	host, def := newMatchmakerUnderTest(t)

	client := &http.Client{Timeout: 5 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err := client.Get(host.httpURL("/slow"))
		if err == nil {
			_ = response.Body.Close()
		}
	}()

	<-def.slowStarted

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := host.Shutdown(ctx); err == nil {
		t.Fatal("shutdown with an expired context returned nil")
	}

	close(def.slowRelease)
	<-done
}

func TestBusyAddressFailsConstruction(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	def := newFakeDefinition()
	host, _, err := New(Options{Addr: listener.Addr().String()}, def)
	if err == nil {
		_ = host.Shutdown(context.Background())
		t.Fatal("host construction succeeded on a busy address")
	}
}

func TestRoomIDIsJSONEncodable(t *testing.T) {
	host, _ := newMatchmakerUnderTest(t)

	id, body := createRoom(t, host)
	if body != id.String() {
		t.Fatalf("response %q does not match the parsed id %q", body, id)
	}
	var decoded uuid.UUID
	if err := json.Unmarshal([]byte("\""+body+"\""), &decoded); err != nil {
		t.Fatalf("room id is not a JSON UUID: %v", err)
	}
	if decoded != id {
		t.Fatalf("decoded id %s, want %s", decoded, id)
	}
}
