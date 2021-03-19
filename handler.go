package main

import (
	"bytes"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"mime"
	"net/url"
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

	if RawDomain != nil && bytes.Equal(ctx.Request.Host(), RawDomain) {
		// Serve raw content from RawDomain

		// TODO: add canonical link and "X-Robots-Tag: noarchive, noindex"

		targetOptions.TryIndexPages = false
		targetOptions.ForbiddenMimeTypes["text/html"] = struct{}{}
		targetOptions.DefaultMimeType = "text/plain; charset=utf-8"

		pathElements := strings.SplitN(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/", 4)
		if len(pathElements) < 2 {
			// https://{RawDomain}/{owner}/{repo}[/@{branch}]/{path} is required
			ctx.Redirect(RawInfoPage, fasthttp.StatusTemporaryRedirect)
			return
		}
		targetOwner = pathElements[0]
		targetRepo = pathElements[1]
		if len(pathElements) > 3 {
			targetPath = strings.Trim(pathElements[2]+"/"+pathElements[3], "/")
		} else if len(pathElements) > 2 {
			targetPath = pathElements[2]
		}

		// raw.codeberg.page/example/myrepo/@main/index.html
		if len(pathElements) > 3 && strings.HasPrefix(pathElements[2], "@") {
			branch, _ := url.PathUnescape(pathElements[2][1:])
			if branch == "" {
				branch = pathElements[2][1:]
			}
			// Check if the branch exists, otherwise treat it as a file path
			targetBranch, targetOptions.BranchTimestamp = getBranchTimestamp(targetOwner, targetRepo, branch)
			if targetOptions.BranchTimestamp != (time.Time{}) {
				targetPath = strings.Trim(pathElements[3], "/") // branch exists, use it
			} else {
				targetBranch = "" // branch doesn't exist, use default branch
			}
		}

	} else if bytes.HasSuffix(ctx.Request.Host(), MainDomainSuffix) {
		// Serve pages from subdomains of MainDomainSuffix

		// TODO: add @branch syntax with "X-Robots-Tag: noarchive, noindex"

		pathElements := strings.SplitN(string(bytes.Trim(ctx.Request.URI().Path(), "/")), "/", 2)
		targetOwner = string(bytes.TrimSuffix(ctx.Request.Host(), MainDomainSuffix))
		targetRepo = pathElements[0]
		if len(pathElements) > 1 {
			targetPath = strings.Trim(pathElements[1], "/")
		}

		// Check if the first directory is a repo with a "pages" branch
		targetBranch, targetOptions.BranchTimestamp = getBranchTimestamp(targetOwner, targetRepo, "pages")
		if targetOptions.BranchTimestamp == (time.Time{}) {
			targetRepo = "pages"
			targetBranch = ""
			targetPath = strings.Trim(pathElements[0]+"/"+targetPath, "/")
		}
	} else {
		// Serve pages from external domains

		targetOwner, targetRepo, targetBranch, targetPath = getTargetFromDNS(ctx)
		if targetOwner == "" {
			ctx.Redirect(BrokenDNSPage, fasthttp.StatusTemporaryRedirect)
			return
		}
	}

	// Check if a username can't exist because it's reserved (we'd risk to hit a Gitea route in that case)
	if _, ok := ReservedUsernames[targetOwner]; ok {
		returnErrorPage(ctx, fasthttp.StatusForbidden)
		return
	}

	// Check for blob path
	if strings.HasPrefix(targetPath, "blob/") {
		returnErrorPage(ctx, fasthttp.StatusForbidden)
		return
	}

	// Try to request the file from the Gitea API
	if !upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, targetOptions) {
		returnErrorPage(ctx, ctx.Response.StatusCode())
	}
}

// returnErrorPage sets the response status code and writes NotFoundPage to the response body, with "%status" replaced
// with the provided status code.
func returnErrorPage(ctx *fasthttp.RequestCtx, code int) {
	ctx.Response.SetStatusCode(code)
	ctx.Response.Header.SetContentType("text/html; charset=utf-8")
	ctx.Response.SetBody(bytes.ReplaceAll(NotFoundPage, []byte("%status"), []byte(strconv.Itoa(code) + " " + fasthttp.StatusMessage(code))))
}

// getBranchTimestamp finds the default branch (if branch is "") and returns the last modification time of the branch
// (or an empty time.Time if the branch doesn't exist)
// TODO: cache responses for ~15 minutes if a branch exists
func getBranchTimestamp(owner, repo, branch string) (branchWithFallback string, t time.Time) {
	branchWithFallback = branch
	if branch == "" {
		var body = make([]byte, 0)
		status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo, 10*time.Second)
		if err != nil || status != 200 {
			branchWithFallback = ""
			return
		}
		branch = fastjson.GetString(body, "default_branch")
		branchWithFallback = branch
	}

	var body = make([]byte, 0)
	status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch, 10*time.Second)
	if err != nil || status != 200 {
		branchWithFallback = ""
		return
	}

	t, _ = time.Parse(time.RFC3339, fastjson.GetString(body, "commit", "timestamp"))
	return
}

// upstream requests a file from the Gitea API at GiteaRoot and writes it to the request context.
func upstream(ctx *fasthttp.RequestCtx, targetOwner string, targetRepo string, targetBranch string, targetPath string, options *upstreamOptions) (success bool) {
	if options.ForbiddenMimeTypes == nil {
		options.ForbiddenMimeTypes = map[string]struct{}{}
	}

	// Check if the branch exists and when it was modified
	if options.BranchTimestamp == (time.Time{}) {
		targetBranch, options.BranchTimestamp = getBranchTimestamp(targetOwner, targetRepo, targetBranch)
	}

	// Handle repositories with no/broken pages setup
	if options.BranchTimestamp == (time.Time{}) || targetBranch == "" {
		ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
		ctx.Response.Header.SetContentType("text/html; charset=utf-8")
		ctx.Response.SetBody(bytes.ReplaceAll(NotFoundPage, []byte("%status"), []byte("pages not set up for this repo")))
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
	err := fasthttp.DoTimeout(req, res, 10*time.Second)

	// Handle errors
	if res.StatusCode() == fasthttp.StatusNotFound {
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
		return false
	}
	if err != nil || res.StatusCode() != fasthttp.StatusOK {
		fmt.Printf("Couldn't fetch contents from \"%s\": %s (status code %d)\n", req.RequestURI(), err, res.StatusCode())
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
	}

	// Append trailing slash if missing (for index files)
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

	// Write the response to the original request
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.SetLastModified(options.BranchTimestamp)
	err = res.BodyWriteTo(ctx.Response.BodyWriter())
	if err != nil {
		fmt.Printf("Couldn't write body for \"%s\": %s\n", req.RequestURI(), err)
		returnErrorPage(ctx, fasthttp.StatusInternalServerError)
		return true
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
