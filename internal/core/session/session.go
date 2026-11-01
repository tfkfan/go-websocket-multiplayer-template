package session

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/tfkfan/gogame/internal/core/snapshot"
)

const WriteTimeout = 5 * time.Second

type Session interface {
	Done() <-chan struct{}
	Publish(kind snapshot.SnapshotType, msg []byte) bool
	NextOutbound() []byte
	Signal() <-chan struct{}
	Write(ctx context.Context, msg []byte) error
	Stop()
	Close(code websocket.StatusCode, reason string)
}

func RunWriter(ctx context.Context, sess Session) {
	for {
		select {
		case <-sess.Done():
			return
		case <-ctx.Done():
			return
		default:
		}

		msg := sess.NextOutbound()
		if msg == nil {
			select {
			case <-sess.Signal():
			case <-sess.Done():
				return
			case <-ctx.Done():
				return
			}
			continue
		}

		writeCtx, cancel := context.WithTimeout(ctx, WriteTimeout)
		err := sess.Write(writeCtx, msg)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("write failed: %v", err)
				sess.Close(websocket.StatusInternalError, "write failed")
			}
			return
		}
	}
}
