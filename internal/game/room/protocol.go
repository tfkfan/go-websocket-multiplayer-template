package room

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tfkfan/gogame/internal/game/player"
)

type MessageType int

const (
	MsgJoin          MessageType = 2
	MsgJoinSuccess   MessageType = 4
	MsgRoomClose     MessageType = 9
	MsgFailure       MessageType = 11
	MsgUpdate        MessageType = 100
	MsgPrivate       MessageType = 110
	MsgPlayerKeyDown MessageType = 200
)

type ClientEvent struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

type KeyEvent struct {
	Key   string `json:"key"`
	State bool   `json:"state"`
}

type JoinSuccessData struct {
	PlayerID uuid.UUID `json:"playerId"`
	RoomID   uuid.UUID `json:"roomId"`
	Width    float64   `json:"width"`
	Height   float64   `json:"height"`
	Radius   float64   `json:"radius"`
	Color    string    `json:"color"`
	TickRate int       `json:"tickRate"`
}

type UpdateData struct {
	Players []player.State `json:"players"`
	Width   float64        `json:"width"`
	Height  float64        `json:"height"`
}

type FailureData struct {
	Message string `json:"message"`
}

type outboundEvent struct {
	Type MessageType `json:"type"`
	Data any         `json:"data"`
}

func marshalEvent(messageType MessageType, data any) ([]byte, error) {
	return json.Marshal(outboundEvent{Type: messageType, Data: data})
}
