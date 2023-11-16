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

func handleRaw(log zerolog.Logger, ctx *context.Context, giteaClient *gitea.Client,
	mainDomainSuffix, rawInfoPage string,
	trimmedHost string,
	pathElements []string,
	canonicalDomainCache, redirectsCache cache.SetGetKey,
) {
	// Serve raw content from RawDomain
	log.Debug().Msg("raw domain")

	if len(pathElements) < 2 {
		// https://{RawDomain}/{owner}/{repo}[/@{branch}]/{path} is required
		ctx.Redirect(rawInfoPage, http.StatusTemporaryRedirect)
		return
	}

	// raw.codeberg.org/example/myrepo/@main/index.html
	if len(pathElements) > 2 && strings.HasPrefix(pathElements[2], "@") {
		log.Debug().Msg("raw domain preparations, now trying with specified branch")
		if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
			ServeRaw:     true,
			TargetOwner:  pathElements[0],
			TargetRepo:   pathElements[1],
			TargetBranch: pathElements[2][1:],
			TargetPath:   path.Join(pathElements[3:]...),
		}, true); works {
			log.Trace().Msg("tryUpstream: serve raw domain with specified branch")
			tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
			return
		}
		log.Debug().Msg("missing branch info")
		html.ReturnErrorPage(ctx, "missing branch info", http.StatusFailedDependency)
		return
	}

	log.Debug().Msg("raw domain preparations, now trying with default branch")
	if targetOpt, works := tryBranch(log, ctx, giteaClient, &upstream.Options{
		TryIndexPages: false,
		ServeRaw:      true,
		TargetOwner:   pathElements[0],
		TargetRepo:    pathElements[1],
		TargetPath:    path.Join(pathElements[2:]...),
	}, true); works {
		log.Trace().Msg("tryUpstream: serve raw domain with default branch")
		tryUpstream(ctx, giteaClient, mainDomainSuffix, trimmedHost, targetOpt, canonicalDomainCache, redirectsCache)
	} else {
		html.ReturnErrorPage(ctx,
			fmt.Sprintf("raw domain could not find repo <code>%s/%s</code> or repo is empty", targetOpt.TargetOwner, targetOpt.TargetRepo),
			http.StatusNotFound)
	}
}
