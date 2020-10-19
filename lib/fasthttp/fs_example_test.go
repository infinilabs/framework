package fasthttp_test

import (
	fasthttp2 "infini.sh/framework/lib/fasthttp"
	"log"
)

func ExampleFS() {
	fs := &fasthttp2.FS{
		// Path to directory to serve.
		Root: "/var/www/static-site",

		// Generate index pages if client requests directory contents.
		GenerateIndexPages: true,

		// Enable transparent compression to save network traffic.
		Compress: true,
	}

	// Create request handler for serving static files.
	h := fs.NewRequestHandler()

	// Start the server.
	if err := fasthttp2.ListenAndServe(":8080", h); err != nil {
		log.Fatalf("error in ListenAndServe: %s", err)
	}
}
