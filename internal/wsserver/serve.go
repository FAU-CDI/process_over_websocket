package wsserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/httpx/websocket"
	"github.com/tkw1536/pkglib/recovery"
)

var (
	messageBufferSize = 5 // size for internal message buffers; should be > 1

	readCallTimeout = time.Second // timeout for reading the call parameters
)

var errUnknownSubprotocol = errors.New(proto.MessageWrongSubprotocol)

// Serve implements the websocket protocol.
//
// There are two kinds of messages:
//
// - text messages, which are used to send input and output.
// - binary messages, which are json-encoded and used for control flow.
//
// To call an action, a client should send a [proto.CallMessage] struct.
// The server will then start handling input and output (via text messages).
// If the client sends a [proto.SignalMessage], the signal is propagated to the underlying context.
//
// If the process suceeds, the server closes the websocket with the normal close message.
// If the process returns an error, the server closes the websocket with [proto.CloseReasonFailed] and an appropriate description of the error.
func Serve(handler proto.Handler, conn *websocket.Connection) (err error) {
	go func() {
		ctx := conn.Context()
		<-ctx.Done()
	}()
	// check that the client specified the correct subprotocol.
	if conn.Subprotocol() != proto.Subprotocol {
		conn.CloseWith(websocket.ClosePolicyViolation, proto.MessageWrongSubprotocol)
		return errUnknownSubprotocol
	}

	var wg sync.WaitGroup

	// once we have finished executing send a binary message (indicating success) to the client.
	defer func() {
		// close the underlying connection, and then wait for everything to finish!
		defer wg.Wait()

		// recover from any errors
		if e := recovery.Recover(recover()); e != nil {
			err = e
		}

		if err == nil {
			conn.CloseWith(websocket.CloseNormalClosure, "")
		} else {
			conn.CloseWith(proto.CloseReasonFailed, fmt.Sprint(err))
		}
	}()

	// create a channel for all future text messages
	// which will receive a nil to close
	var (
		textMessages   = make(chan []byte, messageBufferSize) // input text from the client
		initialMessage = make(chan []byte, 1)                 // initial binary message (only ever received once)
	)

	// create a context to be canceled once done
	ctx, cancel := context.WithCancelCause(conn.Context())
	defer cancel(proto.ErrCancelClientGone)

	// start processing messages
	wg.Add(1)
	go func() {
		defer wg.Done()

		defer close(textMessages)
		defer close(initialMessage)

		var (
			hadInitialMessage = false // did we send the initial binary message?
			hadCancelBefore   = false // did we receive the cancel signal previously?
		)

		for {
			select {
			case msg := <-conn.Read():
				// send a text message to the client
				// and ensure that it is never nil
				if msg.Type == websocket.TextMessage {
					if msg.Bytes == nil {
						msg.Bytes = []byte{}
					}
					textMessages <- msg.Bytes
					continue
				}

				// unknown message type received
				// just ignore it
				if msg.Type != websocket.BinaryMessage {
					continue
				}

				// if we didn't yet have the initial message
				// send it to the channel
				if !hadInitialMessage {
					initialMessage <- msg.Bytes
					hadInitialMessage = true
					continue
				}

				// attempt to decode signal message
				// and if we fail, cancel with a protocol error
				var signal proto.SignalMessage
				if err := json.Unmarshal(msg.Bytes, &signal); err != nil {
					cancel(proto.ErrCancelProtocolError)
					continue
				}

				switch {
				case signal.Signal == proto.SignalClose:
					// client has requested to close the text messages channel
					// so send a flag message (nil) to do the closing
					textMessages <- nil

				case signal.Signal == proto.SignalCancel && !hadCancelBefore:
					// client canceled for the first time
					cancel(proto.ErrCancelClientRequest)
					hadCancelBefore = true
				case signal.Signal == proto.SignalCancel && hadCancelBefore:
					// client canceled for the second time
					// so we also close the input channel
					textMessages <- nil
					hadCancelBefore = true

				default:
					// some unknown signal was sent
					// this is a protocol error
					cancel(proto.ErrCancelProtocolError)
				}

			case <-conn.Context().Done():
				cancel(proto.ErrCancelClientGone)
				return
			}
		}
	}()

	// read the call message
	var call proto.CallMessage
	select {
	case buffer := <-initialMessage:

		// try to read the protocol message.
		// and if we fail to unmarshal it, fail with a protocol error
		if err := json.Unmarshal(buffer, &call); err != nil {
			return proto.ErrCancelProtocolError
		}

	case <-time.After(readCallTimeout):
		return proto.ErrCancelTimeout
	}

	// Find the right process
	process, err := handler.Get(conn.Request(), call.Call, call.Params...)
	if err != nil {
		return err
	}

	// create a pipe to handle the input
	reader, writer := io.Pipe()
	defer writer.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for text := range textMessages {
			if text == nil {
				goto nomore
			}
			writer.Write(text)
		}

	nomore:
		writer.Close()

		// drain channel
		for range textMessages {
		}
	}()

	// write the output to the client as it comes in!
	// NOTE(twiesing): We may eventually need buffering here ...
	output := WriterFunc(func(b []byte) (int, error) {
		<-conn.WriteText(string(b))
		return len(b), nil
	})

	// do the actual processing
	return process.Do(ctx, reader, output, call.Params...)
}

// WriterFunc implements io.Writer using a function.
type WriterFunc func([]byte) (int, error)

func (wf WriterFunc) Write(b []byte) (int, error) {
	return wf(b)
}
