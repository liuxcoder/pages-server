package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

var fallbackCertKey, _ = rsa.GenerateKey(rand.Reader, 1024)
var fallbackCertSpecification = &x509.Certificate{
	Subject: pkix.Name{
		CommonName: strings.TrimPrefix(string(MainDomainSuffix), "."),
	},
	SerialNumber: big.NewInt(0),
	NotBefore: time.Now(),
	NotAfter: time.Now().AddDate(100, 0, 0),
}
var fallbackCertBytes, _ = x509.CreateCertificate(
	rand.Reader,
	fallbackCertSpecification,
	fallbackCertSpecification,
	fallbackCertKey.Public(),
	fallbackCertKey,
)
var fallbackCert, _ = tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{
	Bytes: fallbackCertBytes,
	Type: "CERTIFICATE",
}), pem.EncodeToMemory(&pem.Block{
	Bytes: x509.MarshalPKCS1PrivateKey(fallbackCertKey),
	Type: "RSA PRIVATE KEY",
}))

// tlsConfig contains the configuration for generating, serving and cleaning up Let's Encrypt certificates.
var tlsConfig = &tls.Config{
	GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// TODO: check DNS name & get certificate from Let's Encrypt
		return &fallbackCert, nil
	},
	PreferServerCipherSuites: true,
	// TODO: optimize cipher suites, minimum TLS version, etc.
}

// TODO: HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain
