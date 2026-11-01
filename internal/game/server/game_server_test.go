package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	coreserver "github.com/tfkfan/gogame/internal/core/server"
)

type serverEvent struct {
	Type int             `json:"type"`
	Data json.RawMessage `json:"data"`
}

type joinPayload struct {
	PlayerID uuid.UUID `json:"playerId"`
	RoomID   uuid.UUID `json:"roomId"`
}

func newTestServer(t *testing.T, capacity int) *coreserver.Matchmaker {
	t.Helper()

	cfg := &config.Config{
		Port:              "127.0.0.1:0",
		WebSocketEndpoint: "/ws",
		RoomEndpoint:      "/room",
		AssetsDir:         t.TempDir(),
		RoomTimeout:       time.Minute,
		RoomCapacity:      capacity,
	}
	server, errc, err := NewGameServer(cfg)
	if err != nil {
		t.Fatalf("server construction failed: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		select {
		case <-errc:
		default:
		}
	})
	return server
}

func dialQueue(t *testing.T, server *coreserver.Matchmaker) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://"+server.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatalf("queue dial failed: %v", err)
	}
	return conn
}

func dialRoom(t *testing.T, server *coreserver.Matchmaker, roomID uuid.UUID) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://"+server.Addr().String()+"/ws?room="+roomID.String(), nil)
	if err != nil {
		t.Fatalf("room dial failed: %v", err)
	}
	return conn
}

func readEvent(t *testing.T, conn *websocket.Conn) serverEvent {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var event serverEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("invalid event %q: %v", data, err)
	}
	return event
}

func readJoin(t *testing.T, conn *websocket.Conn) joinPayload {
	t.Helper()

	for attempt := 0; attempt < 10; attempt++ {
		event := readEvent(t, conn)
		if event.Type != 4 {
			continue
		}
		var join joinPayload
		if err := json.Unmarshal(event.Data, &join); err != nil {
			t.Fatalf("invalid join payload: %v", err)
		}
		return join
	}
	t.Fatal("join payload did not arrive")
	return joinPayload{}
}

func createDirectRoom(t *testing.T, server *coreserver.Matchmaker) uuid.UUID {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Post("http://"+server.Addr().String()+"/room", "", nil)
	if err != nil {
		t.Fatalf("room creation failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("room response read failed: %v", err)
	}
	id, err := uuid.Parse(string(body))
	if err != nil {
		t.Fatalf("room id %q is not a UUID: %v", body, err)
	}
	return id
}

func TestQueuedPlayersFormRoom(t *testing.T) {
	server := newTestServer(t, 2)

	first := dialQueue(t, server)
	defer func() {
		_ = first.Close(websocket.StatusNormalClosure, "")
	}()
	second := dialQueue(t, server)
	defer func() {
		_ = second.Close(websocket.StatusNormalClosure, "")
	}()

	firstJoin := readJoin(t, first)
	secondJoin := readJoin(t, second)

	if firstJoin.RoomID != secondJoin.RoomID {
		t.Fatalf("rooms = %s and %s, want the same room", firstJoin.RoomID, secondJoin.RoomID)
	}
	if firstJoin.PlayerID == secondJoin.PlayerID {
		t.Fatalf("both players have id %s", firstJoin.PlayerID)
	}
}

func TestQueueDropsDisconnectedPlayer(t *testing.T) {
	server := newTestServer(t, 2)

	first := dialQueue(t, server)
	if err := first.Close(websocket.StatusNormalClosure, "leaving"); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	second := dialQueue(t, server)
	defer func() {
		_ = second.Close(websocket.StatusNormalClosure, "")
	}()
	third := dialQueue(t, server)
	defer func() {
		_ = third.Close(websocket.StatusNormalClosure, "")
	}()

	secondJoin := readJoin(t, second)
	thirdJoin := readJoin(t, third)
	if secondJoin.RoomID != thirdJoin.RoomID {
		t.Fatalf("rooms = %s and %s, want the same room", secondJoin.RoomID, thirdJoin.RoomID)
	}
}

func TestDirectRoomStillJoins(t *testing.T) {
	server := newTestServer(t, 2)

	roomID := createDirectRoom(t, server)
	conn := dialRoom(t, server, roomID)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	join := readJoin(t, conn)
	if join.RoomID != roomID {
		t.Fatalf("joined room %s, want %s", join.RoomID, roomID)
	}
}
