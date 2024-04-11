package proto

import (
	"context"
	"errors"
	"io"
	"net/http"
)

type Handler interface {
	// Get gets the process when the client requests a specific process.
	//
	// The request holds the original request that the client sent during the upgrade.
	// It may return the special ErrHandlerUnknownProcess constants if something goes wrong.
	Get(r *http.Request, name string, args ...string) (Process, error)
}

// HandlerFunc implements Handler
type HandlerFunc func(r *http.Request, name string, args ...string) (Process, error)

func (hf HandlerFunc) Get(r *http.Request, name string, args ...string) (Process, error) {
	return hf(r, name, args...)
}

var (
	ErrHandlerUnknownProcess      = errors.New("unknown process")
	ErrHandlerInvalidArgs         = errors.New("invalid args")
	ErrHandlerAuthorizationDenied = errors.New("authorization denied")
)

// Process represents a process handled by the protocol
type Process interface {
	// Do starts a process and exists once it is complete.
	//
	// Input and Output are piped dynamically to and from the client.
	// args are the initial arguments provided by the client.
	//
	// The context is cancelled once the client is no longer available.
	// It's CancelCause is one of the special ErrCancel* constants in this package.
	Do(ctx context.Context, input io.Reader, output io.Writer, args ...string) error
}

// ProcessFunc implements Process
type ProcessFunc func(ctx context.Context, input io.Reader, output io.Writer, args ...string) error

func (pf ProcessFunc) Do(ctx context.Context, input io.Reader, output io.Writer, args ...string) error {
	return pf(ctx, input, output, args...)
}

var (
	// ErrCancelClientGone indicates that the client has gone away.
	// Typically this indicates that the websocket connection has died, or that the timeout failed to ping the server without a specific time frame.
	ErrCancelClientGone = errors.New("client has gone away")

	// CancelClientRequest indicates that the client has explicitly requested cancellation.
	ErrCancelClientRequest = errors.New("client requested cancellation")

	// ErrCancelProtocolError indicates that a protocol error occurred and the process should be cancelled for safety reasons.
	ErrCancelProtocolError = errors.New("protocol error occured")

	// CancelTimeout indicates that a process timeout has occurred.
	// This must have been requested by the client at an explicit time.
	ErrCancelTimeout = errors.New("timeout expired")
)
