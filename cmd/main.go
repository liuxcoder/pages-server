package cmd

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server"
)

// AllowedCorsDomains lists the domains for which Cross-Origin Resource Sharing is allowed.
var AllowedCorsDomains = [][]byte{
	RawDomain,
	[]byte("fonts.codeberg.org"),
	[]byte("design.codeberg.org"),
}

// BlacklistedPaths specifies forbidden path prefixes for all Codeberg Pages.
var BlacklistedPaths = [][]byte{
	[]byte("/.well-known/acme-challenge/"),
}

// Serve sets up and starts the web server.
func Serve(ctx *cli.Context) error {
	mainDomainSuffix := []byte(ctx.String("main-domain-suffix"))
	// Make sure MainDomain has a trailing dot, and GiteaRoot has no trailing slash
	if !bytes.HasPrefix(mainDomainSuffix, []byte{'.'}) {
		mainDomainSuffix = append([]byte{'.'}, mainDomainSuffix...)
	}

	GiteaRoot = bytes.TrimSuffix(GiteaRoot, []byte{'/'})

	// Use HOST and PORT environment variables to determine listening address
	address := fmt.Sprintf("%s:%s", server.EnvOr("HOST", "[::]"), server.EnvOr("PORT", "443"))
	log.Printf("Listening on https://%s", address)

	// Create handler based on settings
	handler := server.Handler(mainDomainSuffix, RawDomain, GiteaRoot, RawInfoPage, GiteaApiToken, BlacklistedPaths, AllowedCorsDomains)

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

	// Setup listener and TLS
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Couldn't create listener: %s", err)
	}
	listener = tls.NewListener(listener, server.TlsConfig(mainDomainSuffix, string(GiteaRoot), GiteaApiToken))

	server.SetupCertificates(mainDomainSuffix)
	if os.Getenv("ENABLE_HTTP_SERVER") == "true" {
		go (func() {
			challengePath := []byte("/.well-known/acme-challenge/")
			err := fasthttp.ListenAndServe("[::]:80", func(ctx *fasthttp.RequestCtx) {
				if bytes.HasPrefix(ctx.Path(), challengePath) {
					challenge, ok := server.ChallengeCache.Get(string(server.TrimHostPort(ctx.Host())) + "/" + string(bytes.TrimPrefix(ctx.Path(), challengePath)))
					if !ok || challenge == nil {
						ctx.SetStatusCode(http.StatusNotFound)
						ctx.SetBodyString("no challenge for this token")
					}
					ctx.SetBodyString(challenge.(string))
				} else {
					ctx.Redirect("https://"+string(ctx.Host())+string(ctx.RequestURI()), http.StatusMovedPermanently)
				}
			})
			if err != nil {
				log.Fatalf("Couldn't start HTTP fastServer: %s", err)
			}
		})()
	}

	// Start the web fastServer
	err = fastServer.Serve(listener)
	if err != nil {
		log.Fatalf("Couldn't start fastServer: %s", err)
	}

	return nil
}
