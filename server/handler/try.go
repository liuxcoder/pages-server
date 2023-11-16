package handler

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

// tryUpstream forwards the target request to the Gitea API, and shows an error page on failure.
func tryUpstream(ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix, trimmedHost string,
	options *upstream.Options,
	canonicalDomainCache cache.SetGetKey,
	redirectsCache cache.SetGetKey,
) {
	// check if a canonical domain exists on a request on MainDomain
	if strings.HasSuffix(trimmedHost, mainDomainSuffix) && !options.ServeRaw {
		canonicalDomain, _ := options.CheckCanonicalDomain(giteaClient, "", mainDomainSuffix, canonicalDomainCache)
		if !strings.HasSuffix(strings.SplitN(canonicalDomain, "/", 2)[0], mainDomainSuffix) {
			canonicalPath := ctx.Req.RequestURI
			if options.TargetRepo != defaultPagesRepo {
				path := strings.SplitN(canonicalPath, "/", 3)
				if len(path) >= 3 {
					canonicalPath = "/" + path[2]
				}
			}
			ctx.Redirect("https://"+canonicalDomain+canonicalPath, http.StatusTemporaryRedirect)
			return
		}
	}

	// Add host for debugging.
	options.Host = trimmedHost

	// Try to request the file from the Gitea API
	if !options.Upstream(ctx, giteaClient, redirectsCache) {
		html.ReturnErrorPage(ctx, "gitea client failed", ctx.StatusCode)
	}
}

// tryBranch checks if a branch exists and populates the target variables. If canonicalLink is non-empty,
// it will also disallow search indexing and add a Link header to the canonical URL.
func tryBranch(log zerolog.Logger, ctx *context.Context, giteaClient *gitea.Client,
	targetOptions *upstream.Options, canonicalLink bool,
) (*upstream.Options, bool) {
	if targetOptions.TargetOwner == "" || targetOptions.TargetRepo == "" {
		log.Debug().Msg("tryBranch: owner or repo is empty")
		return nil, false
	}

	// Replace "~" to "/" so we can access branch that contains slash character
	// Branch name cannot contain "~" so doing this is okay
	targetOptions.TargetBranch = strings.ReplaceAll(targetOptions.TargetBranch, "~", "/")

	// Check if the branch exists, otherwise treat it as a file path
	branchExist, _ := targetOptions.GetBranchTimestamp(giteaClient)
	if !branchExist {
		log.Debug().Msg("tryBranch: branch doesn't exist")
		return nil, false
	}

	if canonicalLink {
		// Hide from search machines & add canonical link
		ctx.RespWriter.Header().Set("X-Robots-Tag", "noarchive, noindex")
		ctx.RespWriter.Header().Set("Link", targetOptions.ContentWebLink(giteaClient)+"; rel=\"canonical\"")
	}

	log.Debug().Msg("tryBranch: true")
	return targetOptions, true
}
