package room

import "github.com/tfkfan/gogame/internal/game/player"

type roomCommand interface {
	apply(*GameRoom)
}

type joinCommand struct {
	player *player.Player
	reply  chan error
}

type leaveCommand struct {
	player *player.Player
}

func (c joinCommand) apply(gr *GameRoom) {
	c.reply <- gr.join(c.player)
}

func (c leaveCommand) apply(gr *GameRoom) {
	gr.leave(c.player)
}
