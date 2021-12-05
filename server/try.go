package server

import (
	"bytes"
	"strings"

	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/upstream"
)

// tryUpstream forwards the target request to the Gitea API, and shows an error page on failure.
func tryUpstream(ctx *fasthttp.RequestCtx,
	mainDomainSuffix, trimmedHost []byte,

	targetOptions *upstream.Options,
	targetOwner, targetRepo, targetBranch, targetPath,

	giteaRoot, giteaAPIToken string,
	canonicalDomainCache, branchTimestampCache, fileResponseCache cache.SetGetKey) {
	// check if a canonical domain exists on a request on MainDomain
	if bytes.HasSuffix(trimmedHost, mainDomainSuffix) {
		canonicalDomain, _ := upstream.CheckCanonicalDomain(targetOwner, targetRepo, targetBranch, "", string(mainDomainSuffix), giteaRoot, giteaAPIToken, canonicalDomainCache)
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
	if !targetOptions.Upstream(ctx, targetOwner, targetRepo, targetBranch, targetPath, giteaRoot, giteaAPIToken, branchTimestampCache, fileResponseCache) {
		html.ReturnErrorPage(ctx, ctx.Response.StatusCode())
	}
}
