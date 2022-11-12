package handler

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/version"
)

const (
	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
)

// Handler handles a single HTTP request to the web server.
func Handler(mainDomainSuffix, rawDomain string,
	giteaClient *gitea.Client,
	rawInfoPage string,
	blacklistedPaths, allowedCorsDomains []string,
	dnsLookupCache, canonicalDomainCache cache.SetGetKey,
) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log := log.With().Strs("Handler", []string{string(req.Host), req.RequestURI}).Logger()
		ctx := context.New(w, req)

		ctx.RespWriter.Header().Set("Server", "CodebergPages/"+version.Version)

		// Force new default from specification (since November 2020) - see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy#strict-origin-when-cross-origin
		ctx.RespWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable browser caching for up to 10 minutes
		ctx.RespWriter.Header().Set("Cache-Control", "public, max-age=600")

		trimmedHost := ctx.TrimHostPort()

		// Add HSTS for RawDomain and MainDomainSuffix
		if hsts := getHSTSHeader(trimmedHost, mainDomainSuffix, rawDomain); hsts != "" {
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
		for _, blacklistedPath := range blacklistedPaths {
			if strings.HasPrefix(ctx.Path(), blacklistedPath) {
				html.ReturnErrorPage(ctx, "requested blacklisted path", http.StatusForbidden)
				return
			}
		}

		// Allow CORS for specified domains
		allowCors := false
		for _, allowedCorsDomain := range allowedCorsDomains {
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

		if rawDomain != "" && strings.EqualFold(trimmedHost, rawDomain) {
			log.Debug().Msg("raw domain request detecded")
			handleRaw(log, ctx, giteaClient,
				mainDomainSuffix, rawInfoPage,
				trimmedHost,
				pathElements,
				canonicalDomainCache)
		} else if strings.HasSuffix(trimmedHost, mainDomainSuffix) {
			log.Debug().Msg("subdomain request detecded")
			handleSubDomain(log, ctx, giteaClient,
				mainDomainSuffix,
				trimmedHost,
				pathElements,
				canonicalDomainCache)
		} else {
			log.Debug().Msg("custom domain request detecded")
			handleCustomDomain(log, ctx, giteaClient,
				mainDomainSuffix,
				trimmedHost,
				pathElements,
				dnsLookupCache, canonicalDomainCache)
		}
	}
}
