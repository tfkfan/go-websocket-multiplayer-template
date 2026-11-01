package session

import (
	"context"
	"sync"

	"github.com/coder/websocket"
	"github.com/tfkfan/gogame/internal/core/snapshot"
)

type WebSocket struct {
	conn   *websocket.Conn
	outbox *snapshot.Outbox
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func NewWebSocket(parent context.Context, conn *websocket.Conn) *WebSocket {
	ctx, cancel := context.WithCancel(parent)
	return &WebSocket{
		conn:   conn,
		outbox: snapshot.NewOutbox(ctx.Done()),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *WebSocket) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *WebSocket) Publish(kind snapshot.SnapshotType, msg []byte) bool {
	return s.outbox.Publish(kind, msg)
}

func (s *WebSocket) NextOutbound() []byte {
	return s.outbox.Next()
}

func (s *WebSocket) Signal() <-chan struct{} {
	return s.outbox.Signal
}

func (s *WebSocket) Write(ctx context.Context, msg []byte) error {
	return s.conn.Write(ctx, websocket.MessageText, msg)
}

func (s *WebSocket) Read(ctx context.Context) ([]byte, error) {
	_, msg, err := s.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *WebSocket) StartWriter(ctx context.Context) {
	s.once.Do(func() {
		go RunWriter(ctx, s)
	})
}

func (s *WebSocket) Stop() {
	s.cancel()
}

func (s *WebSocket) Close(code websocket.StatusCode, reason string) {
	_ = s.conn.Close(code, reason)
}
