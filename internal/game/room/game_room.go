package room

import (
	"context"
	"errors"
	"log"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/core/entity"
	coreroom "github.com/tfkfan/gogame/internal/core/room"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/core/spatial"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
)

var (
	ErrRoomClosed     = coreroom.ErrClosed
	ErrPlayerExists   = coreroom.ErrExists
	ErrPlayerNotFound = coreroom.ErrNotFound
	ErrNoSession      = coreroom.ErrNoSession
	ErrRoomFull       = errors.New("game room is full")
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
	*coreroom.Room

	cfg     *config.Config
	world   entity.World
	spatial *spatial.Quadtree
	spawner *Spawner
}

type contributor interface {
	Contribute() (any, bool)
}

type correctable interface {
	spatial.Body
	MoveTo(spatial.Vec2)
	Extent() float64
}

func NewGameRoom(gameRoomContext context.Context, gameRoomCancel context.CancelFunc, srvCfg *config.Config) *GameRoom {
	gr := &GameRoom{
		cfg:     srvCfg,
		world:   entity.World{Width: config.WorldWidth, Height: config.WorldHeight},
		spatial: spatial.NewQuadtree(worldRect, spatial.Options{}),
	}
	gr.Room = coreroom.New(gameRoomContext, gameRoomCancel, coreroom.Config{
		Id:           uuid.New(),
		Timeout:      srvCfg.RoomTimeout,
		TickInterval: time.Second / time.Duration(config.TickRate),
	}, gr)
	gr.spawner = NewSpawner(gr)
	return gr
}

func (gr *GameRoom) OnJoin(member coreroom.Member) error {
	p, ok := member.Entity.(*player.Player)
	if !ok {
		return nil
	}
	if gr.playerCount() > gr.cfg.RoomCapacity {
		return ErrRoomFull
	}

	slot := gr.spawner.nextSlot()
	p.Spawn(gr.world, gr.spawner.slotPosition(slot), palette[slot%len(palette)])

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
		return err
	}
	if member.Session != nil {
		member.Session.Publish(snapshot.SystemType, msg)
	}
	return nil
}

func (gr *GameRoom) OnLeave(coreroom.Member) {
}

func (gr *GameRoom) OnTick(dt float64) {
	moved := gr.moveMembers(dt)
	gr.refreshSpatial()
	gr.handleCollisions(moved)
	gr.publishSnapshot()
}

func (gr *GameRoom) SendTo(playerID uuid.UUID, messageType event.MessageType, data any) error {
	return gr.PublishTo(playerID, snapshot.PrivateType, messageType, data)
}

func (gr *GameRoom) PublishTo(playerID uuid.UUID, kind snapshot.SnapshotType, messageType event.MessageType, data any) error {
	msg, err := event.MarshalEvent(messageType, data)
	if err != nil {
		return err
	}
	return gr.Room.PublishTo(playerID.String(), kind, msg)
}

func (gr *GameRoom) players() []*player.Player {
	members := gr.Members()
	players := make([]*player.Player, 0, len(members))
	for _, member := range members {
		if p, ok := member.Entity.(*player.Player); ok {
			players = append(players, p)
		}
	}
	return players
}

func (gr *GameRoom) playerCount() int {
	return len(gr.players())
}

func (gr *GameRoom) moveMembers(dt float64) map[string]int {
	members := gr.Members()
	sort.Slice(members, func(i, j int) bool {
		return members[i].Entity.Key() < members[j].Entity.Key()
	})

	moved := make(map[string]int, len(members))
	for index, member := range members {
		before := member.Entity.Shape().CenterPoint()
		member.Entity.Update(dt)
		after := member.Entity.Shape().CenterPoint()
		if after != before {
			moved[member.Entity.Key()] = index
		}
	}
	return moved
}

func (gr *GameRoom) refreshSpatial() {
	members := gr.Members()
	bodies := make([]spatial.Body, 0, len(members))
	for _, member := range members {
		bodies = append(bodies, member.Entity)
	}
	gr.spatial.Rebuild(bodies)
}

func (gr *GameRoom) handleCollisions(moved map[string]int) {
	spatial.DetectCollisions(gr.spatial, func(contact spatial.Contact) {
		aIndex, aMoved := moved[contact.A.Key()]
		bIndex, bMoved := moved[contact.B.Key()]

		switch {
		case !aMoved && !bMoved:
			return
		case aMoved && bMoved && aIndex > bIndex:
			gr.correct(contact.A, contact.Normal, contact.Depth)
		case aMoved && bMoved:
			gr.correct(contact.B, contact.Normal.Scale(-1), contact.Depth)
		case aMoved:
			gr.correct(contact.A, contact.Normal, contact.Depth)
		default:
			gr.correct(contact.B, contact.Normal.Scale(-1), contact.Depth)
		}
	})
}

func (gr *GameRoom) correct(body spatial.Body, normal spatial.Vec2, depth float64) {
	mover, ok := body.(correctable)
	if !ok {
		return
	}

	center := mover.Shape().CenterPoint()
	target := center.Add(normal.Scale(depth))
	extent := mover.Extent()
	target.X = math.Min(math.Max(target.X, extent), gr.world.Width-extent)
	target.Y = math.Min(math.Max(target.Y, extent), gr.world.Height-extent)
	mover.MoveTo(spatial.Vec2{X: target.X, Y: target.Y})
}

func (gr *GameRoom) publishSnapshot() {
	members := gr.Members()
	if len(members) == 0 {
		return
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].Entity.Key() < members[j].Entity.Key()
	})

	states := make([]player.State, 0, len(members))
	for _, member := range members {
		source, ok := member.Entity.(contributor)
		if !ok {
			continue
		}
		value, ok := source.Contribute()
		if !ok {
			continue
		}
		state, ok := value.(player.State)
		if !ok {
			continue
		}
		states = append(states, state)
	}
	if len(states) == 0 {
		return
	}

	msg, err := event.MarshalEvent(event.MsgUpdate, event.UpdateData{
		Players: states,
		Width:   gr.world.Width,
		Height:  gr.world.Height,
	})
	if err != nil {
		log.Printf("failed to marshal room update for %s: %v", gr.Id, err)
		return
	}

	gr.Broadcast(snapshot.BroadcastType, msg)
}

func (gr *GameRoom) RoomID() uuid.UUID {
	return gr.Id
}

func (gr *GameRoom) RunRoom(ctx context.Context, onClose func()) {
	gr.Run(ctx, func(*coreroom.Room) {
		onClose()
	})
}
