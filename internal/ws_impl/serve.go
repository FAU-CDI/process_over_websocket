//spellchecker:words impl
package ws_impl

//spellchecker:words context encoding json http strings sync time github process over websocket internal clean proto pkglib errorsx websocketx
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FAU-CDI/process_over_websocket/internal/clean"
	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/errorsx"
	"github.com/tkw1536/pkglib/websocketx"
)

var (
	messageBufferSize = 5 // size for internal message buffers; should be > 1

	readCallTimeout = time.Second // timeout for reading the call parameters
)

type Options = websocketx.Options

// NewServer creates a new server to handle websocket connections.
func NewServer(path string, handler proto.Handler, fallback http.Handler, options Options) *Server {
	server := &Server{
		path: clean.Clean(path),
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
	path    string
	server  websocketx.Server
	handler proto.Handler
}

// ServeHTTP implements handling the protocol.
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !server.isValidRequestPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	server.server.ServeHTTP(w, r)
}

func (server *Server) isValidRequestPath(path string) bool {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return strings.HasPrefix(path, server.path)
}

func (server *Server) handle(conn *websocketx.Connection) {
	_, _ = server.serve(conn)
}

var errUnknown = errors.New("unknown error")

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

		// value from any panics
		if value := recover(); value != nil {
			err = fmt.Errorf("%w: %v", errUnknown, value)
		}

		// assemble the close message
		result := proto.Result{Value: res, Reason: err}
		data, _ := result.MarshalJSON()

		// if the connection is already done, bail out!
		if conn.Context().Err() != nil {
			return
		}

		// write the close data
		if err := conn.Write(websocketx.NewBinaryMessage(data)); err != nil {
			return
		}

		// and close the connection
		conn.ShutdownWith(websocketx.CloseFrame{})
	}()

	// create a channel for all future text messages
	// which will receive a nil to close
	var (
		textMessages   = make(chan []byte, messageBufferSize) // input text from the client
		initialMessage = make(chan []byte, 1)                 // initial binary message (only ever received once)
	)

	// create a context to be canceled once done
	ctx, cancel := context.WithCancelCause(conn.Context())
	defer cancel(proto.ErrCancelHandlerReturn)

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
		return nil, fmt.Errorf("failed to get process: %w", err)
	}

	// create a pipe to handle the input
	reader, writer := io.Pipe()
	defer errorsx.Close(writer, &err, "writer")

	// TODO: this ignores a whole bunch of errors.
	// not sure what to do with them.
	wg.Add(1)
	go func() {
		defer wg.Done()

		for text := range textMessages {
			if text == nil {
				goto no_more
			}
			_, _ = writer.Write(text)
		}

	no_more:
		_ = writer.Close()

		// drain channel
		for range textMessages {
		}
	}()

	// write the output to the client as it comes in!
	// TODO: We may eventually need buffering here ...
	output := WriterFunc(func(b []byte) (int, error) {
		if err := conn.WriteText(string(b)); err != nil {
			return 0, fmt.Errorf("failed to write to connection: %w", err)
		}
		return len(b), nil
	})

	// do the actual processing
	value, err := process.Do(ctx, reader, output, call.Params...)
	if err != nil {
		return nil, fmt.Errorf("process returned error: %w", err)
	}
	return value, nil
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
