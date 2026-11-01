package room

import "github.com/tfkfan/gogame/internal/game/player"

type Command interface {
	apply(room IRoom) error
}

type JoinCommand struct {
	Player *player.Player
}

type LeaveCommand struct {
	Player *player.Player
}

func (c JoinCommand) apply(gr IRoom) error {
	return gr.OnJoin(c.Player)
}

func (c LeaveCommand) apply(gr IRoom) error {
	return gr.OnDisconnect(c.Player)
}
