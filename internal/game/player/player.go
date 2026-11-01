package player

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/core/entity"
	m "github.com/tfkfan/gogame/internal/core/math"
	"github.com/tfkfan/gogame/internal/core/spatial"
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

type State struct {
	ID       uuid.UUID    `json:"id"`
	Position spatial.Vec2 `json:"position"`
	Radius   float64      `json:"radius"`
	Color    string       `json:"color"`
}

type Profile struct {
	Speed  float64
	Radius float64
	Color  string
}

func DefaultProfile() Profile {
	return Profile{
		Speed:  Speed,
		Radius: config.PlayerRadius,
	}
}

type Player struct {
	entity.Base

	world      entity.World
	profile    Profile
	controller Controller
}

func IDFromRequest(r *http.Request) uuid.UUID {
	id, err := uuid.Parse(r.Header.Get("X-Player-Id"))
	if err != nil {
		return uuid.New()
	}
	return id
}

func New(id uuid.UUID, profile Profile, controller Controller) *Player {
	return &Player{
		Id:         id,
		profile:    profile,
		controller: controller,
	}
}

func (p *Player) Snapshot() State {
	return State{
		ID:       p.Id,
		Position: p.Position,
		Radius:   p.profile.Radius,
		Color:    p.profile.Color,
	}
}

func (p *Player) Contribute() (any, bool) {
	return p.Snapshot(), true
}

func (p *Player) Spawn(world entity.World, position spatial.Vec2, color string) {
	p.world = world
	p.Position = position
	p.profile.Color = color
}

func (p *Player) Shape() spatial.Shape {
	return spatial.Circle{
		Center: spatial.Vec2{X: p.Position.X, Y: p.Position.Y},
		Radius: p.profile.Radius,
	}
}

func (p *Player) Extent() float64 {
	return p.profile.Radius
}

func (p *Player) Update(dt float64) {
	if p.controller == nil {
		return
	}
	dx, dy := p.controller.Direction()
	radius := p.profile.Radius
	p.Position.X = m.Clamp(p.Position.X+dx*p.profile.Speed*dt, radius, p.world.Width-radius)
	p.Position.Y = m.Clamp(p.Position.Y+dy*p.profile.Speed*dt, radius, p.world.Height-radius)
}

func (p *Player) ApplyKey(key string, pressed bool) bool {
	controller, ok := p.controller.(KeyController)
	if !ok {
		return false
	}
	return controller.ApplyKey(key, pressed)
}
