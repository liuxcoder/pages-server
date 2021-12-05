package server

import (
	"time"

	"github.com/valyala/fasthttp"
)

func SetupServer(handler fasthttp.RequestHandler) (*fasthttp.Server, error) {
	// Enable compression by wrapping the handler with the compression function provided by FastHTTP
	compressedHandler := fasthttp.CompressHandlerBrotliLevel(handler, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)

	fastServer := &fasthttp.Server{
		Handler:                      compressedHandler,
		DisablePreParseMultipartForm: true,
		MaxRequestBodySize:           0,
		NoDefaultServerHeader:        true,
		NoDefaultDate:                true,
		ReadTimeout:                  30 * time.Second, // needs to be this high for ACME certificates with ZeroSSL & HTTP-01 challenge
		Concurrency:                  1024 * 32,        // TODO: adjust bottlenecks for best performance with Gitea!
		MaxConnsPerIP:                100,
	}

	return fastServer, nil
}
