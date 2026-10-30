package room

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/shared"
)

const (
	WorldWidth   = 1200.0
	WorldHeight  = 800.0
	PlayerRadius = 25.0
	TickRate     = 30

	commandBuffer = 64
)

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
	Id uuid.UUID

	closed atomic.Bool

	ctx         context.Context
	ctxCancel   context.CancelFunc
	roomTimeout time.Duration

	capacity     int
	commands     chan roomCommand
	players      *playerSet
	scheduler    *shared.Scheduler
	spawnCounter int
	tick         uint64
}

func NewGameRoom(gameRoomContext context.Context, gameRoomCancel context.CancelFunc, srvCfg *config.Config) *GameRoom {
	r := &GameRoom{
		Id:           uuid.New(),
		capacity:     srvCfg.RoomCapacity,
		commands:     make(chan roomCommand, commandBuffer),
		players:      newPlayerSet(),
		ctx:          gameRoomContext,
		ctxCancel:    gameRoomCancel,
		roomTimeout:  srvCfg.RoomTimeout,
		scheduler:    shared.NewScheduler(),
		spawnCounter: 0,
	}

	log.Printf("created game room %s (capacity %d)", r.Id, r.capacity)

	return r
}

func (gr *GameRoom) Context() context.Context {
	return gr.ctx
}

func (gr *GameRoom) AddPlayer(p *player.Player) error {
	reply := make(chan error, 1)

	select {
	case gr.commands <- joinCommand{player: p, reply: reply}:
	case <-gr.ctx.Done():
		return ErrRoomClosed
	}

	select {
	case err := <-reply:
		return err
	case <-gr.ctx.Done():
		return ErrRoomClosed
	}
}

func (gr *GameRoom) SendTo(playerID uuid.UUID, messageType MessageType, data any) error {
	return gr.PublishTo(playerID, player.PrivateType, messageType, data)
}

func (gr *GameRoom) PublishTo(playerID uuid.UUID, kind player.SnapshotType, messageType MessageType, data any) error {
	if gr.closed.Load() || gr.ctx.Err() != nil {
		return ErrRoomClosed
	}

	p, ok := gr.players.get(playerID)
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

func (gr *GameRoom) OnDisconnect(p *player.Player) {
	log.Printf("player %s disconnected from room %s", p.Id, gr.Id)
	p.Stop()

	select {
	case gr.commands <- leaveCommand{player: p}:
	case <-gr.ctx.Done():
	}
}

func (gr *GameRoom) join(p *player.Player) error {
	if gr.closed.Load() || gr.ctx.Err() != nil {
		return ErrRoomClosed
	}

	gr.reapStopped()

	if _, exists := gr.players.get(p.Id); exists {
		return ErrPlayerExists
	}
	if gr.players.len() >= gr.capacity {
		return ErrRoomFull
	}

	slot := gr.nextSlot()
	p.Spawn(gr.slotPosition(slot), PlayerRadius, palette[slot%len(palette)])
	if !gr.players.add(p) {
		return ErrPlayerExists
	}

	state := p.Snapshot()
	msg, err := marshalEvent(MsgJoinSuccess, JoinSuccessData{
		PlayerID: p.Id,
		RoomID:   gr.Id,
		Width:    WorldWidth,
		Height:   WorldHeight,
		Radius:   state.Radius,
		Color:    state.Color,
		TickRate: TickRate,
	})
	if err != nil {
		gr.players.remove(p)
		return err
	}

	p.Publish(player.SystemType, msg)
	go gr.writeLoop(p)
	gr.publishSnapshot()

	return nil
}

func (gr *GameRoom) leave(p *player.Player) {
	gr.players.remove(p)
	p.Stop()
}

func (gr *GameRoom) reapStopped() {
	for _, p := range gr.players.list() {
		select {
		case <-p.Done():
			gr.players.remove(p)
		default:
		}
	}
}

func (gr *GameRoom) nextSlot() int {
	slots := gr.capacity
	if slots < 1 {
		return 0
	}

	minDist := PlayerRadius * 2.5
	players := gr.players.list()
	for i := range slots {
		slot := (gr.spawnCounter + i) % slots
		pos := gr.slotPosition(slot)
		occupied := false
		for _, other := range players {
			state := other.Snapshot()
			dx := state.Position.X - pos.X
			dy := state.Position.Y - pos.Y
			if dx*dx+dy*dy < minDist*minDist {
				occupied = true
				break
			}
		}
		if !occupied {
			gr.spawnCounter = slot + 1
			return slot
		}
	}

	slot := gr.spawnCounter % slots
	gr.spawnCounter++
	return slot
}

func (gr *GameRoom) slotPosition(slot int) player.Vec2 {
	slots := gr.capacity
	if slots < 1 {
		slots = 1
	}
	angle := 2*math.Pi*float64(slot)/float64(slots) - math.Pi/2
	ring := math.Min(WorldWidth, WorldHeight) * 0.32
	return player.Vec2{
		X: WorldWidth/2 + ring*math.Cos(angle),
		Y: WorldHeight/2 + ring*math.Sin(angle),
	}
}

func (gr *GameRoom) ReadWebSocket(p *player.Player) error {
	for {
		msg, err := p.Read(gr.ctx)
		if err != nil {
			return err
		}
		gr.handleClientMessage(p, msg)
	}
}

func (gr *GameRoom) handleClientMessage(p *player.Player, raw []byte) {
	var event clientEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		log.Printf("invalid message from player %s: %v", p.Id, err)
		return
	}
	if event.Type != MsgPlayerKeyDown {
		return
	}

	var key keyEvent
	if err := json.Unmarshal(event.Data, &key); err != nil {
		log.Printf("invalid key event from player %s: %v", p.Id, err)
		return
	}
	p.ApplyKey(key.Key, key.State)
}

func (gr *GameRoom) update(dt float64) {
	for _, p := range gr.players.list() {
		p.Move(dt, WorldWidth, WorldHeight)
	}
}

func (gr *GameRoom) publishSnapshot() {
	players := gr.players.list()
	if len(players) == 0 {
		return
	}

	states := make([]player.State, 0, len(players))
	for _, p := range players {
		states = append(states, p.Snapshot())
	}

	gr.tick++
	base := UpdateData{
		Players: states,
		Width:   WorldWidth,
		Height:  WorldHeight,
		Tick:    gr.tick,
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
	gr.scheduler.Schedule(gr.roomTimeout, func() {
		timeout <- struct{}{}
	})

	ticker := time.NewTicker(time.Second / time.Duration(TickRate))
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
			cmd.apply(gr)
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			last = now
			if dt > 0.25 {
				dt = 0.25
			}
			gr.reapStopped()
			gr.update(dt)
			gr.publishSnapshot()
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

	players := gr.players.clear()
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
