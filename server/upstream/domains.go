package upstream

import (
	"strings"

	"codeberg.org/codeberg/pages/server/cache"
)

// CheckCanonicalDomain returns the canonical domain specified in the repo (using the `.domains` file).
func CheckCanonicalDomain(targetOwner, targetRepo, targetBranch, actualDomain, mainDomainSuffix, giteaRoot, giteaAPIToken string, canonicalDomainCache cache.SetGetKey) (string, bool) {
	return checkCanonicalDomain(targetOwner, targetRepo, targetBranch, actualDomain, mainDomainSuffix, giteaRoot, giteaAPIToken, canonicalDomainCache)
}

func checkCanonicalDomain(targetOwner, targetRepo, targetBranch, actualDomain, mainDomainSuffix, giteaRoot, giteaAPIToken string, canonicalDomainCache cache.SetGetKey) (string, bool) {
	var (
		domains []string
		valid   bool
	)
	if cachedValue, ok := canonicalDomainCache.Get(targetOwner + "/" + targetRepo + "/" + targetBranch); ok {
		domains = cachedValue.([]string)
		for _, domain := range domains {
			if domain == actualDomain {
				valid = true
				break
			}
		}
	} else {
		body, err := giteaRawContent(targetOwner, targetRepo, targetBranch, giteaRoot, giteaAPIToken, canonicalDomainConfig)
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
		}
		domains = append(domains, targetOwner+mainDomainSuffix)
		if domains[len(domains)-1] == actualDomain {
			valid = true
		}
		if targetRepo != "" && targetRepo != "pages" {
			domains[len(domains)-1] += "/" + targetRepo
		}
		_ = canonicalDomainCache.Set(targetOwner+"/"+targetRepo+"/"+targetBranch, domains, canonicalDomainCacheTimeout)
	}
	return domains[0], valid
}
