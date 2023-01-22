package upstream

import (
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
)

// canonicalDomainCacheTimeout specifies the timeout for the canonical domain cache.
var canonicalDomainCacheTimeout = 15 * time.Minute

const canonicalDomainConfig = ".domains"

// CheckCanonicalDomain returns the canonical domain specified in the repo (using the `.domains` file).
func (o *Options) CheckCanonicalDomain(giteaClient *gitea.Client, actualDomain, mainDomainSuffix string, canonicalDomainCache cache.SetGetKey) (string, bool) {
	var (
		domains []string
		valid   bool
	)
	if cachedValue, ok := canonicalDomainCache.Get(o.TargetOwner + "/" + o.TargetRepo + "/" + o.TargetBranch); ok {
		domains = cachedValue.([]string)
		for _, domain := range domains {
			if domain == actualDomain {
				valid = true
				break
			}
		}
	} else {
		body, err := giteaClient.GiteaRawContent(o.TargetOwner, o.TargetRepo, o.TargetBranch, canonicalDomainConfig)
		if err == nil {
			for _, domain := range strings.Split(string(body), "\n") {
				domain = strings.ToLower(domain)
				domain = strings.TrimSpace(domain)
				domain = strings.TrimPrefix(domain, "http://")
				domain = strings.TrimPrefix(domain, "https://")
				if len(domain) > 0 && !strings.HasPrefix(domain, "#") && !strings.ContainsAny(domain, "\t /") && strings.ContainsRune(domain, '.') {
					domains = append(domains, domain)
				}
				if domain == actualDomain {
					valid = true
				}
			}
		} else {
			if err != gitea.ErrorNotFound {
				log.Error().Err(err).Msgf("could not read %s of %s/%s", canonicalDomainConfig, o.TargetOwner, o.TargetRepo)
			} else {
				log.Info().Err(err).Msgf("could not read %s of %s/%s", canonicalDomainConfig, o.TargetOwner, o.TargetRepo)
			}
		}
		domains = append(domains, o.TargetOwner+mainDomainSuffix)
		if domains[len(domains)-1] == actualDomain {
			valid = true
		}
		if o.TargetRepo != "" && o.TargetRepo != "pages" {
			domains[len(domains)-1] += "/" + o.TargetRepo
		}
		_ = canonicalDomainCache.Set(o.TargetOwner+"/"+o.TargetRepo+"/"+o.TargetBranch, domains, canonicalDomainCacheTimeout)
	}
	return domains[0], valid
}
