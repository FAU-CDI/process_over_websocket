package rest_impl

import (
	"net/http"

	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/gorilla/websocket"
)

func NewServer(handler proto.Handler, options Options) *Server {
	return &Server{
		handler: handler,
		options: Options{},
	}
}

type Options struct{}

type Server struct {
	options Options
	handler proto.Handler
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// client attempted websocket upgrade, which we do not support
	if websocket.IsWebSocketUpgrade(r) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// rest protocol isn't implemented yet
	http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
}

func (server *Server) Close() {
	// do nothing for now
}

func (server *Server) Shutdown() {
	// do nothing for now
}
