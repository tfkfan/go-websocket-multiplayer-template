package room

import (
	"context"
	"log"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/entity"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/shared"
	"github.com/tfkfan/gogame/internal/spatial"
)

var worldRect = spatial.Rect{
	Min: spatial.Vec2{X: 0, Y: 0},
	Max: spatial.Vec2{X: config.WorldWidth, Y: config.WorldHeight},
}

var palette = []string{
	"#e74c3c", "#3498db", "#2ecc71", "#f1c40f",
	"#9b59b6", "#1abc9c", "#e67e22", "#ec4899",
}

type GameRoom struct {
	BaseRoom
	spawner *Spawner
}

func NewGameRoom(gameRoomContext context.Context, gameRoomCancel context.CancelFunc, srvCfg *config.Config) *GameRoom {
	r := &GameRoom{
		Id:        uuid.New(),
		cfg:       srvCfg,
		commands:  make(chan Command, commandBuffer),
		players:   shared.NewSet[uuid.UUID, *player.Player](),
		spatial:   spatial.NewQuadtree(worldRect, spatial.Options{}),
		ctx:       gameRoomContext,
		ctxCancel: gameRoomCancel,
		scheduler: shared.NewScheduler(),
		world: entity.World{
			Width:  config.WorldWidth,
			Height: config.WorldHeight,
		},
	}
	r.spawner = NewSpawner(r)
	r.BaseRoom.IRoom = r
	r.BaseRoom.CollisionHandler = r

	log.Printf("created game room %s (capacity %d)", r.Id, r.cfg.RoomCapacity)
	return r
}

func (gr *GameRoom) OnJoin(p *player.Player) error {
	if gr.closed.Load() || gr.ctx.Err() != nil {
		return ErrRoomClosed
	}

	if _, exists := gr.players.Get(p.Id); exists {
		return ErrPlayerExists
	}
	if gr.players.Len() >= gr.cfg.RoomCapacity {
		return ErrRoomFull
	}

	slot := gr.spawner.nextSlot()
	p.Spawn(gr.world, gr.spawner.slotPosition(slot), config.PlayerRadius, palette[slot%len(palette)])
	if !gr.players.Add(p.Id, p) {
		return ErrPlayerExists
	}

	state := p.Snapshot()
	msg, err := event.MarshalEvent(event.MsgJoinSuccess, event.JoinSuccessData{
		PlayerID: p.Id,
		RoomID:   gr.Id,
		Width:    gr.world.Width,
		Height:   gr.world.Height,
		Radius:   state.Radius,
		Color:    state.Color,
		TickRate: config.TickRate,
	})
	if err != nil {
		gr.players.Remove(p.Id)
		return err
	}

	p.Publish(entity.SystemType, msg)
	go gr.writeLoop(p)

	return nil
}

func (gr *GameRoom) OnDisconnect(p *player.Player) error {
	gr.players.Remove(p.Id)
	p.Stop()
	return nil
}

func (gr *GameRoom) OnUpdate(dt float64) {
	gr.reapStopped()
	moved := gr.movePlayers(dt)
	gr.refreshSpatial()
	gr.handleCollisions(moved)
	gr.publishSnapshot()
}

func (gr *GameRoom) movePlayers(dt float64) map[string]int {
	players := gr.players.List()
	sort.Slice(players, func(i, j int) bool {
		return players[i].Key() < players[j].Key()
	})

	moved := make(map[string]int, len(players))
	for idx, p := range players {
		before := p.Snapshot()
		p.Update(dt)
		current := p.Snapshot()
		if current.Position != before.Position {
			moved[p.Key()] = idx
		}
	}
	return moved
}

func (gr *GameRoom) handleCollision(body spatial.Body, normal spatial.Vec2, depth float64) {
	p, ok := body.(*player.Player)
	if !ok {
		return
	}

	center := p.Shape().CenterPoint()
	target := center.Add(normal.Scale(depth))
	radius := p.Snapshot().Radius
	target.X = math.Min(math.Max(target.X, radius), gr.world.Width-radius)
	target.Y = math.Min(math.Max(target.Y, radius), gr.world.Height-radius)
	p.MoveTo(spatial.Vec2{X: target.X, Y: target.Y})
}
