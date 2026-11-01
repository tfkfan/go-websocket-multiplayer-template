package entity

import (
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/core/spatial"
)

type Entity interface {
	spatial.Body

	Update(dt float64)
}

type Base struct {
	Id       uuid.UUID
	Position spatial.Vec2
}

func (b *Base) Key() string {
	return b.Id.String()
}

func (b *Base) Solid() bool {
	return true
}

func (b *Base) MoveTo(center spatial.Vec2) {
	b.Position = spatial.Vec2{X: center.X, Y: center.Y}
}

type World struct {
	Width  float64
	Height float64
}
