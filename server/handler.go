package server

import (
	"bytes"
	"strings"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/upstream"
	"codeberg.org/codeberg/pages/server/utils"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// Handler handles a single HTTP request to the web server.
func Handler(mainDomainSuffix, rawDomain []byte, giteaRoot, rawInfoPage, giteaApiToken string, blacklistedPaths, allowedCorsDomains [][]byte) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		log := log.With().Str("Handler", string(ctx.Request.Header.RequestURI())).Logger()

		ctx.Response.Header.Set("Server", "Codeberg Pages")

		// Force new default from specification (since November 2020) - see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy#strict-origin-when-cross-origin
		ctx.Response.Header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enable browser caching for up to 10 minutes
		ctx.Response.Header.Set("Cache-Control", "public, max-age=600")

		trimmedHost := utils.TrimHostPort(ctx.Request.Host())

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
				html.ReturnErrorPage(ctx, fasthttp.StatusForbidden)
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
		var targetOptions = &upstream.Options{
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
			branchTimestampResult := upstream.GetBranchTimestamp(targetOwner, repo, branch, giteaRoot, giteaApiToken)
			if branchTimestampResult == nil {
				// branch doesn't exist
				return false
			}

			// Branch exists, use it
			targetRepo = repo
			targetPath = strings.Trim(strings.Join(path, "/"), "/")
			targetBranch = branchTimestampResult.Branch

			targetOptions.BranchTimestamp = branchTimestampResult.Timestamp

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
				canonicalDomain, _ := checkCanonicalDomain(targetOwner, targetRepo, targetBranch, "", string(mainDomainSuffix), giteaRoot, giteaApiToken)
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
			if !upstream.Upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, giteaRoot, giteaApiToken, targetOptions) {
				html.ReturnErrorPage(ctx, ctx.Response.StatusCode())
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
					giteaRoot+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
				) {
					log.Debug().Msg("tryBranch, now trying upstream")
					tryUpstream()
					return
				}
				log.Debug().Msg("missing branch")
				html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
				return
			} else {
				log.Debug().Msg("raw domain preparations, now trying with default branch")
				tryBranch(targetRepo, "", pathElements[2:],
					giteaRoot+"/"+targetOwner+"/"+targetRepo+"/src/branch/%b/%p",
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
					html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
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
					html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
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
			html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
			return
		} else {
			trimmedHostStr := string(trimmedHost)

			// Serve pages from external domains
			targetOwner, targetRepo, targetBranch = getTargetFromDNS(trimmedHostStr, string(mainDomainSuffix))
			if targetOwner == "" {
				html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
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
				canonicalDomain, valid := checkCanonicalDomain(targetOwner, targetRepo, targetBranch, trimmedHostStr, string(mainDomainSuffix), giteaRoot, giteaApiToken)
				if !valid {
					html.ReturnErrorPage(ctx, fasthttp.StatusMisdirectedRequest)
					return
				} else if canonicalDomain != trimmedHostStr {
					// only redirect if the target is also a codeberg page!
					targetOwner, _, _ = getTargetFromDNS(strings.SplitN(canonicalDomain, "/", 2)[0], string(mainDomainSuffix))
					if targetOwner != "" {
						ctx.Redirect("https://"+canonicalDomain+string(ctx.RequestURI()), fasthttp.StatusTemporaryRedirect)
						return
					} else {
						html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
						return
					}
				}

				log.Debug().Msg("tryBranch, now trying upstream")
				tryUpstream()
				return
			} else {
				html.ReturnErrorPage(ctx, fasthttp.StatusFailedDependency)
				return
			}
		}
	}
}
