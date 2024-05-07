// Package proto holds structs used by the protocol.
//
//spellchecker:words proto
package proto

//spellchecker:words encoding json github pkglib websocketx
import (
	"encoding/json"
	"fmt"

	"github.com/tkw1536/pkglib/websocketx"
)

// CallMessage is sent by the client to the server to invoke a remote procedure.
type CallMessage struct {
	Call   string   `json:"call"`
	Params []string `json:"params,omitempty"`
}

// SignalMessage is sent from the client to the server to stop the current procedure.
type SignalMessage struct {
	Signal Signal `json:"signal"`
}

type Signal string

const (
	SignalCancel Signal = "cancel"
	SignalClose  Signal = "close"
)

// Subprotocol is the mandatory subprotocol to be used by the websocket client.
const Subprotocol = "pow-1"

var ErrWrongSubprotocol = fmt.Errorf("only support subprotocol %q", Subprotocol)

// ResultMessage encapsulates the result of a process.
// See [ResultMessageSuccess] and [ResultMessageFailure]
type ResultMessage struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

func (rm ResultMessage) Frame() websocketx.CloseFrame {
	data, err := json.Marshal(rm)
	if err != nil {
		return websocketx.CloseFrame{
			Code:   websocketx.StatusInternalErr,
			Reason: "error encoding ResultMessage",
		}
	}
	return websocketx.CloseFrame{
		Code:   websocketx.StatusNormalClosure,
		Reason: string(data),
	}
}

// ResultMessageFailure formats a message for a failed process.
func ResultMessageFailure(err error) ResultMessage {
	return ResultMessage{
		Success: false,
		Data:    fmt.Sprint(err),
	}
}

// ResultMessageSuccess formats a message for a succeeded process.
func ResultMessageSuccess(data any) ResultMessage {
	return ResultMessage{
		Success: true,
		Data:    data,
	}
}
