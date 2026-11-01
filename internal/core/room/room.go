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
	"github.com/tfkfan/gogame/internal/core/entity"
	"github.com/tfkfan/gogame/internal/core/session"
	"github.com/tfkfan/gogame/internal/core/snapshot"
	"github.com/tfkfan/gogame/internal/shared"
)

var (
	ErrClosed        = errors.New("game room is closed")
	ErrExists        = errors.New("member already joined")
	ErrNotFound      = errors.New("member not found")
	ErrNoSession     = errors.New("member has no session")
	ErrInvalidMember = errors.New("member has no entity")
)

type Member struct {
	Entity  entity.Entity
	Session session.Session
}

type Hooks interface {
	OnJoin(Member) error
	OnLeave(Member)
	OnTick(dt float64)
}

type Config struct {
	Id            uuid.UUID
	Timeout       time.Duration
	TickInterval  time.Duration
	CommandBuffer int
}

type Room struct {
	Id uuid.UUID

	hooks Hooks

	closed    atomic.Bool
	ctx       context.Context
	ctxCancel context.CancelFunc

	commands  chan Command
	members   *shared.Set[string, Member]
	scheduler *shared.Scheduler

	timeout      time.Duration
	tickInterval time.Duration
}

func New(ctx context.Context, cancel context.CancelFunc, cfg Config, hooks Hooks) *Room {
	buffer := cfg.CommandBuffer
	if buffer <= 0 {
		buffer = 64
	}
	interval := cfg.TickInterval
	if interval <= 0 {
		interval = time.Second / 30
	}

	return &Room{
		Id:           cfg.Id,
		hooks:        hooks,
		ctx:          ctx,
		ctxCancel:    cancel,
		commands:     make(chan Command, buffer),
		members:      shared.NewSet[string, Member](),
		scheduler:    shared.NewScheduler(),
		timeout:      cfg.Timeout,
		tickInterval: interval,
	}
}

func (r *Room) Context() context.Context {
	return r.ctx
}

func (r *Room) Closed() bool {
	return r.closed.Load() || r.ctx.Err() != nil
}

func (r *Room) Len() int {
	return r.members.Len()
}

func (r *Room) Members() []Member {
	return r.members.List()
}

func (r *Room) Member(key string) (Member, bool) {
	return r.members.Get(key)
}

func (r *Room) SubmitCommand(command Command) error {
	select {
	case r.commands <- command:
		return nil
	case <-r.ctx.Done():
		return ErrClosed
	}
}

func (r *Room) Join(member Member) error {
	if r.Closed() {
		return ErrClosed
	}
	if member.Entity == nil {
		return ErrInvalidMember
	}

	key := member.Entity.Key()
	if !r.members.Add(key, member) {
		return ErrExists
	}
	if err := r.hooks.OnJoin(member); err != nil {
		r.members.Remove(key)
		return err
	}
	if member.Session != nil {
		go session.RunWriter(r.ctx, member.Session)
	}
	return nil
}

func (r *Room) Leave(key string) error {
	member, ok := r.members.Remove(key)
	if !ok {
		return nil
	}
	if member.Session != nil {
		member.Session.Stop()
	}
	r.hooks.OnLeave(member)
	return nil
}

func (r *Room) PublishTo(key string, kind snapshot.SnapshotType, msg []byte) error {
	if r.Closed() {
		return ErrClosed
	}

	member, ok := r.members.Get(key)
	if !ok {
		if r.closed.Load() {
			return ErrClosed
		}
		return ErrNotFound
	}
	if member.Session == nil {
		return ErrNoSession
	}

	select {
	case <-member.Session.Done():
		if r.closed.Load() {
			return ErrClosed
		}
		return ErrNotFound
	default:
	}

	member.Session.Publish(kind, msg)
	return nil
}

func (r *Room) Broadcast(kind snapshot.SnapshotType, msg []byte) {
	for _, member := range r.members.List() {
		if member.Session == nil {
			continue
		}
		member.Session.Publish(kind, msg)
	}
}

func (r *Room) Shutdown() error {
	return r.scheduler.Shutdown(context.Background())
}

func (r *Room) Run(ctx context.Context, onClose func(*Room)) {
	timeout := make(chan struct{}, 1)
	r.scheduler.Schedule(r.timeout, func() {
		timeout <- struct{}{}
	})

	ticker := time.NewTicker(r.tickInterval)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-timeout:
			r.close(onClose)
			log.Printf("Game room %s closed because of timeout", r.Id)
			return
		case <-ctx.Done():
			r.close(onClose)
			log.Printf("Game room %s closed because context cancel", r.Id)
			return
		case command := <-r.commands:
			if err := command.Apply(r); err != nil {
				log.Println(err)
				return
			}
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			last = now
			if dt > 0.25 {
				dt = 0.25
			}
			r.reapStopped()
			r.hooks.OnTick(dt)
		}
	}
}

func (r *Room) reapStopped() {
	for _, member := range r.members.List() {
		if member.Session == nil {
			continue
		}
		select {
		case <-member.Session.Done():
			if _, ok := r.members.Remove(member.Entity.Key()); ok {
				r.hooks.OnLeave(member)
			}
		default:
		}
	}
}

func (r *Room) close(onClose func(*Room)) {
	if !r.closed.CompareAndSwap(false, true) {
		return
	}

	_ = r.scheduler.Shutdown(context.Background())

	members := r.members.Clear()
	for _, member := range members {
		if member.Session != nil {
			member.Session.Stop()
		}
	}

	var wg sync.WaitGroup
	for _, member := range members {
		if member.Session == nil {
			continue
		}
		wg.Add(1)
		go func(s session.Session) {
			defer wg.Done()
			s.Close(websocket.StatusGoingAway, "room closed")
		}(member.Session)
	}
	wg.Wait()

	r.ctxCancel()
	if onClose != nil {
		onClose(r)
	}
}
