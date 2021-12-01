// Package main is the new Codeberg Pages server, a solution for serving static pages from Gitea repositories.
//
// Mapping custom domains is not static anymore, but can be done with DNS:
//
// 1) add a "domains.txt" text file to your repository, containing the allowed domains, separated by new lines. The
// first line will be the canonical domain/URL; all other occurrences will be redirected to it.
//
// 2) add a CNAME entry to your domain, pointing to "[[{branch}.]{repo}.]{owner}.codeberg.page" (repo defaults to
// "pages", "branch" defaults to the default branch if "repo" is "pages", or to "pages" if "repo" is something else):
//      www.example.org. IN CNAME main.pages.example.codeberg.page.
//
// 3) if a CNAME is set for "www.example.org", you can redirect there from the naked domain by adding an ALIAS record
// for "example.org" (if your provider allows ALIAS or similar records):
//      example.org IN ALIAS codeberg.page.
//
// Certificates are generated, updated and cleaned up automatically via Let's Encrypt through a TLS challenge.
package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "embed"

	"github.com/valyala/fasthttp"
)

// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
var MainDomainSuffix = []byte("." + envOr("PAGES_DOMAIN", "codeberg.page"))

// GiteaRoot specifies the root URL of the Gitea instance, without a trailing slash.
var GiteaRoot = []byte(envOr("GITEA_ROOT", "https://codeberg.org"))

var GiteaApiToken = envOr("GITEA_API_TOKEN", "")

//go:embed 404.html
var NotFoundPage []byte

// RawDomain specifies the domain from which raw repository content shall be served in the following format:
// https://{RawDomain}/{owner}/{repo}[/{branch|tag|commit}/{version}]/{filepath...}
// (set to []byte(nil) to disable raw content hosting)
var RawDomain = []byte(envOr("RAW_DOMAIN", "raw.codeberg.org"))

// RawInfoPage will be shown (with a redirect) when trying to access RawDomain directly (or without owner/repo/path).
var RawInfoPage = envOr("REDIRECT_RAW_INFO", "https://docs.codeberg.org/pages/raw-content/")

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

// IndexPages lists pages that may be considered as index pages for directories.
var IndexPages = []string{
	"index.html",
}

// main sets up and starts the web server.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--remove-certificate" {
		if len(os.Args) < 2 {
			println("--remove-certificate requires at least one domain as an argument")
			os.Exit(1)
		}
		if keyDatabaseErr != nil {
			panic(keyDatabaseErr)
		}
		for _, domain := range os.Args[2:] {
			if err := keyDatabase.Delete([]byte(domain)); err != nil {
				panic(err)
			}
		}
		if err := keyDatabase.Sync(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	// Make sure MainDomain has a trailing dot, and GiteaRoot has no trailing slash
	if !bytes.HasPrefix(MainDomainSuffix, []byte{'.'}) {
		MainDomainSuffix = append([]byte{'.'}, MainDomainSuffix...)
	}
	GiteaRoot = bytes.TrimSuffix(GiteaRoot, []byte{'/'})

	// Use HOST and PORT environment variables to determine listening address
	address := fmt.Sprintf("%s:%s", envOr("HOST", "[::]"), envOr("PORT", "443"))
	log.Printf("Listening on https://%s", address)

	// Enable compression by wrapping the handler() method with the compression function provided by FastHTTP
	compressedHandler := fasthttp.CompressHandlerBrotliLevel(handler, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)

	server := &fasthttp.Server{
		Handler:                      compressedHandler,
		DisablePreParseMultipartForm: false,
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
	listener = tls.NewListener(listener, tlsConfig)

	setupCertificates()
	if os.Getenv("ENABLE_HTTP_SERVER") == "true" {
		go (func() {
			challengePath := []byte("/.well-known/acme-challenge/")
			err := fasthttp.ListenAndServe("[::]:80", func(ctx *fasthttp.RequestCtx) {
				if bytes.HasPrefix(ctx.Path(), challengePath) {
					challenge, ok := challengeCache.Get(string(TrimHostPort(ctx.Host())) + "/" + string(bytes.TrimPrefix(ctx.Path(), challengePath)))
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
				log.Fatalf("Couldn't start HTTP server: %s", err)
			}
		})()
	}

	// Start the web server
	err = server.Serve(listener)
	if err != nil {
		log.Fatalf("Couldn't start server: %s", err)
	}
}

// envOr reads an environment variable and returns a default value if it's empty.
func envOr(env string, or string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return or
}
