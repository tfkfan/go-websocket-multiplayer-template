package entity

import (
	"context"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/spatial"
)

type IGameEntity interface {
	spatial.Body

	Update(dt float64)
}
type GameEntity struct {
	IGameEntity
	Id uuid.UUID

	Position     spatial.Vec2
	Velocity     spatial.Vec2
	Acceleration spatial.Vec2

	Ctx    context.Context
	Cancel context.CancelFunc

	World World

	Outbox *Outbox
}

func (e *GameEntity) Publish(kind SnapshotType, msg []byte) {
	if kind >= TypeCount {
		return
	}
	select {
	case <-e.Ctx.Done():
		return
	default:
	}
	e.Outbox.Publish(kind, msg)
}

func (e *GameEntity) NextOutbound() []byte {
	return e.Outbox.Next()
}

func (e *GameEntity) OutboxSignal() <-chan struct{} {
	return e.Outbox.Signal
}

func (e *GameEntity) MoveTo(center spatial.Vec2) {
	e.Position = spatial.Vec2{X: center.X, Y: center.Y}
}

func (e *GameEntity) Key() string {
	return e.Id.String()
}

func (e *GameEntity) Solid() bool {
	return true
}
