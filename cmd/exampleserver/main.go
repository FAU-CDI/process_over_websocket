//spellchecker:words main
package main

//spellchecker:words context http signal github process over websocket proto
import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/FAU-CDI/process_over_websocket"
	"github.com/FAU-CDI/process_over_websocket/proto"
)

var bind_addr string = "0.0.0.0:3000"

func main() {
	// listen to cancel events on the context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	// create a new process_over_websocket Server
	var server process_over_websocket.Server
	server.Options.RESTOptions.OpenAPIServerDescription = "Process Over Websocket Testing Server"
	server.Handler = proto.HandlerFunc(func(r *http.Request, name string, args ...string) (proto.Process, error) {
		log.Printf("got request for %s %v", name, args)

		// must be the echo handler
		if name != "echo" {
			return nil, proto.ErrHandlerUnknownProcess
		}

		// return the error handler
		return proto.ProcessFunc(func(ctx context.Context, input io.Reader, output io.Writer, args ...string) (any, error) {
			// log that we are doing something
			log.Println("starting new process")
			defer log.Println("process exited")

			// copy over the content
			if _, err := io.Copy(output, input); err != nil {
				return nil, fmt.Errorf("failed to copy: %w", err)
			}

			return args, context.Cause(ctx)
		}), nil
	})

	// start listening
	listen, err := net.Listen("tcp", bind_addr) //#nosec G102 -- bind_addr is a parameter
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}
	log.Println("listening on ", bind_addr)

	// create a http server and start listening
	var http_server http.Server
	http_server.Handler = &server

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()

		log.Println("shutting down process_over_websocket server")
		server.Shutdown() // shutdown the websocket server
		log.Println("shutting down http server")
		//nolint:contextcheck
		_ = http_server.Shutdown(context.Background()) // shutdown the http server
	}()

	_ = http_server.Serve(listen)
	<-done
}
