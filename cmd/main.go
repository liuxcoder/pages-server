package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
	"codeberg.org/codeberg/pages/server/database"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/handler"
)

// AllowedCorsDomains lists the domains for which Cross-Origin Resource Sharing is allowed.
// TODO: make it a flag
var AllowedCorsDomains = []string{
	"fonts.codeberg.org",
	"design.codeberg.org",
}

// BlacklistedPaths specifies forbidden path prefixes for all Codeberg Pages.
// TODO: Make it a flag too
var BlacklistedPaths = []string{
	"/.well-known/acme-challenge/",
}

// Serve sets up and starts the web server.
func Serve(ctx *cli.Context) error {
	// Initalize the logger.
	logLevel, err := zerolog.ParseLevel(ctx.String("log-level"))
	if err != nil {
		return err
	}
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(logLevel)

	giteaRoot := strings.TrimSuffix(ctx.String("gitea-root"), "/")
	giteaAPIToken := ctx.String("gitea-api-token")
	rawDomain := ctx.String("raw-domain")
	mainDomainSuffix := ctx.String("pages-domain")
	rawInfoPage := ctx.String("raw-info-page")
	listeningAddress := fmt.Sprintf("%s:%s", ctx.String("host"), ctx.String("port"))
	enableHTTPServer := ctx.Bool("enable-http-server")

	acmeAPI := ctx.String("acme-api-endpoint")
	acmeMail := ctx.String("acme-email")
	acmeUseRateLimits := ctx.Bool("acme-use-rate-limits")
	acmeAcceptTerms := ctx.Bool("acme-accept-terms")
	acmeEabKID := ctx.String("acme-eab-kid")
	acmeEabHmac := ctx.String("acme-eab-hmac")
	dnsProvider := ctx.String("dns-provider")
	if (!acmeAcceptTerms || dnsProvider == "") && acmeAPI != "https://acme.mock.directory" {
		return errors.New("you must set $ACME_ACCEPT_TERMS and $DNS_PROVIDER, unless $ACME_API is set to https://acme.mock.directory")
	}

	allowedCorsDomains := AllowedCorsDomains
	if len(rawDomain) != 0 {
		allowedCorsDomains = append(allowedCorsDomains, rawDomain)
	}

	// Make sure MainDomain has a trailing dot, and GiteaRoot has no trailing slash
	if !strings.HasPrefix(mainDomainSuffix, ".") {
		mainDomainSuffix = "." + mainDomainSuffix
	}

	keyCache := cache.NewKeyValueCache()
	challengeCache := cache.NewKeyValueCache()
	// canonicalDomainCache stores canonical domains
	canonicalDomainCache := cache.NewKeyValueCache()
	// dnsLookupCache stores DNS lookups for custom domains
	dnsLookupCache := cache.NewKeyValueCache()
	// clientResponseCache stores responses from the Gitea server
	clientResponseCache := cache.NewKeyValueCache()

	giteaClient, err := gitea.NewClient(giteaRoot, giteaAPIToken, clientResponseCache, ctx.Bool("enable-symlink-support"), ctx.Bool("enable-lfs-support"))
	if err != nil {
		return fmt.Errorf("could not create new gitea client: %v", err)
	}

	// Create handler based on settings
	httpsHandler := handler.Handler(mainDomainSuffix, rawDomain,
		giteaClient,
		rawInfoPage,
		BlacklistedPaths, allowedCorsDomains,
		dnsLookupCache, canonicalDomainCache)

	httpHandler := server.SetupHTTPACMEChallengeServer(challengeCache)

	// Setup listener and TLS
	log.Info().Msgf("Listening on https://%s", listeningAddress)
	listener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		return fmt.Errorf("couldn't create listener: %v", err)
	}

	// TODO: make "key-database.pogreb" set via flag
	certDB, err := database.New("key-database.pogreb")
	if err != nil {
		return fmt.Errorf("could not create database: %v", err)
	}
	defer certDB.Close() //nolint:errcheck    // database has no close ... sync behave like it

	listener = tls.NewListener(listener, certificates.TLSConfig(mainDomainSuffix,
		giteaClient,
		dnsProvider,
		acmeUseRateLimits,
		keyCache, challengeCache, dnsLookupCache, canonicalDomainCache,
		certDB))

	acmeConfig, err := certificates.SetupAcmeConfig(acmeAPI, acmeMail, acmeEabHmac, acmeEabKID, acmeAcceptTerms)
	if err != nil {
		return err
	}

	if err := certificates.SetupCertificates(mainDomainSuffix, dnsProvider, acmeConfig, acmeUseRateLimits, enableHTTPServer, challengeCache, certDB); err != nil {
		return err
	}

	interval := 12 * time.Hour
	certMaintainCtx, cancelCertMaintain := context.WithCancel(context.Background())
	defer cancelCertMaintain()
	go certificates.MaintainCertDB(certMaintainCtx, interval, mainDomainSuffix, dnsProvider, acmeUseRateLimits, certDB)

	if enableHTTPServer {
		go func() {
			log.Info().Msg("Start HTTP server listening on :80")
			err := http.ListenAndServe("[::]:80", httpHandler)
			if err != nil {
				log.Panic().Err(err).Msg("Couldn't start HTTP fastServer")
			}
		}()
	}

	// Start the web fastServer
	log.Info().Msgf("Start listening on %s", listener.Addr())
	if err := http.Serve(listener, httpsHandler); err != nil {
		log.Panic().Err(err).Msg("Couldn't start fastServer")
	}

	return nil
}
