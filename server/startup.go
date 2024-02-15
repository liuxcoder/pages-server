package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	cmd "codeberg.org/codeberg/pages/cli"
	"codeberg.org/codeberg/pages/config"
	"codeberg.org/codeberg/pages/server/acme"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/handler"
)

// Serve sets up and starts the web server.
func Serve(ctx *cli.Context) error {
	// initialize logger with Trace, overridden later with actual level
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(zerolog.TraceLevel)

	cfg, err := config.ReadConfig(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not read config")
	}

	config.MergeConfig(ctx, cfg)

	// Initialize the logger.
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		return err
	}
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(logLevel)

	foo, _ := json.Marshal(cfg)
	log.Trace().RawJSON("config", foo).Msg("starting server with config")

	listeningSSLAddress := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	listeningHTTPAddress := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HttpPort)

	if cfg.Server.RawDomain != "" {
		cfg.Server.AllowedCorsDomains = append(cfg.Server.AllowedCorsDomains, cfg.Server.RawDomain)
	}

	// Make sure MainDomain has a leading dot
	if !strings.HasPrefix(cfg.Server.MainDomain, ".") {
		// TODO make this better
		cfg.Server.MainDomain = "." + cfg.Server.MainDomain
	}

	if len(cfg.Server.PagesBranches) == 0 {
		return fmt.Errorf("no default branches set (PAGES_BRANCHES)")
	}

	// Init ssl cert database
	certDB, closeFn, err := cmd.OpenCertDB(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	keyCache := cache.NewInMemoryCache()
	challengeCache := cache.NewInMemoryCache()
	// canonicalDomainCache stores canonical domains
	canonicalDomainCache := cache.NewInMemoryCache()
	// dnsLookupCache stores DNS lookups for custom domains
	dnsLookupCache := cache.NewInMemoryCache()
	// redirectsCache stores redirects in _redirects files
	redirectsCache := cache.NewInMemoryCache()
	// clientResponseCache stores responses from the Gitea server
	clientResponseCache := cache.NewInMemoryCache()

	giteaClient, err := gitea.NewClient(cfg.Gitea, clientResponseCache)
	if err != nil {
		return fmt.Errorf("could not create new gitea client: %v", err)
	}

	acmeClient, err := acme.CreateAcmeClient(cfg.ACME, cfg.Server.HttpServerEnabled, challengeCache)
	if err != nil {
		return err
	}

	if err := certificates.SetupMainDomainCertificates(cfg.Server.MainDomain, acmeClient, certDB); err != nil {
		return err
	}

	// Create listener for SSL connections
	log.Info().Msgf("Create TCP listener for SSL on %s", listeningSSLAddress)
	listener, err := net.Listen("tcp", listeningSSLAddress)
	if err != nil {
		return fmt.Errorf("couldn't create listener: %v", err)
	}

	// Setup listener for SSL connections
	listener = tls.NewListener(listener, certificates.TLSConfig(
		cfg.Server.MainDomain,
		giteaClient,
		acmeClient,
		cfg.Server.PagesBranches[0],
		keyCache, challengeCache, dnsLookupCache, canonicalDomainCache,
		certDB,
	))

	interval := 12 * time.Hour
	certMaintainCtx, cancelCertMaintain := context.WithCancel(context.Background())
	defer cancelCertMaintain()
	go certificates.MaintainCertDB(certMaintainCtx, interval, acmeClient, cfg.Server.MainDomain, certDB)

	if cfg.Server.HttpServerEnabled {
		// Create handler for http->https redirect and http acme challenges
		httpHandler := certificates.SetupHTTPACMEChallengeServer(challengeCache, uint(cfg.Server.Port))

		// Create listener for http and start listening
		go func() {
			log.Info().Msgf("Start HTTP server listening on %s", listeningHTTPAddress)
			err := http.ListenAndServe(listeningHTTPAddress, httpHandler)
			if err != nil {
				log.Error().Err(err).Msg("Couldn't start HTTP server")
			}
		}()
	}

	// Create ssl handler based on settings
	sslHandler := handler.Handler(cfg.Server, giteaClient, dnsLookupCache, canonicalDomainCache, redirectsCache)

	// Start the ssl listener
	log.Info().Msgf("Start SSL server using TCP listener on %s", listener.Addr())

	return http.Serve(listener, sslHandler)
}
