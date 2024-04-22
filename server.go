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
	Options Options

	handler lazy.Lazy[http.Handler]
}

type Options struct {
	// DisableWebsocket can be set to entirely disable websocket handling.
	DisableWebsocket bool
	WebsocketOptions websocketx.Options

	// DisableREST can be set to entirely disable REST access.
	DisableREST bool
	RESTOptions RESTOptions
}

// RESTOptions are options for the rest impl
type RESTOptions struct{}

// ServeHTTP serves a request
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.handler.Get(server.newHandler).ServeHTTP(w, r)
}

func (server *Server) newHandler() http.Handler {
	// if neither rest, nor websocket are enabled, server nothing
	if server.Options.DisableREST && server.Options.DisableWebsocket {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not found", http.StatusNotFound)
		})
	}

	// generate a rest handler, or leave it at nil
	var rest http.Handler
	if !server.Options.DisableREST {
		rest = server.newRestHandler()
	}

	// if websocket was disabled, return only rest
	if server.Options.DisableWebsocket {
		return rest
	}

	// requested a websocket handler
	return server.newWSHandler(rest)
}

func (server *Server) newWSHandler(fallback http.Handler) http.Handler {
	return ws_impl.NewServer(server.Handler, fallback, server.Options.WebsocketOptions)
}

func (server *Server) newRestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Implemented", http.StatusNotImplemented)
	})
}
