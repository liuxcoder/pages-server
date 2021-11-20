package main

import "bytes"

// GetHSTSHeader returns a HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain, or an empty
// string for custom domains.
func GetHSTSHeader(host []byte) string {
	if bytes.HasSuffix(host, MainDomainSuffix) || bytes.Equal(host, RawDomain) {
		return "max-age=63072000; includeSubdomains; preload"
	} else {
		return ""
	}
}

func TrimHostPort(host []byte) []byte {
	i := bytes.IndexByte(host, ':')
	if i >= 0 {
		return host[:i]
	}
	return host
}
