package room

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/core/entity"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/spatial"
)

type testObject struct {
	entity.Base
	radius   float64
	velocity spatial.Vec2
	updates  int
}

func newTestObject(id uuid.UUID, position spatial.Vec2, radius float64, velocity spatial.Vec2) *testObject {
	return &testObject{
		Base:     entity.Base{Id: id, Position: position},
		radius:   radius,
		velocity: velocity,
	}
}

func (o *testObject) Shape() spatial.Shape {
	return spatial.Circle{Center: o.Position, Radius: o.radius}
}

func (o *testObject) Update(dt float64) {
	o.updates++
	o.Position = o.Position.Add(o.velocity.Scale(dt))
}

func (o *testObject) Extent() float64 {
	return o.radius
}

func newMixedTestRoom(t *testing.T) *GameRoom {
	t.Helper()
	return newTestRoom(t, &config.Config{
		RoomCapacity: 8,
		RoomTimeout:  time.Minute,
	})
}

func TestNonPlayerMemberIsSimulatedAndExcludedFromUpdate(t *testing.T) {
	room := newMixedTestRoom(t)
	controller := player.NewScriptedController()

	p, session := joinPlayer(t, room, uuid.New(), controller)
	p.Spawn(room.world, spatial.Vec2{X: 400, Y: 300}, "#ffffff")

	object := newTestObject(uuid.New(), spatial.Vec2{X: 100, Y: 100}, 25, spatial.Vec2{X: 50})
	if err := room.Join(memberOf(object)); err != nil {
		t.Fatalf("object join failed: %v", err)
	}

	room.OnTick(0.1)

	if object.updates != 1 {
		t.Fatalf("object updates = %d, want exactly one per tick", object.updates)
	}
	if object.Position.X <= 100 {
		t.Fatalf("object x = %g, want it to advance", object.Position.X)
	}

	waitUntil(t, func() bool {
		return len(session.eventsOfType(t, 100)) > 0
	})
	updates := session.eventsOfType(t, 100)
	var payload struct {
		Players []struct {
			ID string `json:"id"`
		} `json:"players"`
	}
	if err := json.Unmarshal(updates[len(updates)-1].Data, &payload); err != nil {
		t.Fatalf("invalid update payload: %v", err)
	}
	if len(payload.Players) != 1 || payload.Players[0].ID != p.Id.String() {
		t.Fatalf("update players = %+v, want only the player", payload.Players)
	}

	if err := room.PublishTo(object.Id, snapshot.CustomType, event.MsgPrivate, "x"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("object send error = %v, want ErrNoSession", err)
	}
}

func TestNonPlayerMoverIsCorrectedAgainstPlayer(t *testing.T) {
	room := newMixedTestRoom(t)
	controller := player.NewScriptedController()

	p, _ := joinPlayer(t, room, uuid.New(), controller)
	p.Spawn(room.world, spatial.Vec2{X: 400, Y: 300}, "#ffffff")

	object := newTestObject(uuid.New(), spatial.Vec2{X: 340, Y: 300}, 25, spatial.Vec2{X: 200})
	if err := room.Join(memberOf(object)); err != nil {
		t.Fatalf("object join failed: %v", err)
	}

	room.OnTick(0.1)

	playerState := p.Snapshot()
	distance := math.Hypot(object.Position.X-playerState.Position.X, object.Position.Y-playerState.Position.Y)
	if distance < playerState.Radius+object.radius-1e-6 {
		t.Fatalf("objects overlap after the tick: distance %.6f", distance)
	}
	if playerState.Position.X != 400 || playerState.Position.Y != 300 {
		t.Fatalf("stationary player moved to %+v", playerState.Position)
	}
}

func TestMixedMembersAdvanceOncePerTickDeterministically(t *testing.T) {
	first := mixedTickPositions(t, false)
	second := mixedTickPositions(t, true)

	if len(first) != len(second) {
		t.Fatalf("member counts differ: %d and %d", len(first), len(second))
	}
	for key, position := range first {
		other, ok := second[key]
		if !ok {
			t.Fatalf("member %s missing from the second room", key)
		}
		if position != other {
			t.Fatalf("member %s at %+v and %+v", key, position, other)
		}
	}
}

func mixedTickPositions(t *testing.T, reversed bool) map[string]spatial.Vec2 {
	t.Helper()

	room := newMixedTestRoom(t)
	playerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	objectID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	controller := player.NewScriptedController()
	controller.SetDirection(1, 0)
	p := player.New(playerID, player.DefaultProfile(), controller)
	object := newTestObject(objectID, spatial.Vec2{X: 300, Y: 500}, 25, spatial.Vec2{X: 100})

	if reversed {
		if err := room.Join(memberOf(object)); err != nil {
			t.Fatalf("object join failed: %v", err)
		}
		if err := room.Join(memberOf(p)); err != nil {
			t.Fatalf("player join failed: %v", err)
		}
	} else {
		if err := room.Join(memberOf(p)); err != nil {
			t.Fatalf("player join failed: %v", err)
		}
		if err := room.Join(memberOf(object)); err != nil {
			t.Fatalf("object join failed: %v", err)
		}
	}
	p.Spawn(room.world, spatial.Vec2{X: 100, Y: 500}, "#ffffff")

	room.OnTick(0.1)
	room.OnTick(0.1)

	return map[string]spatial.Vec2{
		p.Key():      p.Position,
		object.Key(): object.Position,
	}
}
