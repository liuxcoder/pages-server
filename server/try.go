package server

import (
	"net/http"
	"strings"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

// tryUpstream forwards the target request to the Gitea API, and shows an error page on failure.
func tryUpstream(ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix, trimmedHost string,

	targetOptions *upstream.Options,
	targetOwner, targetRepo, targetBranch, targetPath string,

	canonicalDomainCache cache.SetGetKey,
) {
	// check if a canonical domain exists on a request on MainDomain
	if strings.HasSuffix(trimmedHost, mainDomainSuffix) {
		canonicalDomain, _ := upstream.CheckCanonicalDomain(giteaClient, targetOwner, targetRepo, targetBranch, "", string(mainDomainSuffix), canonicalDomainCache)
		if !strings.HasSuffix(strings.SplitN(canonicalDomain, "/", 2)[0], string(mainDomainSuffix)) {
			canonicalPath := ctx.Req.RequestURI
			if targetRepo != "pages" {
				path := strings.SplitN(canonicalPath, "/", 3)
				if len(path) >= 3 {
					canonicalPath = "/" + path[2]
				}
			}
			ctx.Redirect("https://"+canonicalDomain+canonicalPath, http.StatusTemporaryRedirect)
			return
		}
	}

	targetOptions.TargetOwner = targetOwner
	targetOptions.TargetRepo = targetRepo
	targetOptions.TargetBranch = targetBranch
	targetOptions.TargetPath = targetPath
	targetOptions.Host = string(trimmedHost)

	// Try to request the file from the Gitea API
	if !targetOptions.Upstream(ctx, giteaClient) {
		html.ReturnErrorPage(ctx, "", ctx.StatusCode)
	}
}
