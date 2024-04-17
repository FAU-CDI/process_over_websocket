// Package proto holds structs used by the protocol.
package proto

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
const MessageWrongSubprotocol = "only support subprotocol 'pow-1'"

// CloseReasonFailed is the code with which the websocket is closed if an error occurs.
const CloseReasonFailed = 3000
