package main

import (
	"crypto/tls"
	"fmt"
)

// tlsConfig contains the configuration for generating, serving and cleaning up Let's Encrypt certificates.
var tlsConfig = &tls.Config{
	GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// TODO: check DNS name & get certificate from Let's Encrypt
		return nil, fmt.Errorf("NYI")
	},
	PreferServerCipherSuites: true,
	// TODO: optimize cipher suites, minimum TLS version, etc.
}

// TODO: HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain
