//spellchecker:words proto
package proto

//spellchecker:words context errors http
import (
	"context"
	"errors"
	"io"
	"net/http"
)

type Handler interface {
	// Get gets the process when the client requests a specific process.
	//
	// The request holds the original request that the client sent to start the process.
	// This may be a websocket upgrade request or whatever the client sent during the upgrade.
	//
	// It may return the special ErrHandlerUnknownProcess constants if something goes wrong.
	Get(r *http.Request, name string, args ...string) (Process, error)
}

// HandlerFunc implements Handler.
type HandlerFunc func(r *http.Request, name string, args ...string) (Process, error)

func (hf HandlerFunc) Get(r *http.Request, name string, args ...string) (Process, error) {
	return hf(r, name, args...)
}

var (
	ErrHandlerUnknownProcess      = errors.New("unknown process")
	ErrHandlerInvalidArgs         = errors.New("invalid args")
	ErrHandlerAuthorizationDenied = errors.New("authorization denied")
)

// Process represents a process handled by the protocol.
type Process interface {
	// Do starts a process and exists once it is complete.
	//
	// Input and Output are piped dynamically to and from the client.
	// args are the initial arguments provided by the client.
	//
	// The context is cancelled once the client is no longer available.
	// It's CancelCause is one of the special ErrCancel* constants in this package.
	Do(ctx context.Context, input io.Reader, output io.Writer, args ...string) (any, error)
}

// ProcessFunc implements Process.
type ProcessFunc func(ctx context.Context, input io.Reader, output io.Writer, args ...string) (any, error)

func (pf ProcessFunc) Do(ctx context.Context, input io.Reader, output io.Writer, args ...string) (any, error) {
	return pf(ctx, input, output, args...)
}

var (
	// ErrCancelClientGone indicates that the client has gone away.
	// Typically this indicates that the websocket connection has died, or that the timeout failed to ping the server without a specific time frame.
	ErrCancelClientGone = errors.New("client has gone away")

	// ErrCancelHandlerReturn indicates that the context is closed because the process handler has returned.
	// Note that at this point any error handling code may be used to change server-side state, but this can no longer effect the client.
	ErrCancelHandlerReturn = errors.New("handler has returned")

	// CancelClientRequest indicates that the client has explicitly requested cancellation.
	ErrCancelClientRequest = errors.New("client requested cancellation")

	// ErrCancelProtocolError indicates that a protocol error occurred and the process should be cancelled for safety reasons.
	ErrCancelProtocolError = errors.New("protocol error occurred")

	// This must have been requested by the client at an explicit time.
	ErrCancelTimeout = errors.New("timeout expired")
)
