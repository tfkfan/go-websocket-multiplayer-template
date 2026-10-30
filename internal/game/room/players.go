package room

import (
	"sync"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/game/player"
)

type playerSet struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]*player.Player
}

func newPlayerSet() *playerSet {
	return &playerSet{byID: make(map[uuid.UUID]*player.Player)}
}

func (ps *playerSet) add(p *player.Player) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, exists := ps.byID[p.Id]; exists {
		return false
	}
	ps.byID[p.Id] = p
	return true
}

func (ps *playerSet) get(id uuid.UUID) (*player.Player, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	p, ok := ps.byID[id]
	return p, ok
}

func (ps *playerSet) remove(p *player.Player) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if current, ok := ps.byID[p.Id]; ok && current == p {
		delete(ps.byID, p.Id)
	}
}

func (ps *playerSet) len() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return len(ps.byID)
}

func (ps *playerSet) list() []*player.Player {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	players := make([]*player.Player, 0, len(ps.byID))
	for _, p := range ps.byID {
		players = append(players, p)
	}
	return players
}

func (ps *playerSet) clear() []*player.Player {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	players := make([]*player.Player, 0, len(ps.byID))
	for _, p := range ps.byID {
		players = append(players, p)
	}
	ps.byID = make(map[uuid.UUID]*player.Player)
	return players
}
