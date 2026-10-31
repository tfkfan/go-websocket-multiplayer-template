package room

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/spatial"
)

func newSpatialTestRoom(t *testing.T, capacity int) *GameRoom {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	room := NewGameRoom(ctx, cancel, &config.Config{
		RoomCapacity: capacity,
		RoomTimeout:  time.Minute,
	})
	t.Cleanup(func() {
		_ = room.scheduler.Shutdown(context.Background())
		cancel()
	})
	return room
}

func addSpatialPlayer(t *testing.T, room *GameRoom, id uuid.UUID, position player.Vec2, radius float64, keys ...string) *player.Player {
	t.Helper()

	p := player.NewPlayer(room.ctx, id, nil)
	p.Spawn(position, radius, "#ffffff")
	for _, key := range keys {
		p.ApplyKey(key, true)
	}
	if !room.players.Add(p.Id, p) {
		t.Fatalf("failed to add player %s", id)
	}
	return p
}

func playerPosition(p *player.Player) player.Vec2 {
	return p.Snapshot().Position
}

func positionDistance(a, b player.Vec2) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func assertPlayerAt(t *testing.T, p *player.Player, want player.Vec2) {
	t.Helper()
	if got := playerPosition(p); got != want {
		t.Fatalf("player %s at %+v, want %+v", p.Id, got, want)
	}
}

func assertNoOverlap(t *testing.T, players ...*player.Player) {
	t.Helper()
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			first := players[i].Snapshot()
			second := players[j].Snapshot()
			distance := positionDistance(first.Position, second.Position)
			if distance < first.Radius+second.Radius-1e-6 {
				t.Fatalf("players %s and %s overlap: distance %.6f, minimum %.1f",
					players[i].Id, players[j].Id, distance, first.Radius+second.Radius)
			}
		}
	}
}

func assertPlayerInsideWorld(t *testing.T, p *player.Player) {
	t.Helper()
	position := playerPosition(p)
	radius := p.Snapshot().Radius
	if position.X < radius-1e-9 || position.X > config.WorldWidth-radius+1e-9 ||
		position.Y < radius-1e-9 || position.Y > config.WorldHeight-radius+1e-9 {
		t.Fatalf("player at %+v (radius %g) is outside the world", position, radius)
	}
}

func TestSpatialCollisionMovingPlayerKeepsMoving(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	firstStart := player.Vec2{X: 90, Y: 300}
	secondStart := player.Vec2{X: 150, Y: 300}
	first := addSpatialPlayer(t, room, uuid.New(), firstStart, 25, "RIGHT")
	second := addSpatialPlayer(t, room, uuid.New(), secondStart, 25)

	room.OnUpdate(0.1)

	if got := playerPosition(first).X; got <= firstStart.X {
		t.Fatalf("mover X = %g, want movement past its start", got)
	}
	if got := playerPosition(first).X; math.Abs(got-100) > 1e-6 {
		t.Fatalf("mover X = %g, want the contact position 100", got)
	}
	assertPlayerAt(t, second, secondStart)
	assertNoOverlap(t, first, second)
}

func TestSpatialCollisionKeepsTangentialMovement(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	secondStart := player.Vec2{X: 150, Y: 330}
	first := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 90, Y: 300}, 25, "RIGHT", "DOWN")
	second := addSpatialPlayer(t, room, uuid.New(), secondStart, 25)

	room.OnUpdate(0.1)

	position := playerPosition(first)
	if position.X <= 90 {
		t.Fatalf("mover X = %g, want movement to continue", position.X)
	}
	if position.Y <= 300 {
		t.Fatalf("mover Y = %g, want tangential movement to continue", position.Y)
	}
	assertPlayerAt(t, second, secondStart)
	assertNoOverlap(t, first, second)
}

func TestSpatialCollisionLeavesOtherPlayersMoving(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	first := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 90, Y: 300}, 25, "RIGHT")
	second := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 150, Y: 300}, 25)
	third := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 600, Y: 300}, 25, "RIGHT")

	room.OnUpdate(0.1)

	if got := playerPosition(third).X; got <= 600 {
		t.Fatalf("uninvolved player X = %g, want movement to continue", got)
	}
	assertNoOverlap(t, first, second, third)
}

func TestSpatialCollisionKeepsPlayersInsideWorld(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	wallStart := player.Vec2{X: 25, Y: 300}
	wall := addSpatialPlayer(t, room, uuid.New(), wallStart, 25)
	mover := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 80, Y: 300}, 25, "LEFT")

	room.OnUpdate(0.1)

	assertPlayerAt(t, wall, wallStart)
	if got := playerPosition(mover).X; math.Abs(got-75) > 1e-6 {
		t.Fatalf("mover X = %g, want the contact position 75", got)
	}
	assertPlayerInsideWorld(t, wall)
	assertPlayerInsideWorld(t, mover)
	assertNoOverlap(t, wall, mover)
}

func TestSpatialNearestPlayers(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	first := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 100, Y: 100}, 25)
	second := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 200, Y: 100}, 25)
	addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 300, Y: 100}, 25)
	addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 500, Y: 100}, 25)

	room.OnUpdate(0)

	nearest := room.spatial.Nearest(worldVec(120, 100), 0, 2)
	if len(nearest) != 2 {
		t.Fatalf("NearestBodies() returned %d bodies, want 2", len(nearest))
	}
	if nearest[0].Key() != first.Key() || nearest[1].Key() != second.Key() {
		t.Fatalf("NearestBodies() = [%s, %s], want [%s, %s]",
			nearest[0].Key(), nearest[1].Key(), first.Key(), second.Key())
	}

	within := room.spatial.Nearest(worldVec(120, 100), 100, 10)
	if len(within) != 2 {
		t.Fatalf("NearestBodies() within 100 returned %d bodies, want 2", len(within))
	}
}

func TestSpatialReapRemovesPlayerFromIndex(t *testing.T) {
	room := newSpatialTestRoom(t, 8)
	stopped := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 100, Y: 100}, 25)
	active := addSpatialPlayer(t, room, uuid.New(), player.Vec2{X: 200, Y: 100}, 25)
	stopped.Stop()

	room.OnUpdate(0)

	if got := room.spatial.Len(); got != 1 {
		t.Fatalf("spatial index length = %d, want 1", got)
	}
	nearest := room.spatial.Nearest(worldVec(100, 100), 0, 10)
	if len(nearest) != 1 || nearest[0].Key() != active.Key() {
		t.Fatalf("NearestBodies() = %v, want only the active player", bodyKeys(nearest))
	}
}

func TestSpatialCollisionInsertionOrderIndependent(t *testing.T) {
	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		uuid.MustParse("00000000-0000-0000-0000-000000000004"),
	}
	positions := []player.Vec2{
		{X: 200, Y: 300},
		{X: 280, Y: 300},
		{X: 200, Y: 500},
		{X: 280, Y: 500},
	}
	keys := [][]string{
		{"RIGHT"},
		{"LEFT"},
		{"RIGHT"},
		{"LEFT"},
	}

	firstRoom := newSpatialTestRoom(t, 8)
	firstPlayers := make(map[uuid.UUID]*player.Player, len(ids))
	for i, id := range ids {
		firstPlayers[id] = addSpatialPlayer(t, firstRoom, id, positions[i], 25, keys[i]...)
	}
	firstRoom.OnUpdate(0.1)

	secondRoom := newSpatialTestRoom(t, 8)
	secondPlayers := make(map[uuid.UUID]*player.Player, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		secondPlayers[ids[i]] = addSpatialPlayer(t, secondRoom, ids[i], positions[i], 25, keys[i]...)
	}
	secondRoom.OnUpdate(0.1)

	for _, id := range ids {
		firstPosition := playerPosition(firstPlayers[id])
		secondPosition := playerPosition(secondPlayers[id])
		if firstPosition != secondPosition {
			t.Fatalf("player %s ended at %+v in the first order and %+v in the second",
				id, firstPosition, secondPosition)
		}
	}
	assertNoOverlap(t, firstPlayers[ids[0]], firstPlayers[ids[1]])
	assertNoOverlap(t, firstPlayers[ids[2]], firstPlayers[ids[3]])
}

func worldVec(x, y float64) spatial.Vec2 {
	return spatial.Vec2{X: x, Y: y}
}

func bodyKeys(bodies []spatial.Body) []string {
	keys := make([]string, len(bodies))
	for i, body := range bodies {
		keys[i] = body.Key()
	}
	return keys
}
