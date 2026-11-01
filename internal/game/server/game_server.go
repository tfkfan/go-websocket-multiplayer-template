package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	coreroom "github.com/tfkfan/gogame/internal/core/room"
	coreserver "github.com/tfkfan/gogame/internal/core/server"
	"github.com/tfkfan/gogame/internal/core/session"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/game/room"
)

var assetContentTypes = map[string]string{
	".pck":  "application/octet-stream",
	".wasm": "application/wasm",
}

type definition struct {
	cfg *config.Config
}

func NewGameServer(cfg *config.Config) (*coreserver.Matchmaker, <-chan error, error) {
	return coreserver.New(coreserver.Options{
		Addr:              cfg.Port,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		AssetsDir:         cfg.AssetsDir,
		AssetContentTypes: assetContentTypes,
		WebSocketEndpoint: cfg.WebSocketEndpoint,
	}, &definition{cfg: cfg})
}

type pendingPlayer struct {
	ctx      context.Context
	session  *session.WebSocket
	player   *player.Player
	messages chan []byte
}

func (p *pendingPlayer) Key() string {
	return p.player.Key()
}

func (p *pendingPlayer) Done() <-chan struct{} {
	return p.session.Done()
}

func (p *pendingPlayer) readPump() {
	defer close(p.messages)

	for {
		msg, err := p.session.Read(p.ctx)
		if err != nil {
			p.session.Stop()
			return
		}
		select {
		case p.messages <- msg:
		case <-p.ctx.Done():
			p.session.Stop()
			return
		}
	}
}

func (d *definition) Capacity() int {
	return d.cfg.RoomCapacity
}

func (d *definition) NewPending(ctx context.Context, r *http.Request, ws *websocket.Conn) (coreserver.Pending, error) {
	sess := session.NewWebSocket(ctx, ws)
	pending := &pendingPlayer{
		ctx:      ctx,
		session:  sess,
		player:   player.New(player.IDFromRequest(r), player.DefaultProfile(), player.NewHumanController()),
		messages: make(chan []byte, 64),
	}
	go pending.readPump()
	return pending, nil
}

func (d *definition) Place(hosted coreserver.Room, pending []coreserver.Pending) error {
	gr, ok := hosted.(*room.GameRoom)
	if !ok {
		return errors.New("unexpected room type")
	}

	members := make([]coreroom.Member, 0, len(pending))
	for _, item := range pending {
		pp, ok := item.(*pendingPlayer)
		if !ok {
			return errors.New("unexpected pending player")
		}
		members = append(members, coreroom.Member{Entity: pp.player, Session: pp.session})
	}
	return gr.JoinAll(members)
}

func (d *definition) Read(pending coreserver.Pending, hosted coreserver.Room) error {
	pp, ok := pending.(*pendingPlayer)
	if !ok {
		return errors.New("unexpected pending player")
	}
	if _, ok := hosted.(*room.GameRoom); !ok {
		return errors.New("unexpected room type")
	}
	for message := range pp.messages {
		d.handleClientMessage(pp.player, message)
	}
	return nil
}

func (d *definition) Mount(mux *http.ServeMux, host *coreserver.Matchmaker) {
	mux.HandleFunc(d.cfg.RoomEndpoint, host.CreateRoom)
	mux.HandleFunc(d.cfg.WebSocketEndpoint, host.Connect)
}

func (d *definition) NewRoom(ctx context.Context, cancel context.CancelFunc) (coreserver.Room, error) {
	return room.NewGameRoom(ctx, cancel, d.cfg), nil
}

func (d *definition) RoomFromRequest(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.URL.Query().Get("room"))
}

func (d *definition) Admit(r *http.Request, ws *websocket.Conn, hosted coreserver.Room) error {
	gr, ok := hosted.(*room.GameRoom)
	if !ok {
		return errors.New("unexpected room type")
	}

	sess := session.NewWebSocket(gr.Context(), ws)
	p := player.New(player.IDFromRequest(r), player.DefaultProfile(), player.NewHumanController())

	if err := gr.SubmitCommand(coreroom.JoinCommand{
		Member: coreroom.Member{Entity: p, Session: sess},
	}); err != nil {
		sess.Stop()
		return err
	}
	defer func() {
		if err := gr.SubmitCommand(coreroom.LeaveCommand{Key: p.Key()}); err != nil {
			log.Println(err)
		}
	}()

	return d.readWebSocket(sess, p, gr)
}

func (d *definition) readWebSocket(sess *session.WebSocket, p *player.Player, gr *room.GameRoom) error {
	for {
		msg, err := sess.Read(gr.Context())
		if err != nil {
			return err
		}
		d.handleClientMessage(p, msg)
	}
}

func (d *definition) handleClientMessage(p *player.Player, raw []byte) {
	var evt event.ClientEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		log.Printf("invalid message from player %s: %v", p.Id, err)
		return
	}
	if evt.Type != event.MsgPlayerKeyDown {
		return
	}

	var key event.KeyEvent
	if err := json.Unmarshal(evt.Data, &key); err != nil {
		log.Printf("invalid key event from player %s: %v", p.Id, err)
		return
	}
	p.ApplyKey(key.Key, key.State)
}
