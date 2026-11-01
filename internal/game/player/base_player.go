package player

import (
	"context"

	"github.com/coder/websocket"
	"github.com/tfkfan/gogame/internal/game/entity"
	"github.com/tfkfan/gogame/internal/network"
)

type BasePlayer struct {
	entity.GameEntity

	ws *websocket.Conn
}

func (p *BasePlayer) Read(ctx context.Context) ([]byte, error) {
	_, msg, err := p.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *BasePlayer) Write(ctx context.Context, msg []byte) error {
	return p.ws.Write(ctx, websocket.MessageText, msg)
}

func (p *BasePlayer) Done() <-chan struct{} {
	return p.Ctx.Done()
}

func (p *BasePlayer) Stop() {
	p.Cancel()
}

func (p *BasePlayer) CloseWebSocket(statusCode websocket.StatusCode, errorMessage string) {
	network.CloseWebSocket(p.ws, statusCode, errorMessage)
}
