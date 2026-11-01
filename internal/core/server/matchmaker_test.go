package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

func newMatchmakerWithCapacity(t *testing.T, capacity int) (*Matchmaker, *fakeDefinition) {
	t.Helper()

	def := newFakeDefinition()
	def.capacity = capacity
	host, errc, err := New(Options{
		Addr:              "127.0.0.1:0",
		AssetsDir:         t.TempDir(),
		AssetContentTypes: map[string]string{".wasm": "application/wasm"},
		WebSocketEndpoint: "/ws",
	}, def)
	if err != nil {
		t.Fatalf("matchmaker construction failed: %v", err)
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

func startMatch(host *Matchmaker, pending Pending) <-chan matchResult {
	result := make(chan matchResult, 1)
	go func() {
		room, err := host.MakeMatch(pending)
		result <- matchResult{room: room, err: err}
	}()
	return result
}

func waitQueue(t *testing.T, host *Matchmaker, length int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if host.pending.Len() == length {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("queue length = %d, want %d", host.pending.Len(), length)
}

func receiveResult(t *testing.T, result <-chan matchResult) matchResult {
	t.Helper()

	select {
	case outcome := <-result:
		return outcome
	case <-time.After(2 * time.Second):
		t.Fatal("match call did not return")
		return matchResult{}
	}
}

func TestMakeMatchWaitsForCapacity(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 2)

	firstPending := newFakePending("first")
	first := startMatch(host, firstPending)
	waitQueue(t, host, 1)

	select {
	case <-first:
		t.Fatal("match returned before capacity was reached")
	case <-time.After(50 * time.Millisecond):
	}
	if host.roomCount() != 0 {
		t.Fatalf("rooms = %d, want none before the batch fills", host.roomCount())
	}

	secondPending := newFakePending("second")
	second := startMatch(host, secondPending)
	firstResult := receiveResult(t, first)
	secondResult := receiveResult(t, second)

	if firstResult.err != nil || secondResult.err != nil {
		t.Fatalf("match errors: %v and %v", firstResult.err, secondResult.err)
	}
	if firstResult.room.RoomID() != secondResult.room.RoomID() {
		t.Fatalf("players were placed in different rooms")
	}
	if host.roomCount() != 1 {
		t.Fatalf("stored rooms = %d, want 1", host.roomCount())
	}
	if _, ok := host.load(firstResult.room.RoomID()); !ok {
		t.Fatal("formed room is not stored")
	}

	placements := def.placements()
	if len(placements) != 1 {
		t.Fatalf("placements = %d, want 1", len(placements))
	}
	if placements[0][0] != "first" || placements[0][1] != "second" {
		t.Fatalf("placement order = %v, want first, second", placements[0])
	}
}

func TestMatchTakesOldestSubsliceAndQueuesExcess(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 2)

	first := startMatch(host, newFakePending("first"))
	waitQueue(t, host, 1)
	second := startMatch(host, newFakePending("second"))

	firstResult := receiveResult(t, first)
	secondResult := receiveResult(t, second)
	if firstResult.room.RoomID() != secondResult.room.RoomID() {
		t.Fatal("the first two players were split across rooms")
	}

	third := startMatch(host, newFakePending("third"))
	waitQueue(t, host, 1)
	fourth := startMatch(host, newFakePending("fourth"))
	thirdResult := receiveResult(t, third)
	fourthResult := receiveResult(t, fourth)
	if thirdResult.room.RoomID() != fourthResult.room.RoomID() {
		t.Fatal("the excess players did not form the next room")
	}

	placements := def.placements()
	if len(placements) != 2 {
		t.Fatalf("placements = %d, want 2", len(placements))
	}
	if placements[0][0] != "first" || placements[0][1] != "second" {
		t.Fatalf("first batch = %v, want first, second", placements[0])
	}
	if placements[1][0] != "third" || placements[1][1] != "fourth" {
		t.Fatalf("second batch = %v, want third, fourth", placements[1])
	}
}

func TestDisconnectRemovesQueuedPlayerAnywhere(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 4)

	firstPending := newFakePending("first")
	middlePending := newFakePending("middle")
	thirdPending := newFakePending("third")

	first := startMatch(host, firstPending)
	waitQueue(t, host, 1)
	middle := startMatch(host, middlePending)
	waitQueue(t, host, 2)
	third := startMatch(host, thirdPending)
	waitQueue(t, host, 3)

	middlePending.Close()
	waitQueue(t, host, 2)

	middleResult := receiveResult(t, middle)
	if !errors.Is(middleResult.err, ErrPendingClosed) {
		t.Fatalf("disconnected match error = %v, want ErrPendingClosed", middleResult.err)
	}

	fourth := startMatch(host, newFakePending("fourth"))
	waitQueue(t, host, 3)
	fifth := startMatch(host, newFakePending("fifth"))

	firstResult := receiveResult(t, first)
	thirdResult := receiveResult(t, third)
	fourthResult := receiveResult(t, fourth)
	fifthResult := receiveResult(t, fifth)

	if firstResult.err != nil || thirdResult.err != nil || fourthResult.err != nil || fifthResult.err != nil {
		t.Fatalf("match errors: %v, %v, %v, %v", firstResult.err, thirdResult.err, fourthResult.err, fifthResult.err)
	}
	if firstResult.room.RoomID() != fifthResult.room.RoomID() {
		t.Fatal("survivors were split across rooms")
	}

	placements := def.placements()
	if len(placements) != 1 {
		t.Fatalf("placements = %d, want 1", len(placements))
	}
	want := []string{"first", "third", "fourth", "fifth"}
	for index, key := range want {
		if placements[0][index] != key {
			t.Fatalf("placement = %v, want %v", placements[0], want)
		}
	}
}

func TestRemovingLastQueuedPlayerPreservesOrder(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 4)

	first := startMatch(host, newFakePending("first"))
	waitQueue(t, host, 1)
	second := startMatch(host, newFakePending("second"))
	waitQueue(t, host, 2)
	lastPending := newFakePending("last")
	last := startMatch(host, lastPending)
	waitQueue(t, host, 3)

	lastPending.Close()
	waitQueue(t, host, 2)
	if result := receiveResult(t, last); !errors.Is(result.err, ErrPendingClosed) {
		t.Fatalf("disconnected match error = %v, want ErrPendingClosed", result.err)
	}

	fourth := startMatch(host, newFakePending("fourth"))
	waitQueue(t, host, 3)
	fifth := startMatch(host, newFakePending("fifth"))

	firstResult := receiveResult(t, first)
	secondResult := receiveResult(t, second)
	fourthResult := receiveResult(t, fourth)
	fifthResult := receiveResult(t, fifth)

	if firstResult.room.RoomID() != secondResult.room.RoomID() || firstResult.room.RoomID() != fifthResult.room.RoomID() {
		t.Fatal("survivors were split across rooms")
	}
	if fourthResult.err != nil {
		t.Fatalf("fourth match error: %v", fourthResult.err)
	}
	want := []string{"first", "second", "fourth", "fifth"}
	placements := def.placements()
	for index, key := range want {
		if placements[0][index] != key {
			t.Fatalf("placement = %v, want %v", placements[0], want)
		}
	}
}

func TestPlacementBlocksCallersUntilComplete(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 2)
	def.placeGate = make(chan struct{})

	first := startMatch(host, newFakePending("first"))
	waitQueue(t, host, 1)
	second := startMatch(host, newFakePending("second"))

	select {
	case <-def.placeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("placement did not start")
	}

	select {
	case <-first:
		t.Fatal("first caller returned before placement finished")
	case <-second:
		t.Fatal("second caller returned before placement finished")
	case <-time.After(50 * time.Millisecond):
	}
	if host.roomCount() != 0 {
		t.Fatalf("stored rooms = %d, want none before placement finishes", host.roomCount())
	}

	close(def.placeGate)
	if result := receiveResult(t, first); result.err != nil {
		t.Fatalf("first match error: %v", result.err)
	}
	if result := receiveResult(t, second); result.err != nil {
		t.Fatalf("second match error: %v", result.err)
	}
	if host.roomCount() != 1 {
		t.Fatalf("stored rooms = %d, want 1", host.roomCount())
	}
}

func TestPlacementFailureRequeuesSurvivors(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 2)
	def.setPlaceErr(errors.New("placement failed"), 1)

	newFakePending("placeholder")
	first := startMatch(host, newFakePending("first"))
	waitQueue(t, host, 1)
	second := startMatch(host, newFakePending("second"))

	waitFor(t, func() bool { return len(def.placements()) == 1 })
	waitQueue(t, host, 2)

	select {
	case <-first:
		t.Fatal("first caller returned after a failed placement")
	default:
	}

	third := startMatch(host, newFakePending("third"))

	firstResult := receiveResult(t, first)
	secondResult := receiveResult(t, second)
	if firstResult.err != nil || secondResult.err != nil {
		t.Fatalf("requeued survivors failed: %v and %v", firstResult.err, secondResult.err)
	}
	if firstResult.room.RoomID() != secondResult.room.RoomID() {
		t.Fatal("requeued survivors were split")
	}
	select {
	case <-third:
		t.Fatal("the third player was matched into the survivor room")
	case <-time.After(50 * time.Millisecond):
	}

	placements := def.placements()
	if len(placements) != 2 {
		t.Fatalf("placements = %d, want 2 attempts", len(placements))
	}
	if placements[1][0] != "first" || placements[1][1] != "second" {
		t.Fatalf("survivor batch = %v, want first, second", placements[1])
	}
	if host.roomCount() != 1 {
		t.Fatalf("stored rooms = %d, want 1 after the retry", host.roomCount())
	}
}

func TestShutdownReleasesQueuedMatches(t *testing.T) {
	host, _ := newMatchmakerWithCapacity(t, 2)

	waiting := startMatch(host, newFakePending("waiting"))
	waitQueue(t, host, 1)

	if err := host.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	result := receiveResult(t, waiting)
	if !errors.Is(result.err, ErrMatchmakingClosed) && !errors.Is(result.err, ErrPendingClosed) {
		t.Fatalf("blocked match error = %v, want a shutdown error", result.err)
	}
}

func TestConcurrentMatchmaking(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 3)

	const players = 9
	results := make(chan matchResult, players)
	for index := 0; index < players; index++ {
		pending := newFakePending(uuid.NewString())
		go func() {
			room, err := host.MakeMatch(pending)
			results <- matchResult{room: room, err: err}
		}()
	}

	rooms := make(map[uuid.UUID]int)
	for index := 0; index < players; index++ {
		result := receiveResult(t, results)
		if result.err != nil {
			t.Fatalf("match error: %v", result.err)
		}
		rooms[result.room.RoomID()]++
	}
	if len(rooms) != 3 {
		t.Fatalf("rooms = %d, want 3", len(rooms))
	}
	for id, count := range rooms {
		if count != 3 {
			t.Fatalf("room %s has %d players, want 3", id, count)
		}
	}
	if host.roomCount() != 3 {
		t.Fatalf("stored rooms = %d, want 3", host.roomCount())
	}
	placements := def.placements()
	if len(placements) != 3 {
		t.Fatalf("placements = %d, want 3", len(placements))
	}
}

func TestDirectRoomsCoexistWithQueue(t *testing.T) {
	host, def := newMatchmakerWithCapacity(t, 2)

	waiting := startMatch(host, newFakePending("waiting"))
	waitQueue(t, host, 1)

	directID, _ := createRoom(t, host)
	if _, ok := host.load(directID); !ok {
		t.Fatal("direct room is not stored while a player waits")
	}

	partner := startMatch(host, newFakePending("partner"))
	if result := receiveResult(t, waiting); result.err != nil {
		t.Fatalf("waiting match error: %v", result.err)
	}
	if result := receiveResult(t, partner); result.err != nil {
		t.Fatalf("partner match error: %v", result.err)
	}
	if _, ok := host.load(directID); !ok {
		t.Fatal("direct room disappeared after a match formed")
	}
	if len(def.placements()) != 1 {
		t.Fatalf("placements = %d, want 1", len(def.placements()))
	}
}

func TestRoomStorageConcurrentAccess(t *testing.T) {
	host, _ := newMatchmakerWithCapacity(t, 2)

	ids := make([]uuid.UUID, 0, 10)
	for index := 0; index < 10; index++ {
		id, _ := createRoom(t, host)
		ids = append(ids, id)
	}

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			if _, ok := host.load(id); !ok {
				t.Errorf("stored room %s was not found", id)
			}
		}(id)
	}
	for index := 0; index < 5; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 2 * time.Second}
			response, err := client.Post(host.httpURL("/room"), "", nil)
			if err != nil {
				t.Errorf("room creation failed: %v", err)
				return
			}
			_ = response.Body.Close()
		}()
	}
	wg.Wait()
}

func TestMalformedRoomParameterClosesWith1014(t *testing.T) {
	host, _ := newMatchmakerWithCapacity(t, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://"+host.Addr().String()+"/ws?room=not-a-uuid", nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	if code := readCloseCode(t, conn); code != 1014 {
		t.Fatalf("close code = %d, want 1014", code)
	}
}
