package room

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	coreroom "github.com/tfkfan/gogame/internal/core/room"
	"github.com/tfkfan/gogame/internal/game/player"
)

func TestJoinAdmission(t *testing.T) {
	room := newTestRoom(t, &config.Config{
		RoomCapacity: 1,
		RoomTimeout:  time.Minute,
	})

	duplicate, _ := joinPlayer(t, room, uuid.New(), player.NewHumanController())

	if err := room.Join(coreroom.Member{Entity: duplicate}); !errors.Is(err, ErrPlayerExists) {
		t.Fatalf("duplicate join error = %v, want ErrPlayerExists", err)
	}

	other := player.New(uuid.New(), player.DefaultProfile(), player.NewHumanController())
	if err := room.Join(coreroom.Member{Entity: other}); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("join at capacity error = %v, want ErrRoomFull", err)
	}

	if room.Len() != 1 {
		t.Fatalf("membership length = %d, want 1", room.Len())
	}
	if _, ok := room.Member(other.Key()); ok {
		t.Fatal("rejected player was added to membership")
	}
}

func TestReapedPlayerLeavesMembership(t *testing.T) {
	room := newTestRoom(t, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})

	stopped, stoppedSession := joinPlayer(t, room, uuid.New(), player.NewHumanController())
	active, activeSession := joinPlayer(t, room, uuid.New(), player.NewHumanController())

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(runCtx, func(*coreroom.Room) {})
	}()

	stoppedSession.Stop()

	waitUntil(t, func() bool {
		_, ok := room.Member(stopped.Key())
		return !ok
	})
	if _, ok := room.Member(active.Key()); !ok {
		t.Fatal("active player was reaped")
	}

	waitUntil(t, func() bool {
		return len(activeSession.eventsOfType(t, 100)) > 0
	})
	if events := stoppedSession.eventsOfType(t, 100); len(events) > 0 {
		t.Fatalf("reaped player received %d update frames", len(events))
	}

	runCancel()
	<-done
}
