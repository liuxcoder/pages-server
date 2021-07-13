package main

import (
	"github.com/OrlovEvgeny/go-mcache"
	"github.com/valyala/fasthttp"
	"net"
	"strings"
	"time"
)

// DnsLookupCacheTimeout specifies the timeout for the DNS lookup cache.
var DnsLookupCacheTimeout = 15*time.Minute
// dnsLookupCache stores DNS lookups for custom domains
var dnsLookupCache = mcache.New()

// getTargetFromDNS searches for CNAME or TXT entries on the request domain ending with MainDomainSuffix.
// If everything is fine, it returns the target data.
func getTargetFromDNS(domain string) (targetOwner, targetRepo, targetBranch string) {
	// Get CNAME or TXT
	var cname string
	var err error
	if cachedName, ok := dnsLookupCache.Get(domain); ok {
		cname = cachedName.(string)
	} else {
		cname, err = net.LookupCNAME(domain)
		cname = strings.TrimSuffix(cname, ".")
		if err != nil || !strings.HasSuffix(cname, string(MainDomainSuffix)) {
			cname = ""
			// TODO: check if the A record matches!
			names, err := net.LookupTXT(domain)
			if err == nil {
				for _, name := range names {
					name = strings.TrimSuffix(name, ".")
					if strings.HasSuffix(name, string(MainDomainSuffix)) {
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
	cnameParts := strings.Split(strings.TrimSuffix(cname, string(MainDomainSuffix)), ".")
	targetOwner = cnameParts[len(cnameParts)-1]
	if len(cnameParts) > 1 {
		targetRepo = cnameParts[len(cnameParts)-1]
	}
	if len(cnameParts) > 2 {
		targetBranch = cnameParts[len(cnameParts)-2]
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
var CanonicalDomainCacheTimeout = 15*time.Minute
// canonicalDomainCache stores canonical domains
var canonicalDomainCache = mcache.New()

// checkCanonicalDomain returns the canonical domain specified in the repo (using the file `.canonical-domain`).
func checkCanonicalDomain(targetOwner, targetRepo, targetBranch string) (canonicalDomain string) {
	// Check if the canonical domain matches
	if cachedValue, ok := canonicalDomainCache.Get(targetOwner + "/" + targetRepo + "/" + targetBranch); ok {
		canonicalDomain = cachedValue.(string)
	} else {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(string(GiteaRoot) + "/api/v1/repos/" + targetOwner + "/" + targetRepo + "/raw/" + targetBranch + "/.domains")
		res := fasthttp.AcquireResponse()

		err := upstreamClient.Do(req, res)
		if err == nil && res.StatusCode() == fasthttp.StatusOK {
			canonicalDomain = strings.TrimSpace(string(res.Body()))
			if strings.Contains(canonicalDomain, "/") {
				canonicalDomain = ""
			}
		}
		if canonicalDomain == "" {
			canonicalDomain = targetOwner + string(MainDomainSuffix)
			if targetRepo != "" && targetRepo != "pages" {
				canonicalDomain += "/" + targetRepo
			}
		}
		_ = canonicalDomainCache.Set(targetOwner + "/" + targetRepo + "/" + targetBranch, canonicalDomain, CanonicalDomainCacheTimeout)
	}
	return
}
