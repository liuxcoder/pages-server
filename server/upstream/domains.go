package upstream

import (
	"strings"

	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server/cache"
)

// CheckCanonicalDomain returns the canonical domain specified in the repo (using the file `.canonical-domain`).
func CheckCanonicalDomain(targetOwner, targetRepo, targetBranch, actualDomain, mainDomainSuffix, giteaRoot, giteaApiToken string, canonicalDomainCache cache.SetGetKey) (canonicalDomain string, valid bool) {
	domains := []string{}
	if cachedValue, ok := canonicalDomainCache.Get(targetOwner + "/" + targetRepo + "/" + targetBranch); ok {
		domains = cachedValue.([]string)
		for _, domain := range domains {
			if domain == actualDomain {
				valid = true
				break
			}
		}
	} else {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(giteaRoot + "/api/v1/repos/" + targetOwner + "/" + targetRepo + "/raw/" + targetBranch + "/.domains" + "?access_token=" + giteaApiToken)
		res := fasthttp.AcquireResponse()

		err := client.Do(req, res)
		if err == nil && res.StatusCode() == fasthttp.StatusOK {
			for _, domain := range strings.Split(string(res.Body()), "\n") {
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
	canonicalDomain = domains[0]
	return
}
