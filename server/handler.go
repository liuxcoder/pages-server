package server

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OrlovEvgeny/go-mcache"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"codeberg.org/codeberg/pages/html"
)

// Handler handles a single HTTP request to the web server.
func Handler(mainDomainSuffix, rawDomain, giteaRoot []byte, rawInfoPage, giteaApiToken string, blacklistedPaths, allowedCorsDomains [][]byte) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		log := log.With().Str("Handler", string(ctx.Request.Header.RequestURI())).Logger()

		ctx.Response.Header.Set("Server", "Codeberg Pages")

		// Force new default from specification (since November 2020) - see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy#strict-origin-when-cross-origin
		ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable browser caching for up to 10 minutes
		ctx.Response.Header.Set("Cache-Control", "public, max-age=600")

		trimmedHost := TrimHostPort(ctx.Request.Host())

		// Add HSTS for RawDomain and MainDomainSuffix
		if hsts := GetHSTSHeader(trimmedHost, mainDomainSuffix, rawDomain); hsts != "" {
			ctx.Response.Header.Set("Strict-Transport-Security", hsts)
		}

		// Block all methods not required for static pages
		if !ctx.IsGet() && !ctx.IsHead() && !ctx.IsOptions() {
			ctx.Response.Header.Set("Allow", "GET, HEAD, OPTIONS")
			ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			return
		}

		// Block blacklisted paths (like ACME challenges)
		for _, blacklistedPath := range blacklistedPaths {
			if bytes.HasPrefix(ctx.Path(), blacklistedPath) {
				returnErrorPage(ctx, fasthttp.StatusForbidden)
				return
			}
		}

		// Allow CORS for specified domains
		if ctx.IsOptions() {
			allowCors := false
			for _, allowedCorsDomain := range allowedCorsDomains {
				if bytes.Equal(trimmedHost, allowedCorsDomain) {
					allowCors = true
					break
				}
			}
			if allowCors {
				ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
				ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, HEAD")
			}
			ctx.Response.Header.Set("Allow", "GET, HEAD, OPTIONS")
			ctx.Response.Header.SetStatusCode(fasthttp.StatusNoContent)
			return
		}

		// Prepare request information to Gitea
		var targetOwner, targetRepo, targetBranch, targetPath string
		var targetOptions = &upstreamOptions{
			ForbiddenMimeTypes: map[string]struct{}{},
			TryIndexPages:      true,
		}

		// tryBranch checks if a branch exists and populates the target variables. If canonicalLink is non-empty, it will
		// also disallow search indexing and add a Link header to the canonical URL.
		var tryBranch = func(repo string, branch string, path []string, canonicalLink string) bool {
			if repo == "" {
				return false
			}

			// Check if the branch exists, otherwise treat it as a file path
			branchTimestampResult := getBranchTimestamp(targetOwner, repo, branch, string(giteaRoot), giteaApiToken)
			if branchTimestampResult == nil {
				// branch doesn't exist
				return false
			}

			// Branch exists, use it
			targetRepo = repo
			targetPath = strings.Trim(strings.Join(path, "/"), "/")
			targetBranch = branchTimestampResult.branch

			targetOptions.BranchTimestamp = branchTimestampResult.timestamp

			if canonicalLink != "" {
				// Hide from search machines & add canonical link
				ctx.Response.Header.Set("X-Robots-Tag", "noarchive, noindex")
				ctx.Response.Header.Set("Link",
					strings.NewReplacer("%b", targetBranch, "%p", targetPath).Replace(canonicalLink)+
						"; rel=\"canonical\"",
				)
			}

			return true
		}

		// tryUpstream forwards the target request to the Gitea API, and shows an error page on failure.
		var tryUpstream = func() {
			// check if a canonical domain exists on a request on MainDomain
			if bytes.HasSuffix(trimmedHost, mainDomainSuffix) {
				canonicalDomain, _ := checkCanonicalDomain(targetOwner, targetRepo, targetBranch, "", string(mainDomainSuffix), string(giteaRoot), giteaApiToken)
				if !strings.HasSuffix(strings.SplitN(canonicalDomain, "/", 2)[0], string(mainDomainSuffix)) {
					canonicalPath := string(ctx.RequestURI())
					if targetRepo != "pages" {
						canonicalPath = "/" + strings.SplitN(canonicalPath, "/", 3)[2]
					}
					ctx.Redirect("https://"+canonicalDomain+canonicalPath, fasthttp.StatusTemporaryRedirect)
					return
				}
			}

			// Try to request the file from the Gitea API
			if !upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, string(giteaRoot), giteaApiToken, targetOptions) {
				returnErrorPage(ctx, ctx.Response.StatusCode())
			}
		}

		log.Debug().Msg("preparations")

		if rawDomain != nil && bytes.Equal(trimmedHost, rawDomain) {
			// Serve raw content from RawDomain
			log.Debug().Msg("raw domain")

			targetOptions.TryIndexPages = false
			targetOptions.ForbiddenMimeTypes["text/html"] = struct{}{}
			targetOptions.DefaultMimeType = "text/plain; charset=utf-8"

			pathElements := strings.Split(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/")
			if len(pathElements) < 2 {
				// https://{RawDomain}/{owner}/{repo}[/@{branch}]/{path} is required
				ctx.Redirect(rawInfoPage, fasthttp.StatusTemporaryRedirect)
				return
			}
			targetOwner = pathElements[0]
			targetRepo = pathElements[1]

			// raw.codeberg.org/example/myrepo/@main/index.html
			if len(pathElements) > 2 && strings.HasPrefix(pathElements[2], "@") {
				log.Debug().Msg("raw domain preparations, now trying with specified branch")
				if tryBranch(targetRepo, pathElements[2][1:], pathElements[3:],
					string(giteaRoot)+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
				) {
					log.Debug().Msg("tryBranch, now trying upstream")
					tryUpstream()
					return
				}
				log.Debug().Msg("missing branch")
				returnErrorPage(ctx, fasthttp.StatusFailedDependency)
				return
			} else {
				log.Debug().Msg("raw domain preparations, now trying with default branch")
				tryBranch(targetRepo, "", pathElements[2:],
					string(giteaRoot)+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
				)
				log.Debug().Msg("tryBranch, now trying upstream")
				tryUpstream()
				return
			}

		} else if bytes.HasSuffix(trimmedHost, mainDomainSuffix) {
			// Serve pages from subdomains of MainDomainSuffix
			log.Debug().Msg("main domain suffix")

			pathElements := strings.Split(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/")
			targetOwner = string(bytes.TrimSuffix(trimmedHost, mainDomainSuffix))
			targetRepo = pathElements[0]
			targetPath = strings.Trim(strings.Join(pathElements[1:], "/"), "/")

			if targetOwner == "www" {
				// www.codeberg.page redirects to codeberg.page
				ctx.Redirect("https://"+string(mainDomainSuffix[1:])+string(ctx.Path()), fasthttp.StatusPermanentRedirect)
				return
			}

			// Check if the first directory is a repo with the second directory as a branch
			// example.codeberg.page/myrepo/@main/index.html
			if len(pathElements) > 1 && strings.HasPrefix(pathElements[1], "@") {
				if targetRepo == "pages" {
					// example.codeberg.org/pages/@... redirects to example.codeberg.org/@...
					ctx.Redirect("/"+strings.Join(pathElements[1:], "/"), fasthttp.StatusTemporaryRedirect)
					return
				}

				log.Debug().Msg("main domain preparations, now trying with specified repo & branch")
				if tryBranch(pathElements[0], pathElements[1][1:], pathElements[2:],
					"/"+pathElements[0]+"/%p",
				) {
					log.Debug().Msg("tryBranch, now trying upstream")
					tryUpstream()
				} else {
					returnErrorPage(ctx, fasthttp.StatusFailedDependency)
				}
				return
			}

			// Check if the first directory is a branch for the "pages" repo
			// example.codeberg.page/@main/index.html
			if strings.HasPrefix(pathElements[0], "@") {
				log.Debug().Msg("main domain preparations, now trying with specified branch")
				if tryBranch("pages", pathElements[0][1:], pathElements[1:], "/%p") {
					log.Debug().Msg("tryBranch, now trying upstream")
					tryUpstream()
				} else {
					returnErrorPage(ctx, fasthttp.StatusFailedDependency)
				}
				return
			}

			// Check if the first directory is a repo with a "pages" branch
			// example.codeberg.page/myrepo/index.html
			// example.codeberg.page/pages/... is not allowed here.
			log.Debug().Msg("main domain preparations, now trying with specified repo")
			if pathElements[0] != "pages" && tryBranch(pathElements[0], "pages", pathElements[1:], "") {
				log.Debug().Msg("tryBranch, now trying upstream")
				tryUpstream()
				return
			}

			// Try to use the "pages" repo on its default branch
			// example.codeberg.page/index.html
			log.Debug().Msg("main domain preparations, now trying with default repo/branch")
			if tryBranch("pages", "", pathElements, "") {
				log.Debug().Msg("tryBranch, now trying upstream")
				tryUpstream()
				return
			}

			// Couldn't find a valid repo/branch
			returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return
		} else {
			trimmedHostStr := string(trimmedHost)

			// Serve pages from external domains
			targetOwner, targetRepo, targetBranch = getTargetFromDNS(trimmedHostStr, string(mainDomainSuffix))
			if targetOwner == "" {
				returnErrorPage(ctx, fasthttp.StatusFailedDependency)
				return
			}

			pathElements := strings.Split(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/")
			canonicalLink := ""
			if strings.HasPrefix(pathElements[0], "@") {
				targetBranch = pathElements[0][1:]
				pathElements = pathElements[1:]
				canonicalLink = "/%p"
			}

			// Try to use the given repo on the given branch or the default branch
			log.Debug().Msg("custom domain preparations, now trying with details from DNS")
			if tryBranch(targetRepo, targetBranch, pathElements, canonicalLink) {
				canonicalDomain, valid := checkCanonicalDomain(targetOwner, targetRepo, targetBranch, trimmedHostStr, string(mainDomainSuffix), string(giteaRoot), giteaApiToken)
				if !valid {
					returnErrorPage(ctx, fasthttp.StatusMisdirectedRequest)
					return
				} else if canonicalDomain != trimmedHostStr {
					// only redirect if the target is also a codeberg page!
					targetOwner, _, _ = getTargetFromDNS(strings.SplitN(canonicalDomain, "/", 2)[0], string(mainDomainSuffix))
					if targetOwner != "" {
						ctx.Redirect("https://"+canonicalDomain+string(ctx.RequestURI()), fasthttp.StatusTemporaryRedirect)
						return
					} else {
						returnErrorPage(ctx, fasthttp.StatusFailedDependency)
						return
					}
				}

				log.Debug().Msg("tryBranch, now trying upstream")
				tryUpstream()
				return
			} else {
				returnErrorPage(ctx, fasthttp.StatusFailedDependency)
				return
			}
		}
	}
}

// returnErrorPage sets the response status code and writes NotFoundPage to the response body, with "%status" replaced
// with the provided status code.
func returnErrorPage(ctx *fasthttp.RequestCtx, code int) {
	ctx.Response.SetStatusCode(code)
	ctx.Response.Header.SetContentType("text/html; charset=utf-8")
	message := fasthttp.StatusMessage(code)
	if code == fasthttp.StatusMisdirectedRequest {
		message += " - domain not specified in <code>.domains</code> file"
	}
	if code == fasthttp.StatusFailedDependency {
		message += " - target repo/branch doesn't exist or is private"
	}
	// TODO: use template engine?
	ctx.Response.SetBody(bytes.ReplaceAll(html.NotFoundPage, []byte("%status"), []byte(strconv.Itoa(code)+" "+message)))
}

// DefaultBranchCacheTimeout specifies the timeout for the default branch cache. It can be quite long.
var DefaultBranchCacheTimeout = 15 * time.Minute

// BranchExistanceCacheTimeout specifies the timeout for the branch timestamp & existance cache. It should be shorter
// than FileCacheTimeout, as that gets invalidated if the branch timestamp has changed. That way, repo changes will be
// picked up faster, while still allowing the content to be cached longer if nothing changes.
var BranchExistanceCacheTimeout = 5 * time.Minute

// branchTimestampCache stores branch timestamps for faster cache checking
var branchTimestampCache = mcache.New()

type branchTimestamp struct {
	branch    string
	timestamp time.Time
}

// FileCacheTimeout specifies the timeout for the file content cache - you might want to make this quite long, depending
// on your available memory.
var FileCacheTimeout = 5 * time.Minute

// FileCacheSizeLimit limits the maximum file size that will be cached, and is set to 1 MB by default.
var FileCacheSizeLimit = 1024 * 1024

// fileResponseCache stores responses from the Gitea server
// TODO: make this an MRU cache with a size limit
var fileResponseCache = mcache.New()

type fileResponse struct {
	exists   bool
	mimeType string
	body     []byte
}

// getBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or nil if the branch doesn't exist)
func getBranchTimestamp(owner, repo, branch, giteaRoot, giteaApiToken string) *branchTimestamp {
	if result, ok := branchTimestampCache.Get(owner + "/" + repo + "/" + branch); ok {
		if result == nil {
			return nil
		}
		return result.(*branchTimestamp)
	}
	result := &branchTimestamp{}
	result.branch = branch
	if branch == "" {
		// Get default branch
		var body = make([]byte, 0)
		// TODO: use header for API key?
		status, body, err := fasthttp.GetTimeout(body, giteaRoot+"/api/v1/repos/"+owner+"/"+repo+"?access_token="+giteaApiToken, 5*time.Second)
		if err != nil || status != 200 {
			_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, nil, DefaultBranchCacheTimeout)
			return nil
		}
		result.branch = fastjson.GetString(body, "default_branch")
	}

	var body = make([]byte, 0)
	status, body, err := fasthttp.GetTimeout(body, giteaRoot+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch+"?access_token="+giteaApiToken, 5*time.Second)
	if err != nil || status != 200 {
		return nil
	}

	result.timestamp, _ = time.Parse(time.RFC3339, fastjson.GetString(body, "commit", "timestamp"))
	_ = branchTimestampCache.Set(owner+"/"+repo+"/"+branch, result, BranchExistanceCacheTimeout)
	return result
}

var upstreamClient = fasthttp.Client{
	ReadTimeout:        10 * time.Second,
	MaxConnDuration:    60 * time.Second,
	MaxConnWaitTimeout: 1000 * time.Millisecond,
	MaxConnsPerHost:    128 * 16, // TODO: adjust bottlenecks for best performance with Gitea!
}

// upstreamIndexPages lists pages that may be considered as index pages for directories.
var upstreamIndexPages = []string{
	"index.html",
}

// upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func upstream(ctx *fasthttp.RequestCtx, targetOwner, targetRepo, targetBranch, targetPath, giteaRoot, giteaApiToken string, options *upstreamOptions) (final bool) {
	log := log.With().Strs("upstream", []string{targetOwner, targetRepo, targetBranch, targetPath}).Logger()

	if options.ForbiddenMimeTypes == nil {
		options.ForbiddenMimeTypes = map[string]struct{}{}
	}

	// Check if the branch exists and when it was modified
	if options.BranchTimestamp == (time.Time{}) {
		branch := getBranchTimestamp(targetOwner, targetRepo, targetBranch, giteaRoot, giteaApiToken)

		if branch == nil {
			returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return true
		}
		targetBranch = branch.branch
		options.BranchTimestamp = branch.timestamp
	}

	if targetOwner == "" || targetRepo == "" || targetBranch == "" {
		returnErrorPage(ctx, fasthttp.StatusBadRequest)
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
		err = upstreamClient.Do(req, res)
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
				if upstream(ctx, targetOwner, targetRepo, targetBranch, strings.TrimSuffix(targetPath, "/")+"/"+indexPage, giteaRoot, giteaApiToken, &optionsForIndexPages) {
					_ = fileResponseCache.Set(uri+"?timestamp="+strconv.FormatInt(options.BranchTimestamp.Unix(), 10), fileResponse{
						exists: false,
					}, FileCacheTimeout)
					return true
				}
			}
			// compatibility fix for GitHub Pages (/example → /example.html)
			optionsForIndexPages.AppendTrailingSlash = false
			optionsForIndexPages.RedirectIfExists = string(ctx.Request.URI().Path()) + ".html"
			if upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath+".html", giteaRoot, giteaApiToken, &optionsForIndexPages) {
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
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
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
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
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

// upstreamOptions provides various options for the upstream request.
type upstreamOptions struct {
	DefaultMimeType     string
	ForbiddenMimeTypes  map[string]struct{}
	TryIndexPages       bool
	AppendTrailingSlash bool
	RedirectIfExists    string
	BranchTimestamp     time.Time
}
