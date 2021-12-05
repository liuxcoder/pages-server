package server

import (
	"net"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/upstream"
)

// DnsLookupCacheTimeout specifies the timeout for the DNS lookup cache.
var DnsLookupCacheTimeout = 15 * time.Minute

// getTargetFromDNS searches for CNAME or TXT entries on the request domain ending with MainDomainSuffix.
// If everything is fine, it returns the target data.
func getTargetFromDNS(domain, mainDomainSuffix string, dnsLookupCache cache.SetGetKey) (targetOwner, targetRepo, targetBranch string) {
	// Get CNAME or TXT
	var cname string
	var err error
	if cachedName, ok := dnsLookupCache.Get(domain); ok {
		cname = cachedName.(string)
	} else {
		cname, err = net.LookupCNAME(domain)
		cname = strings.TrimSuffix(cname, ".")
		if err != nil || !strings.HasSuffix(cname, mainDomainSuffix) {
			cname = ""
			// TODO: check if the A record matches!
			names, err := net.LookupTXT(domain)
			if err == nil {
				for _, name := range names {
					name = strings.TrimSuffix(name, ".")
					if strings.HasSuffix(name, mainDomainSuffix) {
						cname = name
						break
					}
				}
			}
		}
		_ = dnsLookupCache.Set(domain, cname, DnsLookupCacheTimeout)
	}
	if cname == "" {
		return
	}
	cnameParts := strings.Split(strings.TrimSuffix(cname, mainDomainSuffix), ".")
	targetOwner = cnameParts[len(cnameParts)-1]
	if len(cnameParts) > 1 {
		targetRepo = cnameParts[len(cnameParts)-2]
	}
	if len(cnameParts) > 2 {
		targetBranch = cnameParts[len(cnameParts)-3]
	}
	if targetRepo == "" {
		targetRepo = "pages"
	}
	if targetBranch == "" && targetRepo != "pages" {
		targetBranch = "pages"
	}
	// if targetBranch is still empty, the caller must find the default branch
	return
}

// CanonicalDomainCacheTimeout specifies the timeout for the canonical domain cache.
var CanonicalDomainCacheTimeout = 15 * time.Minute

// checkCanonicalDomain returns the canonical domain specified in the repo (using the file `.canonical-domain`).
func checkCanonicalDomain(targetOwner, targetRepo, targetBranch, actualDomain, mainDomainSuffix, giteaRoot, giteaApiToken string, canonicalDomainCache cache.SetGetKey) (canonicalDomain string, valid bool) {
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

		err := upstream.Client.Do(req, res)
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
		_ = canonicalDomainCache.Set(targetOwner+"/"+targetRepo+"/"+targetBranch, domains, CanonicalDomainCacheTimeout)
	}
	canonicalDomain = domains[0]
	return
}
