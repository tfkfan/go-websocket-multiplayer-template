package player

import "github.com/google/uuid"

type SnapshotType uint8

const (
	SystemType SnapshotType = iota
	PrivateType
	BroadcastType
	CustomType
	TypeCount
)

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type State struct {
	ID       uuid.UUID `json:"id"`
	Position Vec2      `json:"position"`
	Radius   float64   `json:"radius"`
	Color    string    `json:"color"`
}

func (p *Player) Snapshot() State {
	return State{
		ID:       p.Id,
		Position: p.position,
		Radius:   p.radius,
		Color:    p.color,
	}
}
