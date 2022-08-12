package server

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/utils"
)

type fasthttpLogger struct{}

func (fasthttpLogger) Printf(format string, args ...interface{}) {
	log.Printf("FastHTTP: %s", fmt.Sprintf(format, args...))
}

func SetupServer(handler fasthttp.RequestHandler) *fasthttp.Server {
	// Enable compression by wrapping the handler with the compression function provided by FastHTTP
	compressedHandler := fasthttp.CompressHandlerBrotliLevel(handler, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)

	return &fasthttp.Server{
		Handler:                      compressedHandler,
		DisablePreParseMultipartForm: true,
		NoDefaultServerHeader:        true,
		NoDefaultDate:                true,
		ReadTimeout:                  30 * time.Second, // needs to be this high for ACME certificates with ZeroSSL & HTTP-01 challenge
		Logger:                       fasthttpLogger{},
	}
}

func SetupHTTPACMEChallengeServer(challengeCache cache.SetGetKey) *fasthttp.Server {
	challengePath := []byte("/.well-known/acme-challenge/")

	return &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			if bytes.HasPrefix(ctx.Path(), challengePath) {
				challenge, ok := challengeCache.Get(string(utils.TrimHostPort(ctx.Host())) + "/" + string(bytes.TrimPrefix(ctx.Path(), challengePath)))
				if !ok || challenge == nil {
					ctx.SetStatusCode(http.StatusNotFound)
					ctx.SetBodyString("no challenge for this token")
				}
				ctx.SetBodyString(challenge.(string))
			} else {
				ctx.Redirect("https://"+string(ctx.Host())+string(ctx.RequestURI()), http.StatusMovedPermanently)
			}
		},
	}
}
