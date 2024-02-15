package upstream

import (
	"errors"
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
func (o *Options) CheckCanonicalDomain(giteaClient *gitea.Client, actualDomain, mainDomainSuffix string, canonicalDomainCache cache.ICache) (domain string, valid bool) {
	// Check if this request is cached.
	if cachedValue, ok := canonicalDomainCache.Get(o.TargetOwner + "/" + o.TargetRepo + "/" + o.TargetBranch); ok {
		domains := cachedValue.([]string)
		for _, domain := range domains {
			if domain == actualDomain {
				valid = true
				break
			}
		}
		return domains[0], valid
	}

	body, err := giteaClient.GiteaRawContent(o.TargetOwner, o.TargetRepo, o.TargetBranch, canonicalDomainConfig)
	if err != nil && !errors.Is(err, gitea.ErrorNotFound) {
		log.Error().Err(err).Msgf("could not read %s of %s/%s", canonicalDomainConfig, o.TargetOwner, o.TargetRepo)
	}

	var domains []string
	for _, domain := range strings.Split(string(body), "\n") {
		domain = strings.ToLower(domain)
		domain = strings.TrimSpace(domain)
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimPrefix(domain, "https://")
		if domain != "" && !strings.HasPrefix(domain, "#") && !strings.ContainsAny(domain, "\t /") && strings.ContainsRune(domain, '.') {
			domains = append(domains, domain)
		}
		if domain == actualDomain {
			valid = true
		}
	}

	// Add [owner].[pages-domain] as valid domain.
	domains = append(domains, o.TargetOwner+mainDomainSuffix)
	if domains[len(domains)-1] == actualDomain {
		valid = true
	}

	// If the target repository isn't called pages, add `/[repository]` to the
	// previous valid domain.
	if o.TargetRepo != "" && o.TargetRepo != "pages" {
		domains[len(domains)-1] += "/" + o.TargetRepo
	}

	// Add result to cache.
	_ = canonicalDomainCache.Set(o.TargetOwner+"/"+o.TargetRepo+"/"+o.TargetBranch, domains, canonicalDomainCacheTimeout)

	// Return the first domain from the list and return if any of the domains
	// matched the requested domain.
	return domains[0], valid
}
