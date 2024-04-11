package process_over_websocket

import (
	"context"
	"net/http"

	"github.com/FAU-CDI/process_over_websocket/internal/wsserver"
	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/httpx/websocket"
	"github.com/tkw1536/pkglib/lazy"
)

// Server implements process_over_websocket protocol.
type Server struct {
	Handler proto.Handler

	handler lazy.Lazy[http.Handler]
}

// ServeHTTP serves a request
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.handler.Get(server.newHandler).ServeHTTP(w, r)
}

func (server *Server) newHandler() http.Handler {
	return &websocket.Server{
		Context: context.Background(),

		// implement the websocket and the handler itself
		Handler:  server.serveWS,
		Fallback: http.HandlerFunc(server.serveHTTP),
	}
}

func (server *Server) serveWS(conn *websocket.Connection) {
	wsserver.Serve(server.Handler, conn)
}

func (server *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
