package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	DefaultReadTimeout  = 10 * time.Second
	DefaultWriteTimeout = 10 * time.Second
)

type Room interface {
	RoomID() uuid.UUID
	RunRoom(ctx context.Context, onClose func())
}

type Definition interface {
	Mount(mux *http.ServeMux, host *Matchmaker)
	NewRoom(ctx context.Context, cancel context.CancelFunc) (Room, error)
	RoomFromRequest(r *http.Request) (uuid.UUID, error)
	Admit(r *http.Request, ws *websocket.Conn, room Room) error
	Capacity() int
	NewPending(ctx context.Context, r *http.Request, ws *websocket.Conn) (Pending, error)
	Place(room Room, pending []Pending) error
	Read(pending Pending, room Room) error
}

type Options struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	AssetsDir         string
	AssetContentTypes map[string]string
	WebSocketEndpoint string
}

type Matchmaker struct {
	HttpServer *http.Server

	mux  *http.ServeMux
	def  Definition
	opts Options

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	rooms map[uuid.UUID]Room

	pending   *pendingQueue
	formation sync.Mutex

	listener net.Listener
}

func New(opts Options, def Definition) (*Matchmaker, <-chan error, error) {
	if def.Capacity() < 1 {
		return nil, nil, errors.New("matchmaker requires a positive capacity")
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = DefaultReadTimeout
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = DefaultWriteTimeout
	}

	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("listening on http://%v", listener.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	host := &Matchmaker{
		mux:      http.NewServeMux(),
		def:      def,
		opts:     opts,
		ctx:      ctx,
		cancel:   cancel,
		rooms:    make(map[uuid.UUID]Room),
		pending:  newPendingQueue(),
		listener: listener,
	}
	host.mux.Handle("/", assetHandler(opts.AssetsDir, opts.AssetContentTypes))
	def.Mount(host.mux, host)

	log.Printf("listening websocket on ws://%v%v", listener.Addr(), opts.WebSocketEndpoint)

	host.HttpServer = &http.Server{
		Handler:      host,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}

	errChannel := make(chan error, 1)
	go func() {
		errChannel <- host.HttpServer.Serve(listener)
	}()
	return host, errChannel, nil
}

func (h *Matchmaker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Matchmaker) Context() context.Context {
	return h.ctx
}

func (h *Matchmaker) Addr() net.Addr {
	return h.listener.Addr()
}

func (h *Matchmaker) Shutdown(ctx context.Context) error {
	h.cancel()
	return h.HttpServer.Shutdown(ctx)
}

func (h *Matchmaker) CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	roomCtx, roomCancel := context.WithCancel(h.ctx)
	room, err := h.def.NewRoom(roomCtx, roomCancel)
	if err != nil {
		roomCancel()
		log.Printf("failed to create room: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.store(room)
	go room.RunRoom(h.ctx, func() {
		h.remove(room.RoomID())
	})

	txt, err := room.RoomID().MarshalText()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(txt)
}

func (h *Matchmaker) Connect(w http.ResponseWriter, r *http.Request) {
	ws, wsErr := websocket.Accept(w, r, nil)
	if wsErr == nil {
		wsErr = h.serveConnection(r, ws)
	}
	if wsErr == nil {
		return
	}

	if errors.Is(wsErr, context.Canceled) {
		if ws != nil {
			_ = ws.Close(websocket.StatusGoingAway, "room closed")
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
	_ = ws.Close(websocket.StatusBadGateway, wsErr.Error())
}

func (h *Matchmaker) serveConnection(r *http.Request, ws *websocket.Conn) error {
	if r.URL.Query().Has("room") {
		return h.admit(r, ws)
	}

	pending, err := h.def.NewPending(h.ctx, r, ws)
	if err != nil {
		return err
	}
	room, err := h.MakeMatch(pending)
	if err != nil {
		return err
	}
	return h.def.Read(pending, room)
}

func (h *Matchmaker) admit(r *http.Request, ws *websocket.Conn) error {
	roomID, err := h.def.RoomFromRequest(r)
	if err != nil {
		return err
	}
	room, exists := h.load(roomID)
	if !exists {
		return errors.New("game room not found")
	}
	return h.def.Admit(r, ws, room)
}

func (h *Matchmaker) store(room Room) {
	h.mu.Lock()
	h.rooms[room.RoomID()] = room
	h.mu.Unlock()
}

func (h *Matchmaker) remove(id uuid.UUID) {
	h.mu.Lock()
	delete(h.rooms, id)
	h.mu.Unlock()
}

func (h *Matchmaker) load(id uuid.UUID) (Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[id]
	return room, ok
}

func (h *Matchmaker) roomCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.rooms)
}
