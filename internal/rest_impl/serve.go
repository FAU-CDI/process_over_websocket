//spellchecker:words rest impl
package rest_impl

//spellchecker:words context encoding json errors http sync time github process over websocket proto google uuid gorilla internal vapor
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/FAU-CDI/process_over_websocket/internal/vapor"
)

// NewServer creates a new rest server implementation
func NewServer(handler proto.Handler, options Options) *Server {
	return &Server{
		handler: handler,
		options: Options{},
	}
}

// Options are the options for a rest server
type Options struct {
	// timeout after which new elements are automatically removed
	Timeout time.Duration

	// options for the session
	Session SessionOpts
}

const minTimeout = time.Minute

func (opt *Options) SetDefaults() {
	if opt.Timeout < minTimeout {
		opt.Timeout = minTimeout
	}
}

type Server struct {
	init sync.Once // called once for initialization

	// global context for processes
	context context.Context
	cancel  context.CancelCauseFunc

	mux   http.ServeMux
	vapor vapor.Vapor[Session]

	options Options
	handler proto.Handler
}

func (server *Server) doInit() {
	server.init.Do(func() {
		server.options.SetDefaults()

		server.context, server.cancel = context.WithCancelCause(context.Background())

		server.vapor.NewID = func() string {
			uuid, err := uuid.NewRandom()
			if err != nil {
				return ""
			}
			return uuid.String()
		}
		server.vapor.Initialize = func(s *Session) {
			s.Init(server.handler, server.context, server.options.Session)
		}
		server.vapor.Finalize = func(fr vapor.FinalizeReason, s *Session) {
			if fr == vapor.FinalizeReasonExpired {
				s.CloseWith(proto.ErrCancelTimeout)
			}
		}

		server.mux.HandleFunc("POST /new", server.serveNew)
		server.mux.HandleFunc("GET /status/{id}", server.serveStatus)
		server.mux.HandleFunc("POST /input/{id}", server.serveInput)
		server.mux.HandleFunc("POST /closeInput/{id}", server.serveCloseInput)
		server.mux.HandleFunc("POST /cancel/{id}", server.serveCancel)
	})
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.doInit()

	// client attempted websocket upgrade, which we do not support
	if websocket.IsWebSocketUpgrade(r) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// server is closed, don't use it
	if server.context.Err() != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	// serve the server
	server.mux.ServeHTTP(w, r)
}

func (server *Server) serveNew(w http.ResponseWriter, r *http.Request) {
	// decode the call
	var call proto.CallMessage
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		http.Error(w, "failed to decode call message", http.StatusBadRequest)
		return
	}

	// create the new element
	id, session, err := server.vapor.GetNew(server.options.Timeout)
	if err != nil {
		http.Error(w, "failed to create new process", http.StatusInternalServerError)
		return
	}

	// start the session
	session.Start(r, call)

	// return the new id to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(id)
}

func (server *Server) serveStatus(w http.ResponseWriter, r *http.Request) {
	// extract the id from the path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "did not provide id", http.StatusBadRequest)
		return
	}

	// get the session
	session, err := server.vapor.Get(id)
	if err != nil {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	// marshal the status into the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session.Status())
}

func (server *Server) serveInput(w http.ResponseWriter, r *http.Request) {
	// extract the id from the path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "did not provide id", http.StatusBadRequest)
		return
	}

	// get the session
	session, err := server.vapor.Get(id)
	if err != nil {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	// copy the body over
	if _, err := io.Copy(session, r.Body); err != nil {
		http.Error(w, "error copying data to process", http.StatusInternalServerError)
		return
	}

	// done
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "input sent")
}

func (server *Server) serveCloseInput(w http.ResponseWriter, r *http.Request) {
	// extract the id from the path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "did not provide id", http.StatusBadRequest)
		return
	}

	// get the session
	session, err := server.vapor.Get(id)
	if err != nil {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	// Close it's input
	if err := session.CloseInput(); err != nil {
		http.Error(w, "error closing input", http.StatusInternalServerError)
		return
	}

	// done
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "input closed")
}

func (server *Server) serveCancel(w http.ResponseWriter, r *http.Request) {
	// extract the id from the path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "did not provide id", http.StatusBadRequest)
		return
	}

	// get the session
	session, err := server.vapor.Get(id)
	if err != nil {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	// close the session
	session.CloseWith(proto.ErrCancelClientRequest)

	// done
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "input closed")
}

var errServerClose = errors.New("server closing")

func (server *Server) Close() {
	server.doInit()

	// cancel all the ongoing contexts and wait for them to finish
	server.vapor.EvictAfter(func(session *Session) { session.CloseWith(errServerClose) })
	server.vapor.Close()
}
