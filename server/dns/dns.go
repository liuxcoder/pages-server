package dns

import (
	"net"
	"strings"
	"time"

	"codeberg.org/codeberg/pages/server/cache"
)

// lookupCacheTimeout specifies the timeout for the DNS lookup cache.
var lookupCacheTimeout = 15 * time.Minute

var defaultPagesRepo = "pages"

// GetTargetFromDNS searches for CNAME or TXT entries on the request domain ending with MainDomainSuffix.
// If everything is fine, it returns the target data.
func GetTargetFromDNS(domain, mainDomainSuffix, firstDefaultBranch string, dnsLookupCache cache.ICache) (targetOwner, targetRepo, targetBranch string) {
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
					name = strings.TrimSuffix(strings.TrimSpace(name), ".")
					if strings.HasSuffix(name, mainDomainSuffix) {
						cname = name
						break
					}
				}
			}
		}
		_ = dnsLookupCache.Set(domain, cname, lookupCacheTimeout)
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
		targetRepo = defaultPagesRepo
	}
	if targetBranch == "" && targetRepo != defaultPagesRepo {
		targetBranch = firstDefaultBranch
	}
	// if targetBranch is still empty, the caller must find the default branch
	return
}
