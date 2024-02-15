package handler

import (
	"net/http"
	"path"
	"strings"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/dns"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
	"github.com/rs/zerolog"
)

func handleCustomDomain(log zerolog.Logger, ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix string,
	trimmedHost string,
	pathElements []string,
	firstDefaultBranch string,
	dnsLookupCache, canonicalDomainCache, redirectsCache cache.ICache,
) {
	// Serve pages from custom domains
	targetOwner, targetRepo, targetBranch := dns.GetTargetFromDNS(trimmedHost, mainDomainSuffix, firstDefaultBranch, dnsLookupCache)
	if targetOwner == "" {
		html.ReturnErrorPage(ctx,
			"could not obtain repo owner from custom domain",
			http.StatusFailedDependency)
		return
	}

	pathParts := pathElements
	canonicalLink := false
	if strings.HasPrefix(pathElements[0], "@") {
		targetBranch = pathElements[0][1:]
		pathParts = pathElements[1:]
		canonicalLink = true
	}

	// Try to use the given repo on the given branch or the default branch
	log.Debug().Msg("custom domain preparations, now trying with details from DNS")
	if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
		TryIndexPages: true,
		TargetOwner:   targetOwner,
		TargetRepo:    targetRepo,
		TargetBranch:  targetBranch,
		TargetPath:    path.Join(pathParts...),
	}, canonicalLink); works {
		canonicalDomain, valid := targetOpt.CheckCanonicalDomain(giteaClient, trimmedHost, mainDomainSuffix, canonicalDomainCache)
		if !valid {
			html.ReturnErrorPage(ctx, "domain not specified in <code>.domains</code> file", http.StatusMisdirectedRequest)
			return
		} else if canonicalDomain != trimmedHost {
			// only redirect if the target is also a codeberg page!
			targetOwner, _, _ = dns.GetTargetFromDNS(strings.SplitN(canonicalDomain, "/", 2)[0], mainDomainSuffix, firstDefaultBranch, dnsLookupCache)
			if targetOwner != "" {
				ctx.Redirect("https://"+canonicalDomain+"/"+targetOpt.TargetPath, http.StatusTemporaryRedirect)
				return
			}

			html.ReturnErrorPage(ctx, "target is no codeberg page", http.StatusFailedDependency)
			return
		}

		log.Debug().Msg("tryBranch, now trying upstream 7")
		tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
		return
	}

	html.ReturnErrorPage(ctx, "could not find target for custom domain", http.StatusFailedDependency)
}
