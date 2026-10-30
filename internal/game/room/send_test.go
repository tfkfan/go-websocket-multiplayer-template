package room

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/game/player"
)

func TestSendToOnlyTarget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := &GameRoom{
		Id:      uuid.New(),
		ctx:     ctx,
		players: newPlayerSet(),
	}

	first := player.NewPlayer(ctx, uuid.New(), nil)
	second := player.NewPlayer(ctx, uuid.New(), nil)
	if !gr.players.add(first) || !gr.players.add(second) {
		t.Fatal("failed to seed players")
	}

	for _, message := range []string{"old", "secret"} {
		if err := gr.SendTo(first.Id, MsgPrivate, map[string]string{"message": message}); err != nil {
			t.Fatalf("SendTo failed: %v", err)
		}
	}

	gr.publishSnapshot()

	privateMsg := first.NextOutbound()
	if privateMsg == nil {
		t.Fatal("target player received no private frame")
	}

	var privateEvent struct {
		Type MessageType `json:"type"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(privateMsg, &privateEvent); err != nil {
		t.Fatalf("invalid private event: %v", err)
	}
	if privateEvent.Type != MsgPrivate || privateEvent.Data.Message != "secret" {
		t.Fatalf("unexpected private event: %+v", privateEvent)
	}

	firstUpdate := first.NextOutbound()
	if firstUpdate == nil {
		t.Fatal("target player received no update")
	}
	var firstEnvelope struct {
		Type MessageType `json:"type"`
	}
	if err := json.Unmarshal(firstUpdate, &firstEnvelope); err != nil {
		t.Fatalf("invalid target update: %v", err)
	}
	if firstEnvelope.Type != MsgUpdate {
		t.Fatalf("unexpected target update: %+v", firstEnvelope)
	}
	if extra := first.NextOutbound(); extra != nil {
		t.Fatal("expected only one private and one update frame")
	}

	secondMsg := second.NextOutbound()
	if secondMsg == nil {
		t.Fatal("non-target player received no update")
	}
	var secondEnvelope struct {
		Type MessageType `json:"type"`
	}
	if err := json.Unmarshal(secondMsg, &secondEnvelope); err != nil {
		t.Fatalf("invalid non-target update: %v", err)
	}
	if secondEnvelope.Type != MsgUpdate {
		t.Fatalf("unexpected non-target update: %+v", secondEnvelope)
	}
	if extra := second.NextOutbound(); extra != nil {
		t.Fatal("non-target player received a private frame")
	}

	if err := gr.SendTo(uuid.New(), MsgPrivate, nil); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound, got %v", err)
	}

	first.Stop()
	if err := gr.SendTo(first.Id, MsgPrivate, nil); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound for stopped player, got %v", err)
	}

	cancel()
	if err := gr.SendTo(second.Id, MsgPrivate, nil); !errors.Is(err, ErrRoomClosed) {
		t.Fatalf("expected ErrRoomClosed for closed room, got %v", err)
	}
}

func TestPublishToConcurrentWithSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gr := &GameRoom{
		Id:      uuid.New(),
		ctx:     ctx,
		players: newPlayerSet(),
	}
	p := player.NewPlayer(ctx, uuid.New(), nil)
	gr.players.add(p)

	const calls = 200
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < calls; i++ {
			if err := gr.PublishTo(p.Id, player.CustomType, MsgPrivate, i); err != nil {
				t.Errorf("PublishTo failed: %v", err)
				return
			}
		}
	}()

	for i := 0; i < calls; i++ {
		gr.publishSnapshot()
		_ = p.NextOutbound()
	}
	<-done
}
