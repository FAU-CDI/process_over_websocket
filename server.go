package process_over_websocket

import (
	"net/http"

	"github.com/FAU-CDI/process_over_websocket/internal/ws_impl"
	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/lazy"
	"github.com/tkw1536/pkglib/websocketx"
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
	ws_server := &websocketx.Server{
		Options: websocketx.Options{
			Subprotocols: []string{proto.Subprotocol},
		},

		// implement the websocket and the handler itself
		Handler:  server.serveWS,
		Fallback: http.HandlerFunc(server.serveHTTP),
	}
	ws_server.RequireProtocols()
	return ws_server
}

func (server *Server) serveWS(conn *websocketx.Connection) {
	ws_impl.Serve(server.Handler, conn)
}

func (server *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
