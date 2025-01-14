package upstream

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
)

const (
	headerLastModified    = "Last-Modified"
	headerIfModifiedSince = "If-Modified-Since"

	rawMime = "text/plain; charset=utf-8"
)

// upstreamIndexPages lists pages that may be considered as index pages for directories.
var upstreamIndexPages = []string{
	"index.html",
}

// upstreamNotFoundPages lists pages that may be considered as custom 404 Not Found pages.
var upstreamNotFoundPages = []string{
	"404.html",
}

// Options provides various options for the upstream request.
type Options struct {
	TargetOwner  string
	TargetRepo   string
	TargetBranch string
	TargetPath   string

	// Used for debugging purposes.
	Host string

	TryIndexPages   bool
	BranchTimestamp time.Time
	// internal
	appendTrailingSlash bool
	redirectIfExists    string

	ServeRaw bool
}

// Upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func (o *Options) Upstream(ctx *context.Context, giteaClient *gitea.Client, redirectsCache cache.ICache) bool {
	log := log.With().Strs("upstream", []string{o.TargetOwner, o.TargetRepo, o.TargetBranch, o.TargetPath}).Logger()

	log.Debug().Msg("Start")

	if o.TargetOwner == "" || o.TargetRepo == "" {
		html.ReturnErrorPage(ctx, "forge client: either repo owner or name info is missing", http.StatusBadRequest)
		return true
	}

	// Check if the branch exists and when it was modified
	if o.BranchTimestamp.IsZero() {
		branchExist, err := o.GetBranchTimestamp(giteaClient)
		// handle 404
		if err != nil && errors.Is(err, gitea.ErrorNotFound) || !branchExist {
			html.ReturnErrorPage(ctx,
				fmt.Sprintf("branch <code>%q</code> for <code>%s/%s</code> not found", o.TargetBranch, o.TargetOwner, o.TargetRepo),
				http.StatusNotFound)
			return true
		}

		// handle unexpected errors
		if err != nil {
			html.ReturnErrorPage(ctx,
				fmt.Sprintf("could not get timestamp of branch <code>%q</code>: '%v'", o.TargetBranch, err),
				http.StatusFailedDependency)
			return true
		}
	}

	// Check if the browser has a cached version
	if ctx.Response() != nil {
		if ifModifiedSince, err := time.Parse(time.RFC1123, ctx.Response().Header.Get(headerIfModifiedSince)); err == nil {
			if ifModifiedSince.After(o.BranchTimestamp) {
				ctx.RespWriter.WriteHeader(http.StatusNotModified)
				log.Trace().Msg("check response against last modified: valid")
				return true
			}
		}
		log.Trace().Msg("check response against last modified: outdated")
	}

	log.Debug().Msg("Preparing")

	reader, header, statusCode, err := giteaClient.ServeRawContent(o.TargetOwner, o.TargetRepo, o.TargetBranch, o.TargetPath)
	if reader != nil {
		defer reader.Close()
	}

	log.Debug().Msg("Aquisting")

	// Handle not found error
	if err != nil && errors.Is(err, gitea.ErrorNotFound) {
		log.Debug().Msg("Handling not found error")
		// Get and match redirects
		redirects := o.getRedirects(giteaClient, redirectsCache)
		if o.matchRedirects(ctx, giteaClient, redirects, redirectsCache) {
			log.Trace().Msg("redirect")
			return true
		}

		if o.TryIndexPages {
			log.Trace().Msg("try index page")
			// copy the o struct & try if an index page exists
			optionsForIndexPages := *o
			optionsForIndexPages.TryIndexPages = false
			optionsForIndexPages.appendTrailingSlash = true
			for _, indexPage := range upstreamIndexPages {
				optionsForIndexPages.TargetPath = strings.TrimSuffix(o.TargetPath, "/") + "/" + indexPage
				if optionsForIndexPages.Upstream(ctx, giteaClient, redirectsCache) {
					return true
				}
			}
			log.Trace().Msg("try html file with path name")
			// compatibility fix for GitHub Pages (/example → /example.html)
			optionsForIndexPages.appendTrailingSlash = false
			optionsForIndexPages.redirectIfExists = strings.TrimSuffix(ctx.Path(), "/") + ".html"
			optionsForIndexPages.TargetPath = o.TargetPath + ".html"
			if optionsForIndexPages.Upstream(ctx, giteaClient, redirectsCache) {
				return true
			}
		}

		log.Trace().Msg("not found")

		ctx.StatusCode = http.StatusNotFound
		if o.TryIndexPages {
			log.Trace().Msg("try not found page")
			// copy the o struct & try if a not found page exists
			optionsForNotFoundPages := *o
			optionsForNotFoundPages.TryIndexPages = false
			optionsForNotFoundPages.appendTrailingSlash = false
			for _, notFoundPage := range upstreamNotFoundPages {
				optionsForNotFoundPages.TargetPath = "/" + notFoundPage
				if optionsForNotFoundPages.Upstream(ctx, giteaClient, redirectsCache) {
					return true
				}
			}
			log.Trace().Msg("not found page missing")
		}

		return false
	}

	// handle unexpected client errors
	if err != nil || reader == nil || statusCode != http.StatusOK {
		log.Debug().Msg("Handling error")
		var msg string

		if err != nil {
			msg = "forge client: returned unexpected error"
			log.Error().Err(err).Msg(msg)
			msg = fmt.Sprintf("%s: '%v'", msg, err)
		}
		if reader == nil {
			msg = "forge client: returned no reader"
			log.Error().Msg(msg)
		}
		if statusCode != http.StatusOK {
			msg = fmt.Sprintf("forge client: couldn't fetch contents: <code>%d - %s</code>", statusCode, http.StatusText(statusCode))
			log.Error().Msg(msg)
		}

		html.ReturnErrorPage(ctx, msg, http.StatusInternalServerError)
		return true
	}

	// Append trailing slash if missing (for index files), and redirect to fix filenames in general
	// o.appendTrailingSlash is only true when looking for index pages
	if o.appendTrailingSlash && !strings.HasSuffix(ctx.Path(), "/") {
		log.Trace().Msg("append trailing slash and redirect")
		ctx.Redirect(ctx.Path()+"/", http.StatusTemporaryRedirect)
		return true
	}
	if strings.HasSuffix(ctx.Path(), "/index.html") && !o.ServeRaw {
		log.Trace().Msg("remove index.html from path and redirect")
		ctx.Redirect(strings.TrimSuffix(ctx.Path(), "index.html"), http.StatusTemporaryRedirect)
		return true
	}
	if o.redirectIfExists != "" {
		ctx.Redirect(o.redirectIfExists, http.StatusTemporaryRedirect)
		return true
	}

	// Set ETag & MIME
	o.setHeader(ctx, header)

	log.Debug().Msg("Prepare response")

	ctx.RespWriter.WriteHeader(ctx.StatusCode)

	// Write the response body to the original request
	if reader != nil {
		_, err := io.Copy(ctx.RespWriter, reader)
		if err != nil {
			log.Error().Err(err).Msgf("Couldn't write body for %q", o.TargetPath)
			html.ReturnErrorPage(ctx, "", http.StatusInternalServerError)
			return true
		}
	}

	log.Debug().Msg("Sending response")

	return true
}
