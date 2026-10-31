package room

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/shared"
)

func TestJoinAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := NewGameRoom(ctx, cancel, &config.Config{
		RoomCapacity: 1,
		RoomTimeout:  time.Minute,
	})
	t.Cleanup(func() {
		_ = gr.scheduler.Shutdown(context.Background())
	})

	duplicate := player.NewPlayer(ctx, uuid.New(), nil)
	if !gr.players.Add(duplicate.Id, duplicate) {
		t.Fatal("failed to seed the duplicate player")
	}
	if err := gr.OnJoin(duplicate); !errors.Is(err, ErrPlayerExists) {
		t.Fatalf("duplicate join error = %v, want ErrPlayerExists", err)
	}

	other := player.NewPlayer(ctx, uuid.New(), nil)
	if err := gr.OnJoin(other); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("join at capacity error = %v, want ErrRoomFull", err)
	}

	if got := gr.players.Len(); got != 1 {
		t.Fatalf("membership length = %d, want 1", got)
	}
	if _, ok := gr.players.Get(other.Id); ok {
		t.Fatal("rejected player was added to membership")
	}
}

func TestReapStoppedRemovesOnlyStoppedPlayers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := &GameRoom{
		Id:      uuid.New(),
		ctx:     ctx,
		players: shared.NewSet[uuid.UUID, *player.Player](),
	}

	stopped := player.NewPlayer(ctx, uuid.New(), nil)
	active := player.NewPlayer(ctx, uuid.New(), nil)
	gr.players.Add(stopped.Id, stopped)
	gr.players.Add(active.Id, active)
	stopped.Stop()

	gr.reapStopped()

	if _, ok := gr.players.Get(stopped.Id); ok {
		t.Fatal("stopped player still present after reaping")
	}
	if got, ok := gr.players.Get(active.Id); !ok || got != active {
		t.Fatal("active player was removed by reaping")
	}

	gr.publishSnapshot()
	if msg := active.NextOutbound(); msg == nil {
		t.Fatal("remaining player received no broadcast snapshot")
	}
	if msg := stopped.NextOutbound(); msg != nil {
		t.Fatal("reaped player received a broadcast snapshot")
	}
}
