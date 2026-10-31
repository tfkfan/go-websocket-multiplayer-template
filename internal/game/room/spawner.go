package room

import (
	"math"

	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/player"
)

type Spawner struct {
	room         *GameRoom
	spawnCounter int
}

func NewSpawner(room *GameRoom) *Spawner {
	return &Spawner{
		room:         room,
		spawnCounter: 0,
	}
}

func (s *Spawner) nextSlot() int {
	slots := s.room.cfg.RoomCapacity
	if slots < 1 {
		return 0
	}

	minDist := config.PlayerRadius * 2.5
	players := s.room.players.List()
	for i := range slots {
		slot := (s.spawnCounter + i) % slots
		pos := s.slotPosition(slot)
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
			s.spawnCounter = slot + 1
			return slot
		}
	}

	slot := s.spawnCounter % slots
	s.spawnCounter++
	return slot
}

func (s *Spawner) slotPosition(slot int) player.Vec2 {
	slots := max(s.room.cfg.RoomCapacity, 1)
	angle := 2*math.Pi*float64(slot)/float64(slots) - math.Pi/2
	ring := math.Min(config.WorldWidth, config.WorldHeight) * 0.32
	return player.Vec2{
		X: config.WorldWidth/2 + ring*math.Cos(angle),
		Y: config.WorldHeight/2 + ring*math.Sin(angle),
	}
}
