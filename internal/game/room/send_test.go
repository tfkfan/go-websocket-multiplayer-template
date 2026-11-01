package room

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	coreroom "github.com/tfkfan/gogame/internal/core/room"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
)

func TestSendToOnlyTarget(t *testing.T) {
	room := newTestRoom(t, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})

	firstSession := newFakeSession()
	firstSession.gate = make(chan struct{})
	first := player.New(uuid.New(), player.DefaultProfile(), player.NewHumanController())
	if err := room.Join(coreroom.Member{Entity: first, Session: firstSession}); err != nil {
		t.Fatalf("join failed: %v", err)
	}
	_, secondSession := joinPlayer(t, room, uuid.New(), player.NewHumanController())

	for _, message := range []string{"old", "secret"} {
		if err := room.SendTo(first.Id, event.MsgPrivate, map[string]string{"message": message}); err != nil {
			t.Fatalf("SendTo failed: %v", err)
		}
	}

	room.publishSnapshot()
	close(firstSession.gate)

	waitUntil(t, func() bool {
		return len(firstSession.eventsOfType(t, 110)) > 0 && len(firstSession.eventsOfType(t, 100)) > 0
	})

	private := firstSession.eventsOfType(t, 110)
	if len(private) != 1 {
		t.Fatalf("private frames = %d, want one coalesced frame", len(private))
	}
	var payload map[string]string
	if err := json.Unmarshal(private[0].Data, &payload); err != nil {
		t.Fatalf("invalid private payload %q: %v", private[0].Data, err)
	}
	if payload["message"] != "secret" {
		t.Fatalf("private payload = %q, want the latest message", payload["message"])
	}

	waitUntil(t, func() bool {
		return len(secondSession.eventsOfType(t, 100)) > 0
	})
	if events := secondSession.eventsOfType(t, 110); len(events) != 0 {
		t.Fatalf("second player received %d private frames", len(events))
	}
}

func TestSendToReportsMissingSessionlessAndClosed(t *testing.T) {
	room := newTestRoom(t, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})

	sessionless := player.New(uuid.New(), player.DefaultProfile(), player.NewHumanController())
	if err := room.Join(memberOf(sessionless)); err != nil {
		t.Fatalf("session-less join failed: %v", err)
	}
	if err := room.SendTo(sessionless.Id, event.MsgPrivate, "x"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("session-less send error = %v, want ErrNoSession", err)
	}
	if err := room.SendTo(uuid.New(), event.MsgPrivate, "x"); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("unknown player send error = %v, want ErrPlayerNotFound", err)
	}

	roomCtx, roomCancel := context.WithCancel(context.Background())
	closedRoom := NewGameRoom(roomCtx, roomCancel, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})
	t.Cleanup(func() {
		roomCancel()
		_ = closedRoom.Shutdown()
	})
	roomCancel()
	if err := closedRoom.SendTo(uuid.New(), event.MsgPrivate, "x"); !errors.Is(err, ErrRoomClosed) {
		t.Fatalf("closed room send error = %v, want ErrRoomClosed", err)
	}
}

func TestPublishToConcurrentWithSnapshots(t *testing.T) {
	room := newTestRoom(t, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})

	p, session := joinPlayer(t, room, uuid.New(), player.NewHumanController())

	const calls = 200
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < calls; i++ {
			if err := room.PublishTo(p.Id, snapshot.CustomType, event.MsgPrivate, i); err != nil {
				t.Errorf("PublishTo failed: %v", err)
				return
			}
		}
	}()

	for i := 0; i < calls; i++ {
		room.publishSnapshot()
	}
	<-done

	waitUntil(t, func() bool {
		return len(session.eventsOfType(t, 110)) > 0
	})
}
