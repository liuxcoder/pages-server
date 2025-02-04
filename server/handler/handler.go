package handler

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/config"
	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
)

const (
	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
	defaultPagesRepo                = "pages"
)

// Handler handles a single HTTP request to the web server.
func Handler(
	cfg config.ServerConfig,
	giteaClient *gitea.Client,
	dnsLookupCache, canonicalDomainCache, redirectsCache cache.ICache,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Msg("\n----------------------------------------------------------")
		log := log.With().Strs("Handler", []string{req.Host, req.RequestURI}).Logger()
		ctx := context.New(w, req)

		ctx.RespWriter.Header().Set("Server", "pages-server")

		// Force new default from specification (since November 2020) - see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy#strict-origin-when-cross-origin
		ctx.RespWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable browser caching for up to 10 minutes
		ctx.RespWriter.Header().Set("Cache-Control", "public, max-age=600")

		trimmedHost := ctx.TrimHostPort()

		// Add HSTS for RawDomain and MainDomain
		if hsts := getHSTSHeader(trimmedHost, cfg.MainDomain, cfg.RawDomain); hsts != "" {
			ctx.RespWriter.Header().Set("Strict-Transport-Security", hsts)
		}

		// Handle all http methods
		ctx.RespWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodHead+", "+http.MethodOptions)
		switch ctx.Req.Method {
		case http.MethodOptions:
			// return Allow header
			ctx.RespWriter.WriteHeader(http.StatusNoContent)
			return
		case http.MethodGet,
			http.MethodHead:
			// end switch case and handle allowed requests
			break
		default:
			// Block all methods not required for static pages
			ctx.String("Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Block blacklisted paths (like ACME challenges)
		for _, blacklistedPath := range cfg.BlacklistedPaths {
			if strings.HasPrefix(ctx.Path(), blacklistedPath) {
				html.ReturnErrorPage(ctx, "requested path is blacklisted", http.StatusForbidden)
				return
			}
		}

		// Allow CORS for specified domains
		allowCors := false
		for _, allowedCorsDomain := range cfg.AllowedCorsDomains {
			if strings.EqualFold(trimmedHost, allowedCorsDomain) {
				allowCors = true
				break
			}
		}
		if allowCors {
			ctx.RespWriter.Header().Set(headerAccessControlAllowOrigin, "*")
			ctx.RespWriter.Header().Set(headerAccessControlAllowMethods, http.MethodGet+", "+http.MethodHead)
		}

		// Prepare request information to Gitea
		pathElements := strings.Split(strings.Trim(ctx.Path(), "/"), "/")

		if cfg.RawDomain != "" && strings.EqualFold(trimmedHost, cfg.RawDomain) {
			log.Debug().Msg("raw domain request detected")
			handleRaw(log, ctx, giteaClient,
				cfg.MainDomain,
				trimmedHost,
				pathElements,
				canonicalDomainCache, redirectsCache)
		} else if strings.HasSuffix(trimmedHost, cfg.MainDomain) {
			log.Debug().Msg("subdomain request detected")
			handleSubDomain(log, ctx, giteaClient,
				cfg.MainDomain,
				cfg.PagesBranches,
				trimmedHost,
				pathElements,
				canonicalDomainCache, redirectsCache)
		} else {
			log.Debug().Msg("custom domain request detected")
			handleCustomDomain(log, ctx, giteaClient,
				cfg.MainDomain,
				trimmedHost,
				pathElements,
				cfg.PagesBranches[0],
				dnsLookupCache, canonicalDomainCache, redirectsCache)
		}
	}
}
