package room

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/entity"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/shared"
	"github.com/tfkfan/gogame/internal/spatial"
)

const (
	commandBuffer = 64
)

var (
	ErrRoomClosed     = errors.New("game room is closed")
	ErrRoomFull       = errors.New("game room is full")
	ErrPlayerExists   = errors.New("player already connected")
	ErrPlayerNotFound = errors.New("player not found")
)

type IRoom interface {
	OnDisconnect(p *player.Player) error
	OnJoin(p *player.Player) error
	OnUpdate(dt float64)
}

type CollisionHandler interface {
	handleCollision(body spatial.Body, normal spatial.Vec2, depth float64)
}

type BaseRoom struct {
	IRoom
	CollisionHandler

	Id uuid.UUID

	closed    atomic.Bool
	cfg       *config.Config
	ctx       context.Context
	ctxCancel context.CancelFunc

	commands  chan Command
	world     entity.World
	players   *shared.Set[uuid.UUID, *player.Player]
	spatial   *spatial.Quadtree
	scheduler *shared.Scheduler
}

func (gr *BaseRoom) Context() context.Context {
	return gr.ctx
}

func (gr *BaseRoom) SendTo(playerID uuid.UUID, messageType event.MessageType, data any) error {
	return gr.PublishTo(playerID, entity.PrivateType, messageType, data)
}

func (gr *BaseRoom) PublishTo(playerID uuid.UUID, kind entity.SnapshotType, messageType event.MessageType, data any) error {
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

	msg, err := event.MarshalEvent(messageType, data)
	if err != nil {
		return err
	}

	p.Publish(kind, msg)
	return nil
}

func (gr *BaseRoom) SubmitCommand(command Command) error {
	select {
	case gr.commands <- command:
		return nil
	case <-gr.ctx.Done():
		return ErrRoomClosed
	}
}

func (gr *BaseRoom) reapStopped() {
	for _, p := range gr.players.List() {
		select {
		case <-p.Done():
			gr.players.Remove(p.Id)
		default:
		}
	}
}

func (gr *BaseRoom) handleCollisions(moved map[string]int) {
	spatial.DetectCollisions(gr.spatial, func(contact spatial.Contact) {
		aIdx, aMoved := moved[contact.A.Key()]
		bIdx, bMoved := moved[contact.B.Key()]

		switch {
		case !aMoved && !bMoved:
			return
		case aMoved && bMoved && aIdx > bIdx:
			gr.handleCollision(contact.A, contact.Normal, contact.Depth)
		case aMoved && bMoved:
			gr.handleCollision(contact.B, contact.Normal.Scale(-1), contact.Depth)
		case aMoved:
			gr.handleCollision(contact.A, contact.Normal, contact.Depth)
		default:
			gr.handleCollision(contact.B, contact.Normal.Scale(-1), contact.Depth)
		}
	})
}

func (gr *BaseRoom) refreshSpatial() {
	players := gr.players.List()
	bodies := make([]spatial.Body, len(players))
	for i, p := range players {
		bodies[i] = p
	}
	gr.spatial.Rebuild(bodies)
}

func (gr *BaseRoom) publishSnapshot() {
	players := gr.players.List()
	if len(players) == 0 {
		return
	}

	states := make([]player.State, 0, len(players))
	for _, p := range players {
		states = append(states, p.Snapshot())
	}

	base := event.UpdateData{
		Players: states,
		Width:   gr.world.Width,
		Height:  gr.world.Height,
	}

	snapshot, err := event.MarshalEvent(event.MsgUpdate, base)
	if err != nil {
		log.Printf("failed to marshal room update for %s: %v", gr.Id, err)
		return
	}

	for _, p := range players {
		p.Publish(entity.BroadcastType, snapshot)
	}
}

func (gr *BaseRoom) writeLoop(p *player.Player) {
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

func (gr *BaseRoom) Run(ctx context.Context, closeCallback func(gr *BaseRoom)) {
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

func (gr *BaseRoom) close(closeCallback func(gr *BaseRoom)) {
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
