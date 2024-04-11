package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/FAU-CDI/process_over_websocket"
	"github.com/FAU-CDI/process_over_websocket/proto"
)

func main() {
	var server process_over_websocket.Server
	server.Handler = proto.HandlerFunc(func(r *http.Request, name string, args ...string) (proto.Process, error) {
		log.Printf("got request for %s %v", name, args)

		// must be the echo handler
		if name != "echo" {
			return nil, proto.ErrHandlerUnknownProcess
		}
		if len(args) != 0 {
			return nil, proto.ErrHandlerInvalidArgs
		}

		// return the error handler
		return proto.ProcessFunc(func(ctx context.Context, input io.Reader, output io.Writer, args ...string) error {
			// log that we are doing something
			log.Println("starting new process")
			defer log.Println("finished client")

			// and copy things over
			_, err := io.Copy(output, input)
			return err
		}), nil
	})

	bind := "0.0.0.0:3000"
	log.Println("listening on ", bind)
	log.Fatal(http.ListenAndServe(bind, &server))
}
