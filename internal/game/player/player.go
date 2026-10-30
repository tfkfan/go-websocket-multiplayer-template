package player

import (
	"context"
	"math"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/network"
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
	Id uuid.UUID

	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	keys atomic.Uint32

	position Vec2
	radius   float64
	color    string

	outbox *outbox
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
	return &Player{
		Id:     id,
		ws:     ws,
		ctx:    playerCtx,
		cancel: cancel,
		outbox: newOutbox(),
	}
}

func (p *Player) Spawn(pos Vec2, radius float64, color string) {
	p.position = pos
	p.radius = radius
	p.color = color
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

func (p *Player) Move(dt, width, height float64) {
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

	p.position.X = clamp(p.position.X+dx*Speed*dt, p.radius, width-p.radius)
	p.position.Y = clamp(p.position.Y+dy*Speed*dt, p.radius, height-p.radius)
}

func clamp(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (p *Player) Read(ctx context.Context) ([]byte, error) {
	_, msg, err := p.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *Player) Write(ctx context.Context, msg []byte) error {
	return p.ws.Write(ctx, websocket.MessageText, msg)
}

func (p *Player) Done() <-chan struct{} {
	return p.ctx.Done()
}

func (p *Player) Stop() {
	p.cancel()
}

func (p *Player) CloseWebSocket(statusCode websocket.StatusCode, errorMessage string) {
	network.CloseWebSocket(p.ws, statusCode, errorMessage)
}
