package player

import "sync"

type outbox struct {
	mu     sync.Mutex
	data   map[SnapshotType][]byte
	signal chan struct{}
}

func newOutbox() *outbox {
	return &outbox{
		data:   make(map[SnapshotType][]byte),
		signal: make(chan struct{}, 1),
	}
}

func (p *Player) Publish(kind SnapshotType, msg []byte) {
	if kind >= TypeCount {
		return
	}
	select {
	case <-p.ctx.Done():
		return
	default:
	}
	p.outbox.publish(kind, msg)
}

func (p *Player) NextOutbound() []byte {
	return p.outbox.next()
}

func (p *Player) OutboxSignal() <-chan struct{} {
	return p.outbox.signal
}

func (o *outbox) publish(kind SnapshotType, msg []byte) {
	o.mu.Lock()
	o.data[kind] = msg
	o.mu.Unlock()
	o.wake()
}

func (o *outbox) next() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()

	for kind := SnapshotType(0); kind < TypeCount; kind++ {
		if msg, ok := o.data[kind]; ok {
			delete(o.data, kind)
			return msg
		}
	}
	return nil
}

func (o *outbox) wake() {
	select {
	case o.signal <- struct{}{}:
	default:
	}
}
