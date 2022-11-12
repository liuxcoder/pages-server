package handler

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/rs/zerolog"

	"codeberg.org/codeberg/pages/html"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

const defaultPagesRepo = "pages"

func handleSubDomain(log zerolog.Logger, ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix string,
	trimmedHost string,
	pathElements []string,
	canonicalDomainCache cache.SetGetKey,
) {
	// Serve pages from subdomains of MainDomainSuffix
	log.Debug().Msg("main domain suffix")

	targetOwner := strings.TrimSuffix(trimmedHost, mainDomainSuffix)
	targetRepo := pathElements[0]

	if targetOwner == "www" {
		// www.codeberg.page redirects to codeberg.page // TODO: rm hardcoded - use cname?
		ctx.Redirect("https://"+string(mainDomainSuffix[1:])+string(ctx.Path()), http.StatusPermanentRedirect)
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
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache)
		} else {
			html.ReturnErrorPage(ctx,
				fmt.Sprintf("explizite set branch %q do not exist at '%s/%s'", targetOpt.TargetBranch, targetOpt.TargetOwner, targetOpt.TargetRepo),
				http.StatusFailedDependency)
		}
		return
	}

	// Check if the first directory is a branch for the defaultPagesRepo
	// example.codeberg.page/@main/index.html
	if strings.HasPrefix(pathElements[0], "@") {
		log.Debug().Msg("main domain preparations, now trying with specified branch")
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			TryIndexPages: true,
			TargetOwner:   targetOwner,
			TargetRepo:    defaultPagesRepo,
			TargetBranch:  pathElements[0][1:],
			TargetPath:    path.Join(pathElements[1:]...),
		}, true); works {
			log.Trace().Msg("tryUpstream: serve default pages repo with specified branch")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache)
		} else {
			html.ReturnErrorPage(ctx,
				fmt.Sprintf("explizite set branch %q do not exist at '%s/%s'", targetOpt.TargetBranch, targetOpt.TargetOwner, targetOpt.TargetRepo),
				http.StatusFailedDependency)
		}
		return
	}

	// Check if the first directory is a repo with a defaultPagesRepo branch
	// example.codeberg.page/myrepo/index.html
	// example.codeberg.page/pages/... is not allowed here.
	log.Debug().Msg("main domain preparations, now trying with specified repo")
	if pathElements[0] != defaultPagesRepo {
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			TryIndexPages: true,
			TargetOwner:   targetOwner,
			TargetRepo:    pathElements[0],
			TargetBranch:  defaultPagesRepo,
			TargetPath:    path.Join(pathElements[1:]...),
		}, false); works {
			log.Debug().Msg("tryBranch, now trying upstream 5")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache)
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
		tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache)
		return
	}

	// Couldn't find a valid repo/branch
	html.ReturnErrorPage(ctx,
		fmt.Sprintf("couldn't find a valid repo[%s]", targetRepo),
		http.StatusFailedDependency)
}
