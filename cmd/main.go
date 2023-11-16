package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
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
	// Initialize the logger.
	logLevel, err := zerolog.ParseLevel(ctx.String("log-level"))
	if err != nil {
		return err
	}
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(logLevel)

	giteaRoot := ctx.String("gitea-root")
	giteaAPIToken := ctx.String("gitea-api-token")
	rawDomain := ctx.String("raw-domain")
	defaultBranches := ctx.StringSlice("pages-branch")
	mainDomainSuffix := ctx.String("pages-domain")
	listeningHost := ctx.String("host")
	listeningSSLPort := ctx.Uint("port")
	listeningSSLAddress := fmt.Sprintf("%s:%d", listeningHost, listeningSSLPort)
	listeningHTTPAddress := fmt.Sprintf("%s:%d", listeningHost, ctx.Uint("http-port"))
	enableHTTPServer := ctx.Bool("enable-http-server")

	allowedCorsDomains := AllowedCorsDomains
	if rawDomain != "" {
		allowedCorsDomains = append(allowedCorsDomains, rawDomain)
	}

	// Make sure MainDomain has a trailing dot
	if !strings.HasPrefix(mainDomainSuffix, ".") {
		mainDomainSuffix = "." + mainDomainSuffix
	}

	if len(defaultBranches) == 0 {
		return fmt.Errorf("no default branches set (PAGES_BRANCHES)")
	}

	// Init ssl cert database
	certDB, closeFn, err := openCertDB(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	keyCache := cache.NewKeyValueCache()
	challengeCache := cache.NewKeyValueCache()
	// canonicalDomainCache stores canonical domains
	canonicalDomainCache := cache.NewKeyValueCache()
	// dnsLookupCache stores DNS lookups for custom domains
	dnsLookupCache := cache.NewKeyValueCache()
	// redirectsCache stores redirects in _redirects files
	redirectsCache := cache.NewKeyValueCache()
	// clientResponseCache stores responses from the Gitea server
	clientResponseCache := cache.NewKeyValueCache()

	giteaClient, err := gitea.NewClient(giteaRoot, giteaAPIToken, clientResponseCache, ctx.Bool("enable-symlink-support"), ctx.Bool("enable-lfs-support"))
	if err != nil {
		return fmt.Errorf("could not create new gitea client: %v", err)
	}

	acmeClient, err := createAcmeClient(ctx, enableHTTPServer, challengeCache)
	if err != nil {
		return err
	}

	if err := certificates.SetupMainDomainCertificates(mainDomainSuffix, acmeClient, certDB); err != nil {
		return err
	}

	// Create listener for SSL connections
	log.Info().Msgf("Create TCP listener for SSL on %s", listeningSSLAddress)
	listener, err := net.Listen("tcp", listeningSSLAddress)
	if err != nil {
		return fmt.Errorf("couldn't create listener: %v", err)
	}

	// Setup listener for SSL connections
	listener = tls.NewListener(listener, certificates.TLSConfig(mainDomainSuffix,
		giteaClient,
		acmeClient,
		defaultBranches[0],
		keyCache, challengeCache, dnsLookupCache, canonicalDomainCache,
		certDB))

	interval := 12 * time.Hour
	certMaintainCtx, cancelCertMaintain := context.WithCancel(context.Background())
	defer cancelCertMaintain()
	go certificates.MaintainCertDB(certMaintainCtx, interval, acmeClient, mainDomainSuffix, certDB)

	if enableHTTPServer {
		// Create handler for http->https redirect and http acme challenges
		httpHandler := certificates.SetupHTTPACMEChallengeServer(challengeCache, listeningSSLPort)

		// Create listener for http and start listening
		go func() {
			log.Info().Msgf("Start HTTP server listening on %s", listeningHTTPAddress)
			err := http.ListenAndServe(listeningHTTPAddress, httpHandler)
			if err != nil {
				log.Panic().Err(err).Msg("Couldn't start HTTP fastServer")
			}
		}()
	}

	// Create ssl handler based on settings
	sslHandler := handler.Handler(mainDomainSuffix, rawDomain,
		giteaClient,
		BlacklistedPaths, allowedCorsDomains,
		defaultBranches,
		dnsLookupCache, canonicalDomainCache, redirectsCache)

	// Start the ssl listener
	log.Info().Msgf("Start SSL server using TCP listener on %s", listener.Addr())
	if err := http.Serve(listener, sslHandler); err != nil {
		log.Panic().Err(err).Msg("Couldn't start fastServer")
	}

	return nil
}
