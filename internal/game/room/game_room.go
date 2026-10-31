package room

import (
	"context"
	"errors"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/shared"
	"github.com/tfkfan/gogame/internal/spatial"
)

const (
	commandBuffer = 64
)

var worldRect = spatial.Rect{
	Min: spatial.Vec2{X: 0, Y: 0},
	Max: spatial.Vec2{X: config.WorldWidth, Y: config.WorldHeight},
}

var (
	ErrRoomClosed     = errors.New("game room is closed")
	ErrRoomFull       = errors.New("game room is full")
	ErrPlayerExists   = errors.New("player already connected")
	ErrPlayerNotFound = errors.New("player not found")
)

var palette = []string{
	"#e74c3c", "#3498db", "#2ecc71", "#f1c40f",
	"#9b59b6", "#1abc9c", "#e67e22", "#ec4899",
}

type GameRoom struct {
	GameRoomLifecycle

	Id uuid.UUID

	closed    atomic.Bool
	cfg       *config.Config
	ctx       context.Context
	ctxCancel context.CancelFunc

	commands  chan Command
	players   *shared.Set[uuid.UUID, *player.Player]
	spatial   *spatial.Quadtree
	scheduler *shared.Scheduler
	spawner   *Spawner
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
	}
	r.spawner = NewSpawner(r)

	log.Printf("created game room %s (capacity %d)", r.Id, r.cfg.RoomCapacity)

	return r
}

func (gr *GameRoom) Context() context.Context {
	return gr.ctx
}

func (gr *GameRoom) SendTo(playerID uuid.UUID, messageType MessageType, data any) error {
	return gr.PublishTo(playerID, player.PrivateType, messageType, data)
}

func (gr *GameRoom) PublishTo(playerID uuid.UUID, kind player.SnapshotType, messageType MessageType, data any) error {
	if gr.closed.Load() || gr.ctx.Err() != nil {
		return ErrRoomClosed
	}

	p, ok := gr.players.Get(playerID)
	if !ok {
		if gr.closed.Load() {
			return ErrRoomClosed
		}
		return ErrPlayerNotFound
	}

	select {
	case <-p.Done():
		if gr.closed.Load() {
			return ErrRoomClosed
		}
		return ErrPlayerNotFound
	default:
	}

	msg, err := marshalEvent(messageType, data)
	if err != nil {
		return err
	}

	p.Publish(kind, msg)
	return nil
}

func (gr *GameRoom) SubmitCommand(command Command) error {
	select {
	case gr.commands <- command:
		return nil
	case <-gr.ctx.Done():
		return ErrRoomClosed
	}
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
	p.Spawn(gr.spawner.slotPosition(slot), config.PlayerRadius, palette[slot%len(palette)])
	if !gr.players.Add(p.Id, p) {
		return ErrPlayerExists
	}

	state := p.Snapshot()
	msg, err := marshalEvent(MsgJoinSuccess, JoinSuccessData{
		PlayerID: p.Id,
		RoomID:   gr.Id,
		Width:    config.WorldWidth,
		Height:   config.WorldHeight,
		Radius:   state.Radius,
		Color:    state.Color,
		TickRate: config.TickRate,
	})
	if err != nil {
		gr.players.Remove(p.Id)
		return err
	}

	p.Publish(player.SystemType, msg)
	go gr.writeLoop(p)

	return nil
}

func (gr *GameRoom) OnDisconnect(p *player.Player) error {
	gr.players.Remove(p.Id)
	p.Stop()
	return nil
}

func (gr *GameRoom) reapStopped() {
	for _, p := range gr.players.List() {
		select {
		case <-p.Done():
			gr.players.Remove(p.Id)
		default:
		}
	}
}

func (gr *GameRoom) movePlayers(dt float64) map[string]int {
	players := gr.players.List()
	sort.Slice(players, func(i, j int) bool {
		return players[i].Key() < players[j].Key()
	})

	moved := make(map[string]int, len(players))
	for order, p := range players {
		before := p.Snapshot()
		p.Update(dt, config.WorldWidth, config.WorldHeight)
		current := p.Snapshot()
		if current.Position != before.Position {
			moved[p.Key()] = order
		}
	}
	return moved
}

func (gr *GameRoom) handleCollisions(moved map[string]int) {
	spatial.DetectCollisions(gr.spatial, func(contact spatial.Contact) {
		aOrder, aMoved := moved[contact.A.Key()]
		bOrder, bMoved := moved[contact.B.Key()]

		switch {
		case !aMoved && !bMoved:
			return
		case aMoved && bMoved && aOrder > bOrder:
			handleCollision(contact.A, contact.Normal, contact.Depth)
		case aMoved && bMoved:
			handleCollision(contact.B, contact.Normal.Scale(-1), contact.Depth)
		case aMoved:
			handleCollision(contact.A, contact.Normal, contact.Depth)
		default:
			handleCollision(contact.B, contact.Normal.Scale(-1), contact.Depth)
		}
	})
}

func handleCollision(body spatial.Body, normal spatial.Vec2, depth float64) {
	p, ok := body.(*player.Player)
	if !ok {
		return
	}

	center := p.Shape().CenterPoint()
	target := center.Add(normal.Scale(depth))
	radius := p.Snapshot().Radius
	target.X = math.Min(math.Max(target.X, radius), config.WorldWidth-radius)
	target.Y = math.Min(math.Max(target.Y, radius), config.WorldHeight-radius)
	p.MoveTo(spatial.Vec2{X: target.X, Y: target.Y})
}

func (gr *GameRoom) refreshSpatial() {
	players := gr.players.List()
	bodies := make([]spatial.Body, len(players))
	for i, p := range players {
		bodies[i] = p
	}
	gr.spatial.Rebuild(bodies)
}

func (gr *GameRoom) OnUpdate(dt float64) {
	gr.reapStopped()
	gr.handleCollisions(gr.movePlayers(dt))
	gr.refreshSpatial()
	gr.publishSnapshot()
}

func (gr *GameRoom) publishSnapshot() {
	players := gr.players.List()
	if len(players) == 0 {
		return
	}

	states := make([]player.State, 0, len(players))
	for _, p := range players {
		states = append(states, p.Snapshot())
	}

	base := UpdateData{
		Players: states,
		Width:   config.WorldWidth,
		Height:  config.WorldHeight,
	}

	snapshot, err := marshalEvent(MsgUpdate, base)
	if err != nil {
		log.Printf("failed to marshal room update for %s: %v", gr.Id, err)
		return
	}

	for _, p := range players {
		p.Publish(player.BroadcastType, snapshot)
	}
}

func (gr *GameRoom) writeLoop(p *player.Player) {
	for {
		select {
		case <-p.Done():
			return
		case <-gr.ctx.Done():
			return
		default:
		}

		msg := p.NextOutbound()
		if msg == nil {
			select {
			case <-p.OutboxSignal():
			case <-p.Done():
				return
			case <-gr.ctx.Done():
				return
			}
			continue
		}

		writeCtx, cancel := context.WithTimeout(gr.ctx, 5*time.Second)
		err := p.Write(writeCtx, msg)
		cancel()
		if err != nil {
			if gr.ctx.Err() == nil {
				log.Printf("write to player %s failed: %v", p.Id, err)
				p.CloseWebSocket(websocket.StatusInternalError, "write failed")
			}
			return
		}
	}
}

func (gr *GameRoom) Run(ctx context.Context, closeCallback func(gr *GameRoom)) {
	timeout := make(chan struct{}, 1)
	gr.scheduler.Schedule(gr.cfg.RoomTimeout, func() {
		timeout <- struct{}{}
	})

	ticker := time.NewTicker(time.Second / time.Duration(config.TickRate))
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-timeout:
			gr.close(closeCallback)
			log.Printf("Game room %s closed because of timeout", gr.Id)
			return
		case <-ctx.Done():
			gr.close(closeCallback)
			log.Printf("Game room %s closed because context cancel", gr.Id)
			return
		case cmd := <-gr.commands:
			err := cmd.apply(gr)
			if err != nil {
				log.Println(err)
				return
			}
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			last = now
			if dt > 0.25 {
				dt = 0.25
			}
			gr.OnUpdate(dt)
		}
	}
}

func (gr *GameRoom) close(closeCallback func(gr *GameRoom)) {
	if !gr.closed.CompareAndSwap(false, true) {
		return
	}

	if gr.scheduler != nil {
		_ = gr.scheduler.Shutdown(context.Background())
	}

	players := gr.players.Clear()
	for _, p := range players {
		p.Stop()
	}

	var wg sync.WaitGroup
	for _, p := range players {
		wg.Add(1)
		go func(p *player.Player) {
			defer wg.Done()
			p.CloseWebSocket(websocket.StatusGoingAway, "room closed")
		}(p)
	}
	wg.Wait()

	gr.ctxCancel()
	closeCallback(gr)
}
