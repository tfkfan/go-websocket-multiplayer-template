package player

import (
	"context"
	math "math"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/game/entity"
	m "github.com/tfkfan/gogame/internal/math"
	"github.com/tfkfan/gogame/internal/spatial"
)

const Speed = 260.0

const (
	keyUp uint32 = 1 << iota
	keyDown
	keyLeft
	keyRight
)

var keyBits = map[string]uint32{
	"UP":    keyUp,
	"DOWN":  keyDown,
	"LEFT":  keyLeft,
	"RIGHT": keyRight,
}

type Player struct {
	BasePlayer
	keys atomic.Uint32

	radius float64
	color  string
}

type State struct {
	ID       uuid.UUID    `json:"id"`
	Position spatial.Vec2 `json:"position"`
	Radius   float64      `json:"radius"`
	Color    string       `json:"color"`
}

func ExtractPlayerFromRequest(r *http.Request, ws *websocket.Conn, ctx context.Context) (*Player, error) {
	id, err := uuid.Parse(r.Header.Get("X-Player-Id"))
	if err != nil {
		id = uuid.New()
	}
	return NewPlayer(ctx, id, ws), nil
}

func NewPlayer(ctx context.Context, id uuid.UUID, ws *websocket.Conn) *Player {
	playerCtx, cancel := context.WithCancel(ctx)
	p := &Player{
		Id:     id,
		Ctx:    playerCtx,
		Cancel: cancel,
		ws:     ws,
		keys:   atomic.Uint32{},
		radius: 0,
		color:  "",
		Outbox: entity.NewOutbox(),
	}

	p.BasePlayer.IGameEntity = p
	return p
}

func (p *Player) Snapshot() State {
	return State{
		ID:       p.Id,
		Position: p.Position,
		Radius:   p.radius,
		Color:    p.color,
	}
}

func (p *Player) Spawn(world entity.World, pos spatial.Vec2, radius float64, color string) {
	p.BasePlayer.GameEntity.Position = pos
	p.radius = radius
	p.World = world
	p.color = color
}

func (p *Player) Shape() spatial.Shape {
	return spatial.Circle{
		Center: spatial.Vec2{X: p.Position.X, Y: p.Position.Y},
		Radius: p.radius,
	}
}

func (p *Player) Update(dt float64) {
	p.move(dt)
}

func (p *Player) ApplyKey(key string, pressed bool) bool {
	bit, ok := keyBits[key]
	if !ok {
		return false
	}

	for {
		old := p.keys.Load()
		next := old &^ bit
		if pressed {
			next = old | bit
		}
		if p.keys.CompareAndSwap(old, next) {
			return true
		}
	}
}

func (p *Player) move(dt float64) {
	mask := p.keys.Load()

	dx, dy := 0.0, 0.0
	if mask&keyLeft != 0 {
		dx--
	}
	if mask&keyRight != 0 {
		dx++
	}
	if mask&keyUp != 0 {
		dy--
	}
	if mask&keyDown != 0 {
		dy++
	}
	if dx != 0 && dy != 0 {
		dx /= math.Sqrt2
		dy /= math.Sqrt2
	}

	p.Position.X = m.Clamp(p.Position.X+dx*Speed*dt, p.radius, p.World.Width-p.radius)
	p.Position.Y = m.Clamp(p.Position.Y+dy*Speed*dt, p.radius, p.World.Height-p.radius)
}
