package fasthttp_test

import (
	fasthttp2 "infini.sh/framework/lib/fasthttp"
	"log"
)

func ExampleHostClient() {
	// Perpare a client, which fetches webpages via HTTP proxy listening
	// on the localhost:8080.
	c := &fasthttp2.HostClient{
		Addr: "localhost:8080",
	}

	// Fetch google page via local proxy.
	statusCode, body, err := c.Get(nil, "http://google.com/foo/bar")
	if err != nil {
		log.Fatalf("Error when loading google page through local proxy: %s", err)
	}
	if statusCode != fasthttp2.StatusOK {
		log.Fatalf("Unexpected status code: %d. Expecting %d", statusCode, fasthttp2.StatusOK)
	}
	useResponseBody(body)

	// Fetch foobar page via local proxy. Reuse body buffer.
	statusCode, body, err = c.Get(body, "http://foobar.com/google/com")
	if err != nil {
		log.Fatalf("Error when loading foobar page through local proxy: %s", err)
	}
	if statusCode != fasthttp2.StatusOK {
		log.Fatalf("Unexpected status code: %d. Expecting %d", statusCode, fasthttp2.StatusOK)
	}
	useResponseBody(body)
}

func useResponseBody(body []byte) {
	// Do something with body :)
}
