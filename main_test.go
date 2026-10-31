package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"go.uber.org/goleak"
)

const testServer = "http://localhost:8080"

type testEvent struct {
	Type int             `json:"type"`
	Data json.RawMessage `json:"data"`
}

type writeEvent struct {
	Type int `json:"type"`
	Data any `json:"data"`
}

type keyPayload struct {
	Key   string `json:"key"`
	State bool   `json:"state"`
}

type positionView struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type playerView struct {
	ID       string       `json:"id"`
	Position positionView `json:"position"`
	Radius   float64      `json:"radius"`
	Color    string       `json:"color"`
}

type updateView struct {
	Players []playerView `json:"players"`
	Width   float64      `json:"width"`
	Height  float64      `json:"height"`
	Tick    uint64       `json:"tick"`
}

type joinView struct {
	PlayerID string  `json:"playerId"`
	RoomID   string  `json:"roomId"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Radius   float64 `json:"radius"`
	Color    string  `json:"color"`
	TickRate int     `json:"tickRate"`
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestLeaks(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := run(ctx); err != nil &&
		!errors.Is(err, http.ErrServerClosed) &&
		!errors.Is(err, context.Canceled) {
		t.Errorf("App failed: %v", err)
	}
}

func TestRoomGameState(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errc := startServer(t, ctx)

	client := newTestClient(2 * time.Second)
	roomID, err := createGameRoom(client)
	if err != nil {
		cancel()
		t.Fatalf("failed to create game room: %v", err)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	conn, _, err := websocket.Dial(dialCtx, "ws://localhost:8080/ws?room="+roomID.String(), nil)
	dialCancel()
	if err != nil {
		cancel()
		t.Fatalf("dial failed: %v", err)
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "test complete")
	}()

	join := readJoin(t, ctx, conn)
	update := readUpdate(t, ctx, conn)
	if update.Width <= 0 || update.Height <= 0 {
		t.Fatalf("invalid world size: %vx%v", update.Width, update.Height)
	}
	if len(update.Players) == 0 {
		t.Fatal("update contains no players")
	}
	self, ok := findPlayer(update, join.PlayerID)
	if !ok {
		t.Fatalf("self %q missing from update", join.PlayerID)
	}
	if self.Radius <= 0 {
		t.Fatalf("invalid self state: %+v", self)
	}

	dialCtx, dialCancel = context.WithTimeout(ctx, 3*time.Second)
	conn2, _, err := websocket.Dial(dialCtx, "ws://localhost:8080/ws?room="+roomID.String(), nil)
	dialCancel()
	if err != nil {
		cancel()
		t.Fatalf("second dial failed: %v", err)
	}
	defer func() {
		_ = conn2.Close(websocket.StatusNormalClosure, "test complete")
	}()

	join2 := readJoin(t, ctx, conn2)
	update2 := readUpdate(t, ctx, conn2)
	if len(update2.Players) < 2 {
		t.Fatalf("expected two players, got %d", len(update2.Players))
	}
	if join2.PlayerID == "" || join2.PlayerID == join.PlayerID {
		t.Fatalf("second player has invalid id %q", join2.PlayerID)
	}
	if _, ok := findPlayer(update2, join2.PlayerID); !ok {
		t.Fatalf("second self %q missing from update", join2.PlayerID)
	}

	update = readUpdate(t, ctx, conn)
	if len(update.Players) < 2 {
		t.Fatalf("first client did not observe the second player: %d", len(update.Players))
	}
	self, ok = findPlayer(update, join.PlayerID)
	if !ok {
		t.Fatalf("self %q missing after second join", join.PlayerID)
	}

	startX := self.Position.X
	writeCtx, writeCancel := context.WithTimeout(ctx, time.Second)
	err = wsjson.Write(writeCtx, conn, writeEvent{
		Type: 200,
		Data: keyPayload{Key: "RIGHT", State: true},
	})
	writeCancel()
	if err != nil {
		cancel()
		t.Fatalf("failed to send key event: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	moved := false
	for time.Now().Before(deadline) {
		next := readUpdate(t, ctx, conn)
		nextSelf, ok := findPlayer(next, join.PlayerID)
		if ok && nextSelf.Position.X > startX+1 {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatalf("player did not move right from x=%v", startX)
	}

	writeCtx, writeCancel = context.WithTimeout(ctx, time.Second)
	_ = wsjson.Write(writeCtx, conn, writeEvent{
		Type: 200,
		Data: keyPayload{Key: "RIGHT", State: false},
	})
	writeCancel()

	_ = conn.Close(websocket.StatusNormalClosure, "test complete")
	stopServer(t, cancel, errc)
}

func TestRoomGameStateCollision(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errc := startServer(t, ctx)

	client := newTestClient(2 * time.Second)
	roomID, err := createGameRoom(client)
	if err != nil {
		cancel()
		t.Fatalf("failed to create game room: %v", err)
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	firstConn, _, err := websocket.Dial(dialCtx, "ws://localhost:8080/ws?room="+roomID.String(), nil)
	dialCancel()
	if err != nil {
		cancel()
		t.Fatalf("first dial failed: %v", err)
	}
	defer func() {
		_ = firstConn.Close(websocket.StatusNormalClosure, "test complete")
	}()

	firstJoin := readJoin(t, ctx, firstConn)
	if _, ok := findPlayer(readUpdate(t, ctx, firstConn), firstJoin.PlayerID); !ok {
		t.Fatal("first player missing from its first update")
	}

	dialCtx, dialCancel = context.WithTimeout(ctx, 3*time.Second)
	secondConn, _, err := websocket.Dial(dialCtx, "ws://localhost:8080/ws?room="+roomID.String(), nil)
	dialCancel()
	if err != nil {
		cancel()
		t.Fatalf("second dial failed: %v", err)
	}
	defer func() {
		_ = secondConn.Close(websocket.StatusNormalClosure, "test complete")
	}()

	secondJoin := readJoin(t, ctx, secondConn)
	readUpdate(t, ctx, secondConn)

	update := readUpdateWithPlayers(t, ctx, firstConn, firstJoin.PlayerID, secondJoin.PlayerID)
	first, _ := findPlayer(update, firstJoin.PlayerID)
	second, _ := findPlayer(update, secondJoin.PlayerID)

	dx := second.Position.X - first.Position.X
	dy := second.Position.Y - first.Position.Y
	firstKeys := keyList(dx, "RIGHT", "LEFT")
	secondKeys := keyList(-dx, "RIGHT", "LEFT")
	firstKeys = append(firstKeys, keyList(dy, "DOWN", "UP")...)
	secondKeys = append(secondKeys, keyList(-dy, "DOWN", "UP")...)

	for _, key := range firstKeys {
		writeKeyEvent(t, ctx, firstConn, key, true)
	}
	for _, key := range secondKeys {
		writeKeyEvent(t, ctx, secondConn, key, true)
	}

	radius := firstJoin.Radius
	minDistance := math.Inf(1)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		next := readUpdate(t, ctx, firstConn)
		firstPlayer, firstOK := findPlayer(next, firstJoin.PlayerID)
		secondPlayer, secondOK := findPlayer(next, secondJoin.PlayerID)
		if !firstOK || !secondOK {
			continue
		}

		distance := math.Hypot(
			secondPlayer.Position.X-firstPlayer.Position.X,
			secondPlayer.Position.Y-firstPlayer.Position.Y,
		)
		if distance < minDistance {
			minDistance = distance
		}
		if distance < 2*radius-1 {
			t.Fatalf("players overlap in UPDATE: distance %.3f, radius %.1f", distance, radius)
		}
		assertPlayerInWorld(t, firstPlayer, next)
		assertPlayerInWorld(t, secondPlayer, next)

		if minDistance <= 2*radius+35 {
			break
		}
	}

	for _, key := range firstKeys {
		writeKeyEvent(t, ctx, firstConn, key, false)
	}
	for _, key := range secondKeys {
		writeKeyEvent(t, ctx, secondConn, key, false)
	}

	if minDistance > 2*radius+35 {
		t.Fatalf("players never approached: minimum distance %.3f", minDistance)
	}

	_ = firstConn.Close(websocket.StatusNormalClosure, "test complete")
	_ = secondConn.Close(websocket.StatusNormalClosure, "test complete")
	stopServer(t, cancel, errc)
}

func keyList(delta float64, positive, negative string) []string {
	switch {
	case delta > 10:
		return []string{positive}
	case delta < -10:
		return []string{negative}
	default:
		return nil
	}
}

func writeKeyEvent(t *testing.T, ctx context.Context, conn *websocket.Conn, key string, state bool) {
	t.Helper()

	writeCtx, writeCancel := context.WithTimeout(ctx, time.Second)
	defer writeCancel()
	if err := wsjson.Write(writeCtx, conn, writeEvent{
		Type: 200,
		Data: keyPayload{Key: key, State: state},
	}); err != nil {
		t.Fatalf("failed to write key event %s: %v", key, err)
	}
}

func readUpdateWithPlayers(t *testing.T, ctx context.Context, conn *websocket.Conn, firstID, secondID string) updateView {
	t.Helper()

	for i := 0; i < 30; i++ {
		update := readUpdate(t, ctx, conn)
		if _, firstOK := findPlayer(update, firstID); !firstOK {
			continue
		}
		if _, secondOK := findPlayer(update, secondID); !secondOK {
			continue
		}
		return update
	}
	t.Fatal("update with both players did not arrive")
	return updateView{}
}

func assertPlayerInWorld(t *testing.T, p playerView, update updateView) {
	t.Helper()

	if p.Position.X < p.Radius || p.Position.X > update.Width-p.Radius ||
		p.Position.Y < p.Radius || p.Position.Y > update.Height-p.Radius {
		t.Fatalf("player %s at %+v is outside the %gx%g world", p.ID, p.Position, update.Width, update.Height)
	}
}

func startServer(t *testing.T, ctx context.Context) chan error {
	t.Helper()

	errc := make(chan error, 1)
	go func() {
		errc <- run(ctx)
	}()

	client := newTestClient(500 * time.Millisecond)
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := client.Get(testServer + "/")
		if err == nil {
			_ = resp.Body.Close()
			return errc
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func stopServer(t *testing.T, cancel context.CancelFunc, errc chan error) {
	t.Helper()
	cancel()

	select {
	case err := <-errc:
		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, context.Canceled) {
			t.Fatalf("server failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func readJoin(t *testing.T, ctx context.Context, conn *websocket.Conn) joinView {
	t.Helper()

	for {
		messageType, data := readEvent(t, ctx, conn)
		if messageType != 4 {
			continue
		}

		var join joinView
		if err := json.Unmarshal(data, &join); err != nil {
			t.Fatalf("invalid join payload %q: %v", data, err)
		}
		return join
	}
}

func readUpdate(t *testing.T, ctx context.Context, conn *websocket.Conn) updateView {
	t.Helper()

	for {
		messageType, data := readEvent(t, ctx, conn)
		if messageType != 100 {
			continue
		}

		var update updateView
		if err := json.Unmarshal(data, &update); err != nil {
			t.Fatalf("invalid update payload %q: %v", data, err)
		}
		return update
	}
}

func readEvent(t *testing.T, ctx context.Context, conn *websocket.Conn) (int, json.RawMessage) {
	t.Helper()

	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	_, data, err := conn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var event testEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("invalid server event %q: %v", data, err)
	}
	return event.Type, event.Data
}

func findPlayer(update updateView, id string) (playerView, bool) {
	for _, p := range update.Players {
		if p.ID == id {
			return p, true
		}
	}
	return playerView{}, false
}

func newTestClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

func createGameRoom(client *http.Client) (uuid.UUID, error) {
	req, err := http.NewRequest(http.MethodPost, testServer+"/room", nil)
	if err != nil {
		return uuid.UUID{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return uuid.UUID{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.UUID{}, err
	}

	return uuid.Parse(string(body))
}
