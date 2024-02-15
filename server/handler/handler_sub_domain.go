package handler

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

func handleSubDomain(log zerolog.Logger, ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix string,
	defaultPagesBranches []string,
	trimmedHost string,
	pathElements []string,
	canonicalDomainCache, redirectsCache cache.ICache,
) {
	// Serve pages from subdomains of MainDomainSuffix
	log.Debug().Msg("main domain suffix")

	targetOwner := strings.TrimSuffix(trimmedHost, mainDomainSuffix)
	targetRepo := pathElements[0]

	if targetOwner == "www" {
		// www.codeberg.page redirects to codeberg.page // TODO: rm hardcoded - use cname?
		ctx.Redirect("https://"+mainDomainSuffix[1:]+ctx.Path(), http.StatusPermanentRedirect)
		return
	}

	// Check if the first directory is a repo with the second directory as a branch
	// example.codeberg.page/myrepo/@main/index.html
	if len(pathElements) > 1 && strings.HasPrefix(pathElements[1], "@") {
		if targetRepo == defaultPagesRepo {
			// example.codeberg.org/pages/@... redirects to example.codeberg.org/@...
			ctx.Redirect("/"+strings.Join(pathElements[1:], "/"), http.StatusTemporaryRedirect)
			return
		}

		log.Debug().Msg("main domain preparations, now trying with specified repo & branch")
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			TryIndexPages: true,
			TargetOwner:   targetOwner,
			TargetRepo:    pathElements[0],
			TargetBranch:  pathElements[1][1:],
			TargetPath:    path.Join(pathElements[2:]...),
		}, true); works {
			log.Trace().Msg("tryUpstream: serve with specified repo and branch")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
		} else {
			html.ReturnErrorPage(
				ctx,
				formatSetBranchNotFoundMessage(pathElements[1][1:], targetOwner, pathElements[0]),
				http.StatusFailedDependency,
			)
		}
		return
	}

	// Check if the first directory is a branch for the defaultPagesRepo
	// example.codeberg.page/@main/index.html
	if strings.HasPrefix(pathElements[0], "@") {
		targetBranch := pathElements[0][1:]

		// if the default pages branch can be determined exactly, it does not need to be set
		if len(defaultPagesBranches) == 1 && slices.Contains(defaultPagesBranches, targetBranch) {
			// example.codeberg.org/@pages/... redirects to example.codeberg.org/...
			ctx.Redirect("/"+strings.Join(pathElements[1:], "/"), http.StatusTemporaryRedirect)
			return
		}

		log.Debug().Msg("main domain preparations, now trying with specified branch")
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			TryIndexPages: true,
			TargetOwner:   targetOwner,
			TargetRepo:    defaultPagesRepo,
			TargetBranch:  targetBranch,
			TargetPath:    path.Join(pathElements[1:]...),
		}, true); works {
			log.Trace().Msg("tryUpstream: serve default pages repo with specified branch")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
		} else {
			html.ReturnErrorPage(
				ctx,
				formatSetBranchNotFoundMessage(targetBranch, targetOwner, defaultPagesRepo),
				http.StatusFailedDependency,
			)
		}
		return
	}

	for _, defaultPagesBranch := range defaultPagesBranches {
		// Check if the first directory is a repo with a default pages branch
		// example.codeberg.page/myrepo/index.html
		// example.codeberg.page/{PAGES_BRANCHE}/... is not allowed here.
		log.Debug().Msg("main domain preparations, now trying with specified repo")
		if pathElements[0] != defaultPagesBranch {
			if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
				TryIndexPages: true,
				TargetOwner:   targetOwner,
				TargetRepo:    pathElements[0],
				TargetBranch:  defaultPagesBranch,
				TargetPath:    path.Join(pathElements[1:]...),
			}, false); works {
				log.Debug().Msg("tryBranch, now trying upstream 5")
				tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
				return
			}
		}

		// Try to use the defaultPagesRepo on an default pages branch
		// example.codeberg.page/index.html
		log.Debug().Msg("main domain preparations, now trying with default repo")
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			TryIndexPages: true,
			TargetOwner:   targetOwner,
			TargetRepo:    defaultPagesRepo,
			TargetBranch:  defaultPagesBranch,
			TargetPath:    path.Join(pathElements...),
		}, false); works {
			log.Debug().Msg("tryBranch, now trying upstream 6")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
			return
		}
	}

	// Try to use the defaultPagesRepo on its default branch
	// example.codeberg.page/index.html
	log.Debug().Msg("main domain preparations, now trying with default repo/branch")
	if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
		TryIndexPages: true,
		TargetOwner:   targetOwner,
		TargetRepo:    defaultPagesRepo,
		TargetPath:    path.Join(pathElements...),
	}, false); works {
		log.Debug().Msg("tryBranch, now trying upstream 6")
		tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
		return
	}

	// Couldn't find a valid repo/branch
	html.ReturnErrorPage(ctx,
		fmt.Sprintf("could not find a valid repository or branch for repository: <code>%s</code>", targetRepo),
		http.StatusNotFound)
}

func formatSetBranchNotFoundMessage(branch, owner, repo string) string {
	return fmt.Sprintf("explicitly set branch <code>%q</code> does not exist at <code>%s/%s</code>", branch, owner, repo)
}
