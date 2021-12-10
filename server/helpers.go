package server

import (
	"bytes"
)

// GetHSTSHeader returns a HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain, or an empty
// string for custom domains.
func GetHSTSHeader(host, mainDomainSuffix, rawDomain []byte) string {
	if bytes.HasSuffix(host, mainDomainSuffix) || bytes.Equal(host, rawDomain) {
		return "max-age=63072000; includeSubdomains; preload"
	} else {
		return ""
	}
}
