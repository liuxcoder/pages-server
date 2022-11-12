package handler

import (
	"strings"
)

// getHSTSHeader returns a HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain, or an empty
// string for custom domains.
func getHSTSHeader(host, mainDomainSuffix, rawDomain string) string {
	if strings.HasSuffix(host, mainDomainSuffix) || strings.EqualFold(host, rawDomain) {
		return "max-age=63072000; includeSubdomains; preload"
	} else {
		return ""
	}
}
