package upstream

import (
	"strconv"
	"strings"
	"time"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/gitea"
	"github.com/rs/zerolog/log"
)

type Redirect struct {
	From       string
	To         string
	StatusCode int
}

// redirectsCacheTimeout specifies the timeout for the redirects cache.
var redirectsCacheTimeout = 10 * time.Minute

const redirectsConfig = "_redirects"

// getRedirects returns redirects specified in the _redirects file.
func (o *Options) getRedirects(giteaClient *gitea.Client, redirectsCache cache.SetGetKey) []Redirect {
	var redirects []Redirect
	cacheKey := o.TargetOwner + "/" + o.TargetRepo + "/" + o.TargetBranch

	// Check for cached redirects
	if cachedValue, ok := redirectsCache.Get(cacheKey); ok {
		redirects = cachedValue.([]Redirect)
	} else {
		// Get _redirects file and parse
		body, err := giteaClient.GiteaRawContent(o.TargetOwner, o.TargetRepo, o.TargetBranch, redirectsConfig)
		if err == nil {
			for _, line := range strings.Split(string(body), "\n") {
				redirectArr := strings.Fields(line)

				// Ignore comments and invalid lines
				if strings.HasPrefix(line, "#") || len(redirectArr) < 2 {
					continue
				}

				// Get redirect status code
				statusCode := 301
				if len(redirectArr) == 3 {
					statusCode, err = strconv.Atoi(redirectArr[2])
					if err != nil {
						log.Info().Err(err).Msgf("could not read %s of %s/%s", redirectsConfig, o.TargetOwner, o.TargetRepo)
					}
				}

				redirects = append(redirects, Redirect{
					From:       redirectArr[0],
					To:         redirectArr[1],
					StatusCode: statusCode,
				})
			}
		}
		_ = redirectsCache.Set(cacheKey, redirects, redirectsCacheTimeout)
	}
	return redirects
}

func (o *Options) matchRedirects(ctx *context.Context, giteaClient *gitea.Client, redirects []Redirect, redirectsCache cache.SetGetKey) (final bool) {
	if len(redirects) > 0 {
		for _, redirect := range redirects {
			reqUrl := ctx.Req.RequestURI
			// remove repo and branch from request url
			reqUrl = strings.TrimPrefix(reqUrl, "/"+o.TargetRepo)
			reqUrl = strings.TrimPrefix(reqUrl, "/@"+o.TargetBranch)

			// check if from url matches request url
			if strings.TrimSuffix(redirect.From, "/") == strings.TrimSuffix(reqUrl, "/") {
				// do rewrite if status code is 200
				if redirect.StatusCode == 200 {
					o.TargetPath = redirect.To
					o.Upstream(ctx, giteaClient, redirectsCache)
					return true
				} else {
					ctx.Redirect(redirect.To, redirect.StatusCode)
					return true
				}
			}

			// handle wildcard redirects
			trimmedFromUrl := strings.TrimSuffix(redirect.From, "/*")
			if strings.HasSuffix(redirect.From, "/*") && strings.HasPrefix(reqUrl, trimmedFromUrl) {
				if strings.Contains(redirect.To, ":splat") {
					splatUrl := strings.ReplaceAll(redirect.To, ":splat", strings.TrimPrefix(reqUrl, trimmedFromUrl))
					// do rewrite if status code is 200
					if redirect.StatusCode == 200 {
						o.TargetPath = splatUrl
						o.Upstream(ctx, giteaClient, redirectsCache)
						return true
					} else {
						ctx.Redirect(splatUrl, redirect.StatusCode)
						return true
					}
				} else {
					// do rewrite if status code is 200
					if redirect.StatusCode == 200 {
						o.TargetPath = redirect.To
						o.Upstream(ctx, giteaClient, redirectsCache)
						return true
					} else {
						ctx.Redirect(redirect.To, redirect.StatusCode)
						return true
					}
				}
			}
		}
	}

	return false
}
