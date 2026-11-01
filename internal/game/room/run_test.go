package room

import (
	"context"
	"testing"
	"time"

	"github.com/tfkfan/gogame/internal/config"
	"go.uber.org/goleak"
)

func TestRoomTimeoutClosesRoom(t *testing.T) {
	defer goleak.VerifyNone(t)

	roomCtx, roomCancel := context.WithCancel(context.Background())
	defer roomCancel()

	gr := NewGameRoom(roomCtx, roomCancel, &config.Config{
		RoomTimeout:  30 * time.Millisecond,
		RoomCapacity: 8,
	})

	closed := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		gr.Run(context.Background(), func(room *BaseRoom) {
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
		t.Fatal("room actor did not stop after close")
	}
}
