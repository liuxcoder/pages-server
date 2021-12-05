package dns

import (
	"net"
	"strings"

	"codeberg.org/codeberg/pages/server/cache"
)

// GetTargetFromDNS searches for CNAME or TXT entries on the request domain ending with MainDomainSuffix.
// If everything is fine, it returns the target data.
func GetTargetFromDNS(domain, mainDomainSuffix string, dnsLookupCache cache.SetGetKey) (targetOwner, targetRepo, targetBranch string) {
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
