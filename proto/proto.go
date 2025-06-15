// Package proto holds structs used by the protocol.
//
//spellchecker:words proto
package proto

//spellchecker:words encoding json
import (
	"encoding/json"
	"fmt"
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

type Result struct {
	Value  any
	Reason error
}

// MarshalJSON marshals this result as a message.
func (res *Result) MarshalJSON() ([]byte, error) {
	if res == nil {
		return []byte(`{"status":"pending"}`), nil
	}

	status := "rejected"
	data := "reason"
	if res.Reason == nil {
		status = "fulfilled"
		data = "value"
	}

	content := (func() string {
		defer func() {
			_ = recover() // ignore any panic()s during the marshal
		}()

		// find the object to marshal
		obj := res.Value
		if res.Reason != nil {
			obj = fmt.Sprint(res.Reason)
		}

		// format the value
		bytes, err := json.Marshal(obj)
		if err != nil {
			return ""
		}
		return string(bytes)
	})()

	if len(content) == 0 {
		return []byte(`{"status":"` + status + `"}`), nil
	}
	return []byte(`{"status":"` + status + `","` + data + `":` + content + `}`), nil
}
