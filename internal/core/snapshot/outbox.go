package snapshot

import "sync"

type Outbox struct {
	done   <-chan struct{}
	mu     sync.Mutex
	data   map[SnapshotType][]byte
	Signal chan struct{}
}

func NewOutbox(done <-chan struct{}) *Outbox {
	return &Outbox{
		done:   done,
		data:   make(map[SnapshotType][]byte),
		Signal: make(chan struct{}, 1),
	}
}

func (o *Outbox) Publish(kind SnapshotType, msg []byte) bool {
	if kind >= TypeCount {
		return false
	}
	if o.done != nil {
		select {
		case <-o.done:
			return false
		default:
		}
	}

	o.mu.Lock()
	o.data[kind] = msg
	o.mu.Unlock()
	o.wake()
	return true
}

func (o *Outbox) Next() []byte {
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

func (o *Outbox) wake() {
	select {
	case o.Signal <- struct{}{}:
	default:
	}
}
