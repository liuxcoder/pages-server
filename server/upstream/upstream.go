package upstream

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"

	"codeberg.org/codeberg/pages/html"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// upstreamIndexPages lists pages that may be considered as index pages for directories.
var upstreamIndexPages = []string{
	"index.html",
}

// Options provides various options for the upstream request.
type Options struct {
	DefaultMimeType     string
	ForbiddenMimeTypes  map[string]struct{}
	TryIndexPages       bool
	AppendTrailingSlash bool
	RedirectIfExists    string
	BranchTimestamp     time.Time
}

var Client = fasthttp.Client{
	ReadTimeout:        10 * time.Second,
	MaxConnDuration:    60 * time.Second,
	MaxConnWaitTimeout: 1000 * time.Millisecond,
	MaxConnsPerHost:    128 * 16, // TODO: adjust bottlenecks for best performance with Gitea!
}

// Upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func Upstream(ctx *fasthttp.RequestCtx, targetOwner, targetRepo, targetBranch, targetPath, giteaRoot, giteaApiToken string, options *Options) (final bool) {
	log := log.With().Strs("upstream", []string{targetOwner, targetRepo, targetBranch, targetPath}).Logger()

	if options.ForbiddenMimeTypes == nil {
		options.ForbiddenMimeTypes = map[string]struct{}{}
	}

	// Check if the branch exists and when it was modified
	if options.BranchTimestamp == (time.Time{}) {
		branch := GetBranchTimestamp(targetOwner, targetRepo, targetBranch, giteaRoot, giteaApiToken)

		if branch == nil {
			html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return true
		}
		targetBranch = branch.Branch
		options.BranchTimestamp = branch.Timestamp
	}

	if targetOwner == "" || targetRepo == "" || targetBranch == "" {
		html.ReturnErrorPage(ctx, fasthttp.StatusBadRequest)
		return true
	}

	// Check if the browser has a cached version
	if ifModifiedSince, err := time.Parse(time.RFC1123, string(ctx.Request.Header.Peek("If-Modified-Since"))); err == nil {
		if !ifModifiedSince.Before(options.BranchTimestamp) {
			ctx.Response.SetStatusCode(fasthttp.StatusNotModified)
			return true
		}
	}
	log.Debug().Msg("preparations")

	// Make a GET request to the upstream URL
	uri := targetOwner + "/" + targetRepo + "/raw/" + targetBranch + "/" + targetPath
	var req *fasthttp.Request
	var res *fasthttp.Response
	var cachedResponse fileResponse
	var err error
	if cachedValue, ok := fileResponseCache.Get(uri + "?timestamp=" + strconv.FormatInt(options.BranchTimestamp.Unix(), 10)); ok && len(cachedValue.(fileResponse).body) > 0 {
		cachedResponse = cachedValue.(fileResponse)
	} else {
		req = fasthttp.AcquireRequest()
		req.SetRequestURI(giteaRoot + "/api/v1/repos/" + uri + "?access_token=" + giteaApiToken)
		res = fasthttp.AcquireResponse()
		res.SetBodyStream(&strings.Reader{}, -1)
		err = Client.Do(req, res)
	}
	log.Debug().Msg("acquisition")

	// Handle errors
	if (res == nil && !cachedResponse.exists) || (res != nil && res.StatusCode() == fasthttp.StatusNotFound) {
		if options.TryIndexPages {
			// copy the options struct & try if an index page exists
			optionsForIndexPages := *options
			optionsForIndexPages.TryIndexPages = false
			optionsForIndexPages.AppendTrailingSlash = true
			for _, indexPage := range upstreamIndexPages {
				if Upstream(ctx, targetOwner, targetRepo, targetBranch, strings.TrimSuffix(targetPath, "/")+"/"+indexPage, giteaRoot, giteaApiToken, &optionsForIndexPages) {
					_ = fileResponseCache.Set(uri+"?timestamp="+strconv.FormatInt(options.BranchTimestamp.Unix(), 10), fileResponse{
						exists: false,
					}, FileCacheTimeout)
					return true
				}
			}
			// compatibility fix for GitHub Pages (/example → /example.html)
			optionsForIndexPages.AppendTrailingSlash = false
			optionsForIndexPages.RedirectIfExists = string(ctx.Request.URI().Path()) + ".html"
			if Upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath+".html", giteaRoot, giteaApiToken, &optionsForIndexPages) {
				_ = fileResponseCache.Set(uri+"?timestamp="+strconv.FormatInt(options.BranchTimestamp.Unix(), 10), fileResponse{
					exists: false,
				}, FileCacheTimeout)
				return true
			}
		}
		ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
		if res != nil {
			// Update cache if the request is fresh
			_ = fileResponseCache.Set(uri+"?timestamp="+strconv.FormatInt(options.BranchTimestamp.Unix(), 10), fileResponse{
				exists: false,
			}, FileCacheTimeout)
		}
		return false
	}
	if res != nil && (err != nil || res.StatusCode() != fasthttp.StatusOK) {
		fmt.Printf("Couldn't fetch contents from \"%s\": %s (status code %d)\n", req.RequestURI(), err, res.StatusCode())
		html.ReturnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}

	// Append trailing slash if missing (for index files), and redirect to fix filenames in general
	// options.AppendTrailingSlash is only true when looking for index pages
	if options.AppendTrailingSlash && !bytes.HasSuffix(ctx.Request.URI().Path(), []byte{'/'}) {
		ctx.Redirect(string(ctx.Request.URI().Path())+"/", fasthttp.StatusTemporaryRedirect)
		return true
	}
	if bytes.HasSuffix(ctx.Request.URI().Path(), []byte("/index.html")) {
		ctx.Redirect(strings.TrimSuffix(string(ctx.Request.URI().Path()), "index.html"), fasthttp.StatusTemporaryRedirect)
		return true
	}
	if options.RedirectIfExists != "" {
		ctx.Redirect(options.RedirectIfExists, fasthttp.StatusTemporaryRedirect)
		return true
	}
	log.Debug().Msg("error handling")

	// Set the MIME type
	mimeType := mime.TypeByExtension(path.Ext(targetPath))
	mimeTypeSplit := strings.SplitN(mimeType, ";", 2)
	if _, ok := options.ForbiddenMimeTypes[mimeTypeSplit[0]]; ok || mimeType == "" {
		if options.DefaultMimeType != "" {
			mimeType = options.DefaultMimeType
		} else {
			mimeType = "application/octet-stream"
		}
	}
	ctx.Response.Header.SetContentType(mimeType)

	// Everything's okay so far
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.SetLastModified(options.BranchTimestamp)

	log.Debug().Msg("response preparations")

	// Write the response body to the original request
	var cacheBodyWriter bytes.Buffer
	if res != nil {
		if res.Header.ContentLength() > FileCacheSizeLimit {
			err = res.BodyWriteTo(ctx.Response.BodyWriter())
		} else {
			// TODO: cache is half-empty if request is cancelled - does the ctx.Err() below do the trick?
			err = res.BodyWriteTo(io.MultiWriter(ctx.Response.BodyWriter(), &cacheBodyWriter))
		}
	} else {
		_, err = ctx.Write(cachedResponse.body)
	}
	if err != nil {
		fmt.Printf("Couldn't write body for \"%s\": %s\n", req.RequestURI(), err)
		html.ReturnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}
	log.Debug().Msg("response")

	if res != nil && ctx.Err() == nil {
		cachedResponse.exists = true
		cachedResponse.mimeType = mimeType
		cachedResponse.body = cacheBodyWriter.Bytes()
		_ = fileResponseCache.Set(uri+"?timestamp="+strconv.FormatInt(options.BranchTimestamp.Unix(), 10), cachedResponse, FileCacheTimeout)
	}

	return true
}
