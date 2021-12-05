package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
	"codeberg.org/codeberg/pages/server/database"
)

// AllowedCorsDomains lists the domains for which Cross-Origin Resource Sharing is allowed.
// TODO: make it a flag
var AllowedCorsDomains = [][]byte{
	[]byte("fonts.codeberg.org"),
	[]byte("design.codeberg.org"),
}

// BlacklistedPaths specifies forbidden path prefixes for all Codeberg Pages.
// TODO: Make it a flag too
var BlacklistedPaths = [][]byte{
	[]byte("/.well-known/acme-challenge/"),
}

// Serve sets up and starts the web server.
func Serve(ctx *cli.Context) error {
	giteaRoot := strings.TrimSuffix(ctx.String("gitea-root"), "/")
	giteaAPIToken := ctx.String("gitea-api-token")
	rawDomain := ctx.String("raw-domain")
	mainDomainSuffix := []byte(ctx.String("pages-domain"))
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

	keyCache := cache.NewKeyValueCache()
	challengeCache := cache.NewKeyValueCache()
	// canonicalDomainCache stores canonical domains
	var canonicalDomainCache = cache.NewKeyValueCache()
	// dnsLookupCache stores DNS lookups for custom domains
	var dnsLookupCache = cache.NewKeyValueCache()
	// branchTimestampCache stores branch timestamps for faster cache checking
	var branchTimestampCache = cache.NewKeyValueCache()
	// fileResponseCache stores responses from the Gitea server
	// TODO: make this an MRU cache with a size limit
	var fileResponseCache = cache.NewKeyValueCache()

	// Create handler based on settings
	handler := server.Handler(mainDomainSuffix, []byte(rawDomain),
		giteaRoot, rawInfoPage, giteaAPIToken,
		BlacklistedPaths, allowedCorsDomains,
		dnsLookupCache, canonicalDomainCache, branchTimestampCache, fileResponseCache)

	fastServer := server.SetupServer(handler)
	httpServer := server.SetupHTTPACMEChallengeServer(challengeCache)

	// Setup listener and TLS
	log.Info().Msgf("Listening on https://%s", listeningAddress)
	listener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		return fmt.Errorf("couldn't create listener: %s", err)
	}

	// TODO: make "key-database.pogreb" set via flag
	certDB, err := database.New("key-database.pogreb")
	if err != nil {
		return fmt.Errorf("could not create database: %v", err)
	}
	defer certDB.Close() //nolint:errcheck    // database has no close ... sync behave like it

	listener = tls.NewListener(listener, certificates.TLSConfig(mainDomainSuffix,
		giteaRoot, giteaAPIToken, dnsProvider,
		acmeUseRateLimits,
		keyCache, challengeCache, dnsLookupCache, canonicalDomainCache,
		certDB))

	acmeConfig, err := certificates.SetupAcmeConfig(acmeAPI, acmeMail, acmeEabHmac, acmeEabKID, acmeAcceptTerms)
	if err != nil {
		return err
	}

	certificates.SetupCertificates(mainDomainSuffix, dnsProvider, acmeConfig, acmeUseRateLimits, enableHTTPServer, challengeCache, certDB)

	interval := 12 * time.Hour
	certMaintainCtx, cancelCertMaintain := context.WithCancel(context.Background())
	defer cancelCertMaintain()
	go certificates.MaintainCertDB(certMaintainCtx, interval, mainDomainSuffix, dnsProvider, acmeUseRateLimits, certDB)

	if enableHTTPServer {
		go func() {
			err := httpServer.ListenAndServe("[::]:80")
			if err != nil {
				log.Panic().Err(err).Msg("Couldn't start HTTP fastServer")
			}
		}()
	}

	// Start the web fastServer
	err = fastServer.Serve(listener)
	if err != nil {
		log.Panic().Err(err).Msg("Couldn't start fastServer")
	}

	return nil
}
