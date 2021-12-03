package cmd

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/database"
	"codeberg.org/codeberg/pages/server/utils"
)

// AllowedCorsDomains lists the domains for which Cross-Origin Resource Sharing is allowed.
// TODO: make it a flag
var AllowedCorsDomains = [][]byte{
	[]byte("fonts.codeberg.org"),
	[]byte("design.codeberg.org"),
}

// BlacklistedPaths specifies forbidden path prefixes for all Codeberg Pages.
var BlacklistedPaths = [][]byte{
	[]byte("/.well-known/acme-challenge/"),
}

// Serve sets up and starts the web server.
func Serve(ctx *cli.Context) error {
	giteaRoot := strings.TrimSuffix(ctx.String("gitea-root"), "/")
	giteaAPIToken := ctx.String("gitea-api-token")
	rawDomain := ctx.String("raw-domain")
	mainDomainSuffix := []byte(ctx.String("main-domain-suffix"))
	rawInfoPage := ctx.String("raw-info-page")
	listeningAddress := fmt.Sprintf("%s:%s", ctx.String("host"), ctx.String("port"))
	enableHTTPServer := ctx.Bool("enable-http-server")

	acmeAPI := ctx.String("acme-api")
	acmeMail := ctx.String("acme-email")
	acmeUseRateLimits := ctx.Bool("acme-use-rate-limits")
	acmeAcceptTerms := ctx.Bool("acme-accept-terms")
	acmeEabKID := ctx.String("acme-eab-kid")
	acmeEabHmac := ctx.String("acme-eab-hmac")
	dnsProvider := ctx.String("dns-provider")
	if acmeAcceptTerms || (dnsProvider == "" && acmeAPI != "https://acme.mock.directory") {
		return errors.New("you must set $ACME_ACCEPT_TERMS and $DNS_PROVIDER, unless $ACME_API is set to https://acme.mock.directory")
	}

	allowedCorsDomains := AllowedCorsDomains
	if len(rawDomain) != 0 {
		allowedCorsDomains = append(allowedCorsDomains, []byte(rawDomain))
	}

	// Make sure MainDomain has a trailing dot, and GiteaRoot has no trailing slash
	if !bytes.HasPrefix(mainDomainSuffix, []byte{'.'}) {
		mainDomainSuffix = append([]byte{'.'}, mainDomainSuffix...)
	}

	// Create handler based on settings
	handler := server.Handler(mainDomainSuffix, []byte(rawDomain), giteaRoot, rawInfoPage, giteaAPIToken, BlacklistedPaths, allowedCorsDomains)

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
	log.Info().Msgf("Listening on https://%s", listeningAddress)
	listener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		return fmt.Errorf("couldn't create listener: %s", err)
	}

	// TODO: make "key-database.pogreb" set via flag
	keyDatabase, err := database.New("key-database.pogreb")
	if err != nil {
		return fmt.Errorf("could not create database: %v", err)
	}

	keyCache := cache.NewKeyValueCache()
	listener = tls.NewListener(listener, server.TLSConfig(mainDomainSuffix, giteaRoot, giteaAPIToken, dnsProvider, acmeUseRateLimits, keyCache, keyDatabase))

	server.SetupCertificates(mainDomainSuffix, acmeAPI, acmeMail, acmeEabHmac, acmeEabKID, dnsProvider, acmeUseRateLimits, acmeAcceptTerms, enableHTTPServer, keyDatabase)
	if enableHTTPServer {
		go (func() {
			challengePath := []byte("/.well-known/acme-challenge/")
			err := fasthttp.ListenAndServe("[::]:80", func(ctx *fasthttp.RequestCtx) {
				if bytes.HasPrefix(ctx.Path(), challengePath) {
					challenge, ok := server.ChallengeCache.Get(string(utils.TrimHostPort(ctx.Host())) + "/" + string(bytes.TrimPrefix(ctx.Path(), challengePath)))
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
				log.Fatal().Err(err).Msg("Couldn't start HTTP fastServer")
			}
		})()
	}

	// Start the web fastServer
	err = fastServer.Serve(listener)
	if err != nil {
		log.Fatal().Err(err).Msg("Couldn't start fastServer")
	}

	return nil
}
