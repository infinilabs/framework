package reuseport_test

import (
	"fmt"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
	reuseport2 "infini.sh/framework/lib/fasthttp/reuseport"
	"log"
)

func ExampleListen() {
	ln, err := reuseport2.Listen("tcp4", "localhost:12345")
	if err != nil {
		log.Fatalf("error in reuseport listener: %s", err)
	}

	if err = fasthttp2.Serve(ln, requestHandler); err != nil {
		log.Fatalf("error in fasthttp Server: %s", err)
	}
}

func requestHandler(ctx *fasthttp2.RequestCtx) {
	fmt.Fprintf(ctx, "Hello, world!")
}
