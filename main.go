// Package main is the new Codeberg Pages server, a solution for serving static pages from Gitea repositories.
//
// Mapping custom domains is not static anymore, but can be done with DNS:
//
// 1) add a "codeberg-pages-domains.txt" text file to your repository, containing the allowed domains
//
// 2) add a CNAME entry to your domain, pointing to "[[{branch}.]{repo}.]{owner}.codeberg.page" (repo defaults to
// "pages", "branch" defaults to the default branch if "repo" is "pages", or to "pages" if "repo" is something else):
//      www.example.org. IN CNAME main.pages.example.codeberg.page.
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
	"mime"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
var MainDomainSuffix = []byte(".codeberg.page")

// GiteaRoot specifies the root URL of the Gitea instance, without a trailing slash.
var GiteaRoot = []byte("https://codeberg.org")

//go:embed 404.html
var NotFoundPage []byte

// BrokenDNSPage will be shown (with a redirect) when trying to access a domain for which no DNS CNAME record exists.
var BrokenDNSPage = "https://docs.codeberg.org/codeberg-pages/custom-domains/"

// RawDomain specifies the domain from which raw repository content shall be served in the following format:
// https://{RawDomain}/{owner}/{repo}[/{branch|tag|commit}/{version}]/{filepath...}
// (set to []byte(nil) to disable raw content hosting)
var RawDomain = []byte("raw.codeberg.page")

// RawInfoPage will be shown (with a redirect) when trying to access RawDomain directly (or without owner/repo/path).
var RawInfoPage = "https://docs.codeberg.org/codeberg-pages/raw-content/"

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

// ReservedUsernames specifies the usernames that are reserved by Gitea and thus may not be used as owner names.
// The contents are taken from https://github.com/go-gitea/gitea/blob/master/models/user.go#L783; reserved names with
// dots are removed as they are forbidden for Codeberg Pages anyways.
var ReservedUsernames = createLookupMapFromWords(`
	admin api assets attachments avatars captcha commits debug error explore ghost help install issues less login metrics milestones new notifications org plugins pulls raw repo search stars template user
	
`)

// main sets up and starts the web server.
func main() {
	// Make sure MainDomain has a trailing dot, and GiteaRoot has no trailing slash
	if !bytes.HasPrefix(MainDomainSuffix, []byte{'.'}) {
		MainDomainSuffix = append([]byte{'.'}, MainDomainSuffix...)
	}
	GiteaRoot = bytes.TrimSuffix(GiteaRoot, []byte{'/'})

	// Use HOST and PORT environment variables to determine listening address
	address := fmt.Sprintf("%s:%s", envOr("HOST", "[::]"), envOr("PORT", "80"))
	fmt.Printf("Listening on http://%s\n", address)

	// Enable compression by wrapping the handler() method with the compression function provided by FastHTTP
	compressedHandler := fasthttp.CompressHandlerBrotliLevel(handler, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)

	// Setup listener and TLS
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Printf("Couldn't create listener: %s\n", err)
		os.Exit(1)
	}
	if envOr("LETS_ENCRYPT", "0") == "1" {
		tls.NewListener(listener, &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// TODO: check DNS name & get certificate from Let's Encrypt
				return nil, fmt.Errorf("NYI")
			},
			PreferServerCipherSuites: true,
			// TODO: optimize cipher suites, minimum TLS version, etc.
		})
	}

	// Start the web server
	err = (&fasthttp.Server{
		Handler:                      compressedHandler,
		DisablePreParseMultipartForm: false,
		MaxRequestBodySize:           0,
		NoDefaultServerHeader:        true,
		NoDefaultDate:                true,
		ReadTimeout:                  10 * time.Second,
	}).Serve(listener)
	if err != nil {
		fmt.Printf("Couldn't start server: %s\n", err)
		os.Exit(1)
	}
}

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

	if upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, targetOptions) {
		return
	}

	returnErrorPage(ctx, ctx.Response.StatusCode())
}

func getTargetFromDNS(ctx *fasthttp.RequestCtx) (targetOwner, targetRepo, targetBranch, targetPath string) {
	// TODO: read CNAME record for host and "www.{host}" to get those values
	// TODO: check codeberg-pages-domains.txt
	return
}

// returnErrorPage sets the response status code and writes NotFoundPage to the response body, with "%status" replaced
// with the provided status code.
func returnErrorPage(ctx *fasthttp.RequestCtx, code int) {
	ctx.Response.SetStatusCode(code)
	ctx.Response.SetBody(bytes.ReplaceAll(NotFoundPage, []byte("%status"), []byte(strconv.Itoa(code))))
}

func getBranchTimestamp(owner, repo, branch string) (branchWithFallback string, t time.Time) {
	branchWithFallback = branch
	if branch == "" {
		var body = make([]byte, 0)
		status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo, 10*time.Second)
		if err != nil || status != 200 {
			return
		}
		branch = fastjson.GetString(body, "default_branch")
		branchWithFallback = branch
	}

	var body = make([]byte, 0)
	status, body, err := fasthttp.GetTimeout(body, string(GiteaRoot)+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch, 10*time.Second)
	if err != nil || status != 200 {
		return
	}

	t, _ = time.Parse(time.RFC3339, fastjson.GetString(body, "commit", "timestamp"))
	return
}

// upstream requests an URL from GiteaRoot and writes it to the request context; if "final" is set, it also returns a
// 404 error if the page couldn't be loaded.
func upstream(ctx *fasthttp.RequestCtx, targetOwner string, targetRepo string, targetBranch string, targetPath string, options *upstreamOptions) (success bool) {
	fmt.Printf("Trying: %s/%s/%s/%s\n", targetOwner, targetRepo, targetBranch, targetPath)
	if options.ForbiddenMimeTypes == nil {
		options.ForbiddenMimeTypes = map[string]struct{}{}
	}

	// Check if the branch exists and when it was modified
	if options.BranchTimestamp == (time.Time{}) {
		targetBranch, options.BranchTimestamp = getBranchTimestamp(targetOwner, targetRepo, targetBranch)
		if options.BranchTimestamp == (time.Time{}) {
			ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
			return false
		}
	}

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

// envOr reads an environment variable and returns a default value if it's empty.
func envOr(env string, or string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return or
}

func createLookupMapFromWords(input string) map[string]struct{} {
	var res = map[string]struct{}{}
	input = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(input)
	for _, word := range strings.Split(input, " ") {
		if len(word) > 0 {
			res[word] = struct{}{}
		}
	}
	return res
}
