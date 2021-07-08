package main

import (
	"bytes"
	"fmt"
	"github.com/OrlovEvgeny/go-mcache"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"io"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"
)

// handler handles a single HTTP request to the web server.
func handler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Server", "Codeberg Pages")

	// Force new default from specification (since November 2020) - see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy#strict-origin-when-cross-origin
	ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Enable caching, but require revalidation to reduce confusion
	ctx.Response.Header.Set("Cache-Control", "must-revalidate")

	// Block all methods not required for static pages
	if !ctx.IsGet() && !ctx.IsHead() && !ctx.IsOptions() {
		ctx.Response.Header.Set("Allow", "GET, HEAD, OPTIONS")
		ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
		return
	}

	// Block blacklisted paths (like ACME challenges)
	for _, blacklistedPath := range BlacklistedPaths {
		if bytes.HasPrefix(ctx.Path(), blacklistedPath) {
			returnErrorPage(ctx, fasthttp.StatusForbidden)
			return
		}
	}

	// Allow CORS for specified domains
	if ctx.IsOptions() {
		allowCors := false
		for _, allowedCorsDomain := range AllowedCorsDomains {
			if bytes.Equal(ctx.Request.Host(), allowedCorsDomain) {
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
		branchTimestampResult := getBranchTimestamp(targetOwner, repo, branch)
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
		if bytes.HasSuffix(ctx.Request.Host(), MainDomainSuffix) {
			canonicalDomain := checkCanonicalDomain(targetOwner, targetRepo, targetBranch)
			if !strings.HasSuffix(strings.SplitN(canonicalDomain, "/", 2)[0], string(MainDomainSuffix)) {
				canonicalPath := string(ctx.RequestURI())
				if targetRepo != "pages" {
					canonicalPath = "/" + strings.SplitN(canonicalPath, "/", 3)[2]
				}
				ctx.Redirect("https://" + canonicalDomain + canonicalPath, fasthttp.StatusTemporaryRedirect)
				return
			}
		}

		// Try to request the file from the Gitea API
		if !upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, targetOptions) {
			returnErrorPage(ctx, ctx.Response.StatusCode())
		}
	}

	if RawDomain != nil && bytes.Equal(ctx.Request.Host(), RawDomain) {
		// Serve raw content from RawDomain

		targetOptions.TryIndexPages = false
		targetOptions.ForbiddenMimeTypes["text/html"] = struct{}{}
		targetOptions.DefaultMimeType = "text/plain; charset=utf-8"

		pathElements := strings.Split(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/")
		if len(pathElements) < 2 {
			// https://{RawDomain}/{owner}/{repo}[/@{branch}]/{path} is required
			ctx.Redirect(RawInfoPage, fasthttp.StatusTemporaryRedirect)
			return
		}
		targetOwner = pathElements[0]
		targetRepo = pathElements[1]

		// raw.codeberg.page/example/myrepo/@main/index.html
		if len(pathElements) > 2 && strings.HasPrefix(pathElements[2], "@") {
			if tryBranch(targetRepo, pathElements[2][1:], pathElements[3:],
				string(GiteaRoot)+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
			) {
				tryUpstream()
				return
			}
			returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return
		} else {
			tryBranch(targetRepo, "", pathElements[2:],
				string(GiteaRoot)+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
			)
			tryUpstream()
			return
		}

	} else if bytes.HasSuffix(ctx.Request.Host(), MainDomainSuffix) {
		// Serve pages from subdomains of MainDomainSuffix

		pathElements := strings.Split(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/")
		targetOwner = string(bytes.TrimSuffix(ctx.Request.Host(), MainDomainSuffix))
		targetRepo = pathElements[0]
		targetPath = strings.Trim(strings.Join(pathElements[1:], "/"), "/")

		// Check if the first directory is a repo with the second directory as a branch
		// example.codeberg.page/myrepo/@main/index.html
		if len(pathElements) > 1 && strings.HasPrefix(pathElements[1], "@") {
			if targetRepo == "pages" {
				// example.codeberg.org/pages/@... redirects to example.codeberg.org/@...
				ctx.Redirect("/" + strings.Join(pathElements[1:], "/"), fasthttp.StatusTemporaryRedirect)
				return
			}

			if tryBranch(pathElements[0], pathElements[1][1:], pathElements[2:],
				"/"+pathElements[0]+"/%p",
			) {
				tryUpstream()
			} else {
				returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			}
			return
		}

		// Check if the first directory is a branch for the "pages" repo
		// example.codeberg.page/@main/index.html
		if strings.HasPrefix(pathElements[0], "@") {
			if tryBranch("pages", pathElements[0][1:], pathElements[1:], "/%p") {
				tryUpstream()
			} else {
				returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			}
			return
		}

		// Check if the first directory is a repo with a "pages" branch
		// example.codeberg.page/myrepo/index.html
		// example.codeberg.page/pages/... is not allowed here.
		if pathElements[0] != "pages" && tryBranch(pathElements[0], "pages", pathElements[1:], "") {
			tryUpstream()
			return
		}

		// Try to use the "pages" repo on its default branch
		// example.codeberg.page/index.html
		if tryBranch("pages", "", pathElements, "") {
			tryUpstream()
			return
		}

		// Couldn't find a valid repo/branch
		returnErrorPage(ctx, fasthttp.StatusFailedDependency)
		return
	} else {
		// Serve pages from external domains
		targetOwner, targetRepo, targetBranch = getTargetFromDNS(string(ctx.Request.Host()))
		if targetOwner == "" {
			ctx.Redirect(BrokenDNSPage, fasthttp.StatusTemporaryRedirect)
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
		if tryBranch(targetRepo, targetBranch, pathElements, canonicalLink) {
			canonicalDomain := checkCanonicalDomain(targetOwner, targetRepo, targetBranch)
			if canonicalDomain != string(ctx.Request.Host()) {
				// only redirect if
				targetOwner, _, _ = getTargetFromDNS(strings.SplitN(canonicalDomain, "/", 2)[0])
				if targetOwner != "" {
					ctx.Redirect("https://"+canonicalDomain+string(ctx.RequestURI()), fasthttp.StatusTemporaryRedirect)
					return
				} else {
					ctx.Redirect(BrokenDNSPage, fasthttp.StatusTemporaryRedirect)
					return
				}
			}

			tryUpstream()
			return
		} else {
			returnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return
		}
	}
}

// returnErrorPage sets the response status code and writes NotFoundPage to the response body, with "%status" replaced
// with the provided status code.
func returnErrorPage(ctx *fasthttp.RequestCtx, code int) {
	ctx.Response.SetStatusCode(code)
	ctx.Response.Header.SetContentType("text/html; charset=utf-8")
	ctx.Response.SetBody(bytes.ReplaceAll(NotFoundPage, []byte("%status"), []byte(strconv.Itoa(code)+" "+fasthttp.StatusMessage(code))))
}

// BranchCacheTimeout specifies the timeout for the branch timestamp cache.
var BranchCacheTimeout = 60*time.Second
// branchTimestampCache stores branch timestamps for faster cache checking
var branchTimestampCache = mcache.New()
type branchTimestamp struct {
	branch string
	timestamp time.Time
}

// FileCacheTimeout specifies the timeout for the file content cache - you might want to make this shorter
// than BranchCacheTimeout when running out of memory.
var FileCacheTimeout = 60*time.Second
// fileResponseCache stores responses from the Gitea server
var fileResponseCache = mcache.New()
type fileResponse struct {
	exists bool
	mimeType string
	body []byte
}

// getBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or an empty time.Time if the branch doesn't exist)
func getBranchTimestamp(owner, repo, branch string) *branchTimestamp {
	if result, ok := branchTimestampCache.Get(owner + "/" + repo + "/" + branch); ok {
		return result.(*branchTimestamp)
	}
	result := &branchTimestamp{}
	result.branch = branch
	if branch == "" {
		var body = make([]byte, 0)
		status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo, BranchCacheTimeout)
		if err != nil || status != 200 {
			return nil
		}
		result.branch = fastjson.GetString(body, "default_branch")
	}

	var body = make([]byte, 0)
	status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch, BranchCacheTimeout)
	if err != nil || status != 200 {
		return nil
	}

	result.timestamp, _ = time.Parse(time.RFC3339, fastjson.GetString(body, "commit", "timestamp"))
	_ = branchTimestampCache.Set(owner + "/" + repo + "/" + branch, result, 15 * time.Second)
	return result
}

var upstreamClient = fasthttp.Client{
	ReadTimeout: 10 * time.Second,
	MaxConnDuration: 60 * time.Second,
	MaxConnWaitTimeout: 1000 * time.Millisecond,
	MaxConnsPerHost: 128 * 16, // TODO: adjust bottlenecks for best performance with Gitea!
}

// upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func upstream(ctx *fasthttp.RequestCtx, targetOwner string, targetRepo string, targetBranch string, targetPath string, options *upstreamOptions) (final bool) {
	if options.ForbiddenMimeTypes == nil {
		options.ForbiddenMimeTypes = map[string]struct{}{}
	}

	// Check if the branch exists and when it was modified
	if options.BranchTimestamp == (time.Time{}) {
		branch := getBranchTimestamp(targetOwner, targetRepo, targetBranch)

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

	// Make a GET request to the upstream URL
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(string(GiteaRoot) + "/api/v1/repos/" + targetOwner + "/" + targetRepo + "/raw/" + targetBranch + "/" + targetPath)
	res := fasthttp.AcquireResponse()
	isCached := false
	var cachedResponse fileResponse
	var err error
	if cachedValue, ok := fileResponseCache.Get(string(req.RequestURI())); ok {
		isCached = true
		cachedResponse = cachedValue.(fileResponse)
	} else {
		err = upstreamClient.Do(req, res)
	}

	// Handle errors
	if (isCached && !cachedResponse.exists) || res.StatusCode() == fasthttp.StatusNotFound {
		if options.TryIndexPages {
			// copy the options struct & try if an index page exists
			optionsForIndexPages := *options
			optionsForIndexPages.TryIndexPages = false
			optionsForIndexPages.AppendTrailingSlash = true
			for _, indexPage := range IndexPages {
				if upstream(ctx, targetOwner, targetRepo, targetBranch, strings.TrimSuffix(targetPath, "/")+"/"+indexPage, &optionsForIndexPages) {
					return true
				}
			}
		}
		ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
		if !isCached {
			// Update cache if the request is fresh
			_ = fileResponseCache.Set(string(req.RequestURI()), fileResponse{
				exists: false,
			}, FileCacheTimeout)
		}
		return false
	}
	if !isCached && (err != nil || res.StatusCode() != fasthttp.StatusOK) {
		fmt.Printf("Couldn't fetch contents from \"%s\": %s (status code %d)\n", req.RequestURI(), err, res.StatusCode())
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}

	// Append trailing slash if missing (for index files)
	// options.AppendTrailingSlash is only true when looking for index pages
	if options.AppendTrailingSlash && !bytes.HasSuffix(ctx.Request.URI().Path(), []byte{'/'}) {
		ctx.Redirect(string(ctx.Request.URI().Path())+"/", fasthttp.StatusTemporaryRedirect)
		return true
	}

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

	// Write the response body to the original request
	var cacheBodyWriter bytes.Buffer
	if !isCached {
		err = res.BodyWriteTo(io.MultiWriter(ctx.Response.BodyWriter(), &cacheBodyWriter))
	} else {
		_, err = ctx.Write(cachedResponse.body)
	}
	if err != nil {
		fmt.Printf("Couldn't write body for \"%s\": %s\n", req.RequestURI(), err)
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}

	if !isCached {
		cachedResponse.exists = true
		cachedResponse.mimeType = mimeType
		cachedResponse.body = cacheBodyWriter.Bytes()
		_ = fileResponseCache.Set(string(req.RequestURI()), cachedResponse, FileCacheTimeout)
	}

	return true
}

// upstreamOptions provides various options for the upstream request.
type upstreamOptions struct {
	DefaultMimeType     string
	ForbiddenMimeTypes  map[string]struct{}
	TryIndexPages       bool
	AppendTrailingSlash bool
	BranchTimestamp     time.Time
}
