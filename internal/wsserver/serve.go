package wsserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/httpx/websocket"
	"github.com/tkw1536/pkglib/recovery"
)

var (
	messageBufferSize = 5 // size for internal message buffers; should be > 1

	readCallTimeout = 100 * time.Second // timeout for reading the call parameters

	errProtocolError   = errors.New("protocol error")
	errReadCallTimeout = errors.New("timeout reading call parameters")
)

var (
	errUnknownSubprotocol = errors.New("unknown subprotocol")
	msgUnknownSubprotocol = websocket.NewTextMessage(errUnknownSubprotocol.Error()).MustPrepare()
)

// checkSubprotocol checks that the connection specifies the right subprotocol
// TODO: Enforce one here
func checkSubprotocol(conn *websocket.Connection) error {
	if conn.Subprotocol() != "" {
		<-conn.WritePrepared(msgUnknownSubprotocol)
		return errUnknownSubprotocol
	}

	return nil
}

// Serve implements the websocket version 1 of the protocol.
// It is frozen and should not be changed.
//
// There are two kinds of messages:
//
// - text messages, which are used to send input and output.
// - binary messages, which are json-encoded and used for control flow.
//
// To call an action, a client should send a [proto.CallMessage] struct.
// The server will then start handling input and output (via text messages).
// If the client sends a [proto.SignalMessage], the signal is propragnated to the underlying context.
// Finally it will send a [proto.ResultMessage] once handling is complete.
func Serve(handler proto.Handler, conn *websocket.Connection) (err error) {
	// check that the right subprotocol is set, or bail out
	if err := checkSubprotocol(conn); err != nil {
		return err
	}

	var wg sync.WaitGroup

	// once we have finished executing send a binary message (indicating success) to the client.
	defer func() {
		// close the underlying connection, and then wait for everything to finish!
		defer wg.Wait()
		defer conn.Close()

		// recover from any errors
		if e := recovery.Recover(recover()); e != nil {
			err = e
		}

		// generate a result message
		var result proto.ResultMessage
		if err == nil {
			result.Success = true
		} else {
			result.Success = false
			result.Message = err.Error()
			if result.Message == "" {
				result.Message = "unspecified error"
			}
		}

		// encode the result message to json!
		var message websocket.Message
		message.Type = websocket.BinaryMessage
		message.Bytes, err = json.Marshal(result)

		// silently fail if the message fails to encode
		// although this should not happen
		if err != nil {
			return
		}

		// and tell the client about it!
		<-conn.Write(message)
	}()

	// create channels to receive text and bytes messages
	textMessages := make(chan string, messageBufferSize)
	binaryMessages := make(chan []byte, messageBufferSize)

	// start reading text and binary messages
	// and redirect everything to the right channels
	wg.Add(1)
	go func() {
		defer wg.Done()

		defer close(textMessages)
		defer close(binaryMessages)

		for {
			select {
			case msg := <-conn.Read():
				if msg.Type == websocket.TextMessage {
					textMessages <- string(msg.Bytes)
				}
				if msg.Type == websocket.BinaryMessage {
					binaryMessages <- msg.Bytes
				}
			case <-conn.Context().Done():
				return
			}
		}

	}()

	// read the call message
	var call proto.CallMessage
	select {
	case buffer := <-binaryMessages:
		if err := json.Unmarshal(buffer, &call); err != nil {
			return errProtocolError
		}

	case <-time.After(readCallTimeout):
		return errReadCallTimeout
	}

	// Find the right process
	process, err := handler.Get(conn.Request(), call.Call, call.Params...)
	if err != nil {
		return err
	}

	// create a context to be canceled once done
	ctx, cancel := context.WithCancelCause(conn.Context())
	defer cancel(proto.ErrCancelClientGone)

	// handle any signal messages
	wg.Add(1)
	go func() {
		defer wg.Done()
		var signal proto.SignalMessage

		for binary := range binaryMessages {
			signal.Signal = ""

			// read the signal message
			if err := json.Unmarshal(binary, &signal); err != nil {
				continue
			}

			// if we got a cancel message, do the cancellation!
			if signal.Signal == proto.SignalCancel {
				cancel(proto.ErrCancelClientRequest)
			}
		}
	}()

	// create a pipe to handle the input
	// and start handling it
	var inputR, inputW = io.Pipe()
	defer inputW.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for text := range textMessages {
			inputW.Write([]byte(text))
		}
	}()

	// write the output to the client as it comes in!
	// NOTE(twiesing): We may eventually need buffering here ...
	output := WriterFunc(func(b []byte) (int, error) {
		<-conn.WriteText(string(b))
		return len(b), nil
	})

	// do the actual processing
	return process.Do(ctx, inputR, output, call.Params...)
}

// WriterFunc implements io.Writer using a function.
type WriterFunc func([]byte) (int, error)

func (wf WriterFunc) Write(b []byte) (int, error) {
	return wf(b)
}
