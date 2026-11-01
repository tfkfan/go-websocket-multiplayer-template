package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/event"
	"github.com/tfkfan/gogame/internal/game/player"
	"github.com/tfkfan/gogame/internal/game/room"
	"github.com/tfkfan/gogame/internal/network"
)

type GameServer struct {
	HttpServer *http.Server

	serveMux     http.ServeMux
	serverCtx    context.Context
	serverCancel context.CancelFunc
	gameServerMu sync.Mutex

	cfg *config.Config

	rooms map[uuid.UUID]*room.GameRoom
}

func NewGameServer(cfg *config.Config) (*GameServer, chan error, error) {
	listener, err := net.Listen("tcp", cfg.Port)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("listening on http://%v", listener.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	srv := &GameServer{
		rooms:        make(map[uuid.UUID]*room.GameRoom),
		serverCtx:    ctx,
		serverCancel: cancel,
		cfg:          cfg,
	}
	srv.serveMux.Handle("/", assetHandler(cfg.AssetsDir))
	srv.serveMux.HandleFunc(cfg.WebSocketEndpoint, srv.handleWsConnection)
	srv.serveMux.HandleFunc(cfg.RoomEndpoint, srv.handleRoom)

	log.Printf("listening websocket on ws://%v%v", listener.Addr(), cfg.WebSocketEndpoint)

	httpSrv := &http.Server{
		Handler:      srv,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	srv.HttpServer = httpSrv

	errChannel := make(chan error, 1)
	go func() {
		errChannel <- httpSrv.Serve(listener)
	}()
	return srv, errChannel, nil
}

func (srv *GameServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.serveMux.ServeHTTP(w, r)
}

func (srv *GameServer) Shutdown(ctx context.Context) error {
	srv.serverCancel()
	return srv.HttpServer.Shutdown(ctx)
}

func (srv *GameServer) handleWsConnection(w http.ResponseWriter, r *http.Request) {
	ws, wsErr := websocket.Accept(w, r, nil)
	if wsErr == nil {
		wsErr = srv.tryConnect(r, ws)
	}
	if wsErr == nil {
		return
	}

	if errors.Is(wsErr, context.Canceled) {
		if ws != nil {
			network.CloseWebSocket(ws, websocket.StatusGoingAway, "room closed")
		}
		return
	}

	switch websocket.CloseStatus(wsErr) {
	case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		return
	}

	if ws == nil {
		log.Printf("error on websocket accept: %v", wsErr)
		return
	}
	log.Printf("error on websocket join: %v", wsErr)
	network.CloseWebSocket(ws, websocket.StatusBadGateway, wsErr.Error())
}

func (srv *GameServer) tryConnect(r *http.Request, ws *websocket.Conn) error {
	roomID, err := uuid.Parse(r.URL.Query().Get("room"))
	if err != nil {
		return err
	}
	gr, exists := srv.gameRoom(roomID)
	if !exists {
		return errors.New("game room not found")
	}

	p, err := player.ExtractPlayerFromRequest(r, ws, gr.Context())
	if err != nil {
		return err
	}

	if err := gr.SubmitCommand(room.JoinCommand{
		Player: p,
	}); err != nil {
		p.Stop()
		return err
	}
	defer func() {
		e := gr.SubmitCommand(room.LeaveCommand{Player: p})
		if e != nil {
			log.Println(e)
		}
	}()

	return srv.ReadWebSocket(p, gr)
}

func (srv *GameServer) ReadWebSocket(p *player.Player, gr *room.GameRoom) error {
	for {
		msg, err := p.Read(gr.Context())
		if err != nil {
			return err
		}
		srv.handleClientMessage(p, msg)
	}
}

func (srv *GameServer) handleClientMessage(p *player.Player, raw []byte) {
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

func (srv *GameServer) gameRoom(id uuid.UUID) (*room.GameRoom, bool) {
	srv.gameServerMu.Lock()
	defer srv.gameServerMu.Unlock()

	gr, ok := srv.rooms[id]
	return gr, ok
}

func (srv *GameServer) handleRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	gameRoomContext, gameRoomCancel := context.WithCancel(srv.serverCtx)

	gr := room.NewGameRoom(gameRoomContext, gameRoomCancel, srv.cfg)

	srv.gameServerMu.Lock()
	srv.rooms[gr.Id] = gr
	srv.gameServerMu.Unlock()

	go gr.Run(srv.serverCtx, func(room *room.BaseRoom) {
		srv.DeleteGameRoom(room.Id)
	})

	txt, err := gr.Id.MarshalText()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(txt); err != nil {
		return
	}
}

func (srv *GameServer) DeleteGameRoom(id uuid.UUID) {
	srv.gameServerMu.Lock()
	defer srv.gameServerMu.Unlock()
	delete(srv.rooms, id)
}
