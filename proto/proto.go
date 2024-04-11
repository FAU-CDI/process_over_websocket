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
)

// ResultMessage is sent by the server to the client to report the success of a remote procedure
type ResultMessage struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
