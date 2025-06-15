//spellchecker:words rest impl
package rest_impl

//spellchecker:words context encoding json errors http sync github process over websocket internal finbuf proto pkglib recovery
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/FAU-CDI/process_over_websocket/internal/finbuf"
	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/recovery"
)

// Session holds information about an ongoing process.
//
//nolint:containedctx
type Session struct {
	// m protects changing of state
	m     sync.RWMutex
	stage stage

	// handler and call hold the original call
	// used to initiate this session
	handler proto.Handler
	call    proto.CallMessage

	// context and cancel can be used to cancel the underlying process
	context context.Context
	cancel  context.CancelCauseFunc

	// done is closed once the process has returned
	done chan struct{}

	// input to the session
	inr *io.PipeReader
	inw *io.PipeWriter

	// out holds the output of this session
	out finbuf.FiniteBuffer

	// result of the process
	result any
	err    error
}

type stage uint8

const (
	stageInit stage = iota
	stageRunning
	stageFinished
)

type SessionOpts struct {
	MaxLines int
}

const minMaxLines = 1000

func (opt *SessionOpts) SetDefaults() {
	if opt.MaxLines < minMaxLines {
		opt.MaxLines = minMaxLines
	}
}

// Init initializes this session, preparing it for accepting a new session.
func (session *Session) Init(handler proto.Handler, ctx context.Context, opt SessionOpts) {
	opt.SetDefaults()

	session.out.MaxLines = opt.MaxLines
	session.handler = handler

	session.context, session.cancel = context.WithCancelCause(ctx)
	session.done = make(chan struct{})

	session.inr, session.inw = io.Pipe()
}

// Start starts the given call in this session.
func (session *Session) Start(r *http.Request, call proto.CallMessage) bool {
	session.m.Lock()
	defer session.m.Unlock()

	// we only work in the initial stage
	if session.stage != stageInit {
		return false
	}

	// and we're now in the running stage
	session.stage = stageRunning
	session.call = call
	go session.run(r)

	return true
}

var errPanic = errors.New("panic() in process")

func (session *Session) run(r *http.Request) {
	var res any
	var err = errPanic

	defer close(session.done)
	defer func() {
		if e := recovery.Recover(recover()); e != nil {
			err = e
		}

		session.m.Lock()
		defer session.m.Unlock()

		session.result = res
		session.err = err
		session.stage = stageFinished
	}()
	defer session.cancel(proto.ErrCancelHandlerReturn)
	defer func() { _ = session.inw.Close() }()

	res, err = func() (any, error) {
		// get the handler
		process, err := session.handler.Get(r, session.call.Call, session.call.Params...)
		if err != nil {
			return nil, fmt.Errorf("failed to get process: %w", err)
		}

		// and do the call
		return process.Do(session.context, session.inr, &session.out, session.call.Params...)
	}()
}

// CloseInput closes the input of the session.
func (session *Session) CloseInput() error {
	return errors.Join(
		session.inw.Close(),
		session.inr.Close(),
	)
}

func (session *Session) Write(data []byte) (int, error) {
	n, err := session.inw.Write(data)
	if err != nil {
		return n, fmt.Errorf("failed to write input: %w", err)
	}
	return n, nil
}

// Wait waits for this session to complete, and then returns it's result and error.
// If context closes before the session is completed, immediately returns the context's error.
func (session *Session) Wait(ctx context.Context) (result any, err error) {
	select {
	case <-session.done:
		return session.result, session.err
	case <-ctx.Done():
		//nolint:wrapcheck
		return nil, ctx.Err()
	}
}

// CloseWith cancels the session with the given error.
func (session *Session) CloseWith(err error) {
	func() {
		session.m.RLock()
		defer session.m.RUnlock()

		if session.cancel != nil {
			session.cancel(err)
		}
		_ = session.CloseInput() // TODO: not sure what to do with this error
	}()

	<-session.done
}

// Stage returns information about the current stage of the session.
//
// Running indicates if the session is currently running its associated process.
// Started indicates if the session process was started previously.
func (session *Session) Stage() (running, started bool) {
	session.m.RLock()
	defer session.m.RUnlock()

	switch session.stage {
	case stageInit:
		return false, false
	case stageRunning:
		return true, false
	case stageFinished:
		return false, true
	}
	panic("never reached")
}

type Status struct {
	Buffer string
	Result *proto.Result
}

type statusJSON struct {
	Buffer string          `json:"buffer,omitempty"`
	Result json.RawMessage `json:"result"`
}

func (status Status) MarshalJSON() ([]byte, error) {
	var data statusJSON
	var err error

	data.Buffer = status.Buffer
	data.Result, err = status.Result.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result as json: %w", err)
	}

	res, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	return res, nil
}

// Status returns the status.
func (session *Session) Status() Status {
	session.m.RLock()
	defer session.m.RUnlock()

	switch session.stage {
	case stageInit:
		return Status{}
	case stageRunning:
		return Status{
			Result: nil,
			Buffer: session.out.String(),
		}
	case stageFinished:
		return Status{
			Result: &proto.Result{
				Value:  session.result,
				Reason: session.err,
			},
			Buffer: session.out.String(),
		}
	}

	panic("never reached")
}
