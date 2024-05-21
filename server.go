//spellchecker:words process over websocket
package process_over_websocket

//spellchecker:words http sync github process over websocket internal rest impl proto pkglib websocketx
import (
	"net/http"
	"sync"

	"github.com/FAU-CDI/process_over_websocket/internal/rest_impl"
	"github.com/FAU-CDI/process_over_websocket/internal/ws_impl"
	"github.com/FAU-CDI/process_over_websocket/proto"
	"github.com/tkw1536/pkglib/websocketx"
)

// Server implements process_over_websocket protocol.
type Server struct {
	Handler proto.Handler
	Options Options

	init      sync.Once
	handler   http.Handler
	websocket *ws_impl.Server
	rest      *rest_impl.Server
}

type Options struct {
	// DisableWebsocket can be set to entirely disable websocket handling.
	DisableWebsocket bool
	WebsocketOptions websocketx.Options

	// DisableREST can be set to entirely disable REST access.
	DisableREST bool
	RESTOptions rest_impl.Options
}

// ServeHTTP serves a request
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.doInit()
	server.handler.ServeHTTP(w, r)
}

func (server *Server) doInit() {
	server.init.Do(func() {
		server.handler = func() http.Handler {
			// setup the rest server if requested
			if !server.Options.DisableREST {
				server.rest = rest_impl.NewServer(server.Handler, server.Options.RESTOptions)
			}

			// setup the websocket handler if requested
			if !server.Options.DisableWebsocket {
				server.websocket = ws_impl.NewServer(server.Handler, server.rest, server.Options.WebsocketOptions)
			}

			// nothing is enabled =>
			if server.Options.DisableREST && server.Options.DisableWebsocket {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Not found", http.StatusNotFound)
				})
			}

			// return the right handler
			if server.Options.DisableWebsocket {
				return server.rest
			}
			return server.websocket
		}()
	})
}

func (server *Server) Close() {
	server.doInit()

	var wg sync.WaitGroup

	// close the websocket server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if server.websocket == nil {
			return
		}
		server.websocket.Close()
	}()

	// close the rest server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if server.rest == nil {
			return
		}
		server.rest.Close()
	}()

	// and be done with it
	wg.Wait()
}

func (server *Server) Shutdown() {
	server.doInit()

	var wg sync.WaitGroup

	// shutdown the websocket server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if server.websocket == nil {
			return
		}
		server.websocket.Shutdown()
	}()

	// shutdown the rest server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if server.rest == nil {
			return
		}
		server.rest.Close()
	}()

	// and be done with it
	wg.Wait()
}
