//spellchecker:words impl
package ws_impl

//spellchecker:words context encoding json http sync time github process over websocket proto pkglib recovery websocketx
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/recovery"
	"github.com/tkw1536/pkglib/websocketx"
)

var (
	messageBufferSize = 5 // size for internal message buffers; should be > 1

	readCallTimeout = time.Second // timeout for reading the call parameters
)

type Options = websocketx.Options

// NewServer creates a new server to handle websocket connections.
func NewServer(handler proto.Handler, fallback http.Handler, options Options) *Server {
	server := &Server{
		server: websocketx.Server{
			Options:  options,
			Fallback: fallback,
		},
		handler: handler,
	}

	// setup the handler for the server
	server.server.Handler = server.handle

	// set up the subprotocols
	server.server.Options.Subprotocols = []string{proto.Subprotocol}
	server.server.RequireProtocols()

	// return the server
	return server
}

// Server implements the websocket protocol.
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
// If nothing unexpected happens (e.g. an abnormal closure from the client), the server will close the connection and send a
// [proto.ResultMessage] to the client.
type Server struct {
	server  websocketx.Server
	handler proto.Handler
}

// ServeHTTP implements handling the protocol
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.server.ServeHTTP(w, r)
}

func (server *Server) handle(conn *websocketx.Connection) {
	_, _ = server.serve(conn)
}

func (server *Server) serve(conn *websocketx.Connection) (res any, err error) {
	// check that the client specified the correct subprotocol.
	if conn.Subprotocol() != proto.Subprotocol {
		panic("server did not enforce subprotocol")
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

		// assemble the close message
		// and return it
		var msg proto.ResultMessage
		if err == nil {
			msg = proto.ResultMessageSuccess(res)
		} else {
			msg = proto.ResultMessageFailure(err)
		}
		conn.ShutdownWith(msg.Frame())
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
				if msg.Text() {
					if msg.Body == nil {
						msg.Body = []byte{}
					}
					textMessages <- msg.Body
					continue
				}

				// unknown message type received
				// just ignore it
				if !msg.Binary() {
					continue
				}

				// if we didn't yet have the initial message
				// send it to the channel
				if !hadInitialMessage {
					initialMessage <- msg.Body
					hadInitialMessage = true
					continue
				}

				// attempt to decode signal message
				// and if we fail, cancel with a protocol error
				var signal proto.SignalMessage
				if err := json.Unmarshal(msg.Body, &signal); err != nil {
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
			return nil, proto.ErrCancelProtocolError
		}

	case <-time.After(readCallTimeout):
		return nil, proto.ErrCancelTimeout
	}

	// Find the right process
	process, err := server.handler.Get(conn.Request(), call.Call, call.Params...)
	if err != nil {
		return nil, err
	}

	// create a pipe to handle the input
	reader, writer := io.Pipe()
	defer writer.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for text := range textMessages {
			if text == nil {
				goto no_more
			}
			writer.Write(text)
		}

	no_more:
		writer.Close()

		// drain channel
		for range textMessages {
		}
	}()

	// write the output to the client as it comes in!
	// TODO: We may eventually need buffering here ...
	output := WriterFunc(func(b []byte) (int, error) {
		conn.WriteText(string(b))
		return len(b), nil
	})

	// do the actual processing
	return process.Do(ctx, reader, output, call.Params...)
}

func (server *Server) Close() {
	server.server.Close()
}

func (server *Server) Shutdown() {
	server.server.Shutdown()
}

// WriterFunc implements io.Writer using a function.
type WriterFunc func([]byte) (int, error)

func (wf WriterFunc) Write(b []byte) (int, error) {
	return wf(b)
}
