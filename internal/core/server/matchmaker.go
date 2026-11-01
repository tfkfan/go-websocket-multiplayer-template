package server

import (
	"context"
	"errors"
)

var (
	ErrMatchmakingClosed = errors.New("matchmaking is closed")
	ErrPendingClosed     = errors.New("pending player is closed")
)

type Pending interface {
	Key() string
	Done() <-chan struct{}
}

type matchResult struct {
	room Room
	err  error
}

type queuedPending struct {
	pending Pending
	result  chan matchResult
}

func (m *Matchmaker) MakeMatch(pending Pending) (Room, error) {
	entry := queuedPending{
		pending: pending,
		result:  make(chan matchResult, 1),
	}
	m.pending.Enqueue(entry)

	for {
		if isClosed(pending) {
			m.pending.Remove(pending.Key())
			return nil, ErrPendingClosed
		}

		select {
		case outcome := <-entry.result:
			return outcome.room, outcome.err
		default:
		}

		if m.formBatch() {
			continue
		}

		select {
		case outcome := <-entry.result:
			return outcome.room, outcome.err
		case <-m.pending.Wait():
		case <-pending.Done():
			m.pending.Remove(pending.Key())
			return nil, ErrPendingClosed
		case <-m.ctx.Done():
			m.pending.Remove(pending.Key())
			return nil, ErrMatchmakingClosed
		}
	}
}

func (m *Matchmaker) Remove(pending Pending) {
	m.pending.Remove(pending.Key())
}

func (m *Matchmaker) formBatch() bool {
	m.formation.Lock()
	batch := m.pending.TakeBatch(m.def.Capacity())
	if batch == nil {
		m.formation.Unlock()
		return false
	}

	m.formMatch(batch)
	m.formation.Unlock()
	return true
}

func (m *Matchmaker) formMatch(batch []queuedPending) {
	roomCtx, roomCancel := context.WithCancel(m.ctx)
	room, err := m.def.NewRoom(roomCtx, roomCancel)
	if err != nil {
		for _, queued := range batch {
			queued.result <- matchResult{err: err}
		}
		return
	}

	pendings := make([]Pending, len(batch))
	for index, queued := range batch {
		pendings[index] = queued.pending
	}

	if err := m.def.Place(room, pendings); err != nil {
		roomCancel()
		survivors := make([]queuedPending, 0, len(batch))
		for _, queued := range batch {
			if isClosed(queued.pending) {
				continue
			}
			survivors = append(survivors, queued)
		}
		m.pending.Requeue(survivors)
		return
	}

	m.store(room)
	go room.RunRoom(m.ctx, func() {
		m.remove(room.RoomID())
	})
	for _, queued := range batch {
		queued.result <- matchResult{room: room}
	}
}
