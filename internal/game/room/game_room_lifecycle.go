package room

import "github.com/tfkfan/gogame/internal/game/player"

type GameRoomLifecycle interface {
	OnDisconnect(p *player.Player)
	OnJoin(p *player.Player) error
	OnUpdate(dt float64)
}
