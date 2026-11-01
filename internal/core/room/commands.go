package room

type Command interface {
	Apply(*Room) error
}

type JoinCommand struct {
	Member Member
}

type LeaveCommand struct {
	Key string
}

func (c JoinCommand) Apply(r *Room) error {
	return r.Join(c.Member)
}

func (c LeaveCommand) Apply(r *Room) error {
	return r.Leave(c.Key)
}
