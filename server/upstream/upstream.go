package upstream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
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
	TargetOwner,
	TargetRepo,
	TargetBranch,
	TargetPath,

	DefaultMimeType string
	ForbiddenMimeTypes map[string]bool
	TryIndexPages      bool
	BranchTimestamp    time.Time
	// internal
	appendTrailingSlash bool
	redirectIfExists    string
}

// Upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func (o *Options) Upstream(ctx *fasthttp.RequestCtx, giteaClient *gitea.Client, branchTimestampCache, fileResponseCache cache.SetGetKey) (final bool) {
	log := log.With().Strs("upstream", []string{o.TargetOwner, o.TargetRepo, o.TargetBranch, o.TargetPath}).Logger()

	// Check if the branch exists and when it was modified
	if o.BranchTimestamp.IsZero() {
		branch := GetBranchTimestamp(giteaClient, o.TargetOwner, o.TargetRepo, o.TargetBranch, branchTimestampCache)

		if branch == nil {
			html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return true
		}
		o.TargetBranch = branch.Branch
		o.BranchTimestamp = branch.Timestamp
	}

	if o.TargetOwner == "" || o.TargetRepo == "" || o.TargetBranch == "" {
		html.ReturnErrorPage(ctx, fasthttp.StatusBadRequest)
		return true
	}

	// Check if the browser has a cached version
	if ifModifiedSince, err := time.Parse(time.RFC1123, string(ctx.Request.Header.Peek("If-Modified-Since"))); err == nil {
		if !ifModifiedSince.Before(o.BranchTimestamp) {
			ctx.Response.SetStatusCode(fasthttp.StatusNotModified)
			return true
		}
	}
	log.Debug().Msg("preparations")

	// Make a GET request to the upstream URL
	uri := o.generateUri()
	var res *fasthttp.Response
	var cachedResponse gitea.FileResponse
	var err error
	if cachedValue, ok := fileResponseCache.Get(uri + "?timestamp=" + o.timestamp()); ok && !cachedValue.(gitea.FileResponse).IsEmpty() {
		cachedResponse = cachedValue.(gitea.FileResponse)
	} else {
		res, err = giteaClient.ServeRawContent(uri)
	}
	log.Debug().Msg("acquisition")

	// Handle errors
	if (err != nil && errors.Is(err, gitea.ErrorNotFound)) || (res == nil && !cachedResponse.Exists) {
		if o.TryIndexPages {
			// copy the o struct & try if an index page exists
			optionsForIndexPages := *o
			optionsForIndexPages.TryIndexPages = false
			optionsForIndexPages.appendTrailingSlash = true
			for _, indexPage := range upstreamIndexPages {
				optionsForIndexPages.TargetPath = strings.TrimSuffix(o.TargetPath, "/") + "/" + indexPage
				if optionsForIndexPages.Upstream(ctx, giteaClient, branchTimestampCache, fileResponseCache) {
					_ = fileResponseCache.Set(uri+"?timestamp="+o.timestamp(), gitea.FileResponse{
						Exists: false,
					}, fileCacheTimeout)
					return true
				}
			}
			// compatibility fix for GitHub Pages (/example → /example.html)
			optionsForIndexPages.appendTrailingSlash = false
			optionsForIndexPages.redirectIfExists = strings.TrimSuffix(string(ctx.Request.URI().Path()), "/") + ".html"
			optionsForIndexPages.TargetPath = o.TargetPath + ".html"
			if optionsForIndexPages.Upstream(ctx, giteaClient, branchTimestampCache, fileResponseCache) {
				_ = fileResponseCache.Set(uri+"?timestamp="+o.timestamp(), gitea.FileResponse{
					Exists: false,
				}, fileCacheTimeout)
				return true
			}
		}
		ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
		if o.TryIndexPages {
			// copy the o struct & try if a not found page exists
			optionsForNotFoundPages := *o
			optionsForNotFoundPages.TryIndexPages = false
			optionsForNotFoundPages.appendTrailingSlash = false
			for _, notFoundPage := range upstreamNotFoundPages {
				optionsForNotFoundPages.TargetPath = "/" + notFoundPage
				if optionsForNotFoundPages.Upstream(ctx, giteaClient, branchTimestampCache, fileResponseCache) {
					_ = fileResponseCache.Set(uri+"?timestamp="+o.timestamp(), gitea.FileResponse{
						Exists: false,
					}, fileCacheTimeout)
					return true
				}
			}
		}
		if res != nil {
			// Update cache if the request is fresh
			_ = fileResponseCache.Set(uri+"?timestamp="+o.timestamp(), gitea.FileResponse{
				Exists: false,
			}, fileCacheTimeout)
		}
		return false
	}
	if res != nil && (err != nil || res.StatusCode() != fasthttp.StatusOK) {
		fmt.Printf("Couldn't fetch contents from \"%s\": %s (status code %d)\n", uri, err, res.StatusCode())
		html.ReturnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}

	// Append trailing slash if missing (for index files), and redirect to fix filenames in general
	// o.appendTrailingSlash is only true when looking for index pages
	if o.appendTrailingSlash && !bytes.HasSuffix(ctx.Request.URI().Path(), []byte{'/'}) {
		ctx.Redirect(string(ctx.Request.URI().Path())+"/", fasthttp.StatusTemporaryRedirect)
		return true
	}
	if bytes.HasSuffix(ctx.Request.URI().Path(), []byte("/index.html")) {
		ctx.Redirect(strings.TrimSuffix(string(ctx.Request.URI().Path()), "index.html"), fasthttp.StatusTemporaryRedirect)
		return true
	}
	if o.redirectIfExists != "" {
		ctx.Redirect(o.redirectIfExists, fasthttp.StatusTemporaryRedirect)
		return true
	}
	log.Debug().Msg("error handling")

	// Set the MIME type
	mimeType := o.getMimeTypeByExtension()
	ctx.Response.Header.SetContentType(mimeType)

	// Set ETag
	if cachedResponse.Exists {
		ctx.Response.Header.SetBytesV(fasthttp.HeaderETag, cachedResponse.ETag)
	} else if res != nil {
		cachedResponse.ETag = res.Header.Peek(fasthttp.HeaderETag)
		ctx.Response.Header.SetBytesV(fasthttp.HeaderETag, cachedResponse.ETag)
	}

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		// Everything's okay so far
		ctx.Response.SetStatusCode(fasthttp.StatusOK)
	}
	ctx.Response.Header.SetLastModified(o.BranchTimestamp)

	log.Debug().Msg("response preparations")

	// Write the response body to the original request
	var cacheBodyWriter bytes.Buffer
	if res != nil {
		if res.Header.ContentLength() > fileCacheSizeLimit {
			err = res.BodyWriteTo(ctx.Response.BodyWriter())
		} else {
			// TODO: cache is half-empty if request is cancelled - does the ctx.Err() below do the trick?
			err = res.BodyWriteTo(io.MultiWriter(ctx.Response.BodyWriter(), &cacheBodyWriter))
		}
	} else {
		_, err = ctx.Write(cachedResponse.Body)
	}
	if err != nil {
		fmt.Printf("Couldn't write body for \"%s\": %s\n", uri, err)
		html.ReturnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}
	log.Debug().Msg("response")

	if res != nil && ctx.Err() == nil {
		cachedResponse.Exists = true
		cachedResponse.MimeType = mimeType
		cachedResponse.Body = cacheBodyWriter.Bytes()
		_ = fileResponseCache.Set(uri+"?timestamp="+o.timestamp(), cachedResponse, fileCacheTimeout)
	}

	return true
}
