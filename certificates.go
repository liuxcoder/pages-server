package main

import (
	"bytes"
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

// tlsConfig contains the configuration for generating, serving and cleaning up Let's Encrypt certificates.
var tlsConfig = &tls.Config{
	GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// TODO: check DNS name & get certificate from Let's Encrypt
		return FallbackCertificate(), nil
	},
	PreferServerCipherSuites: true,

	// generated 2021-07-13, Mozilla Guideline v5.6, Go 1.14.4, intermediate configuration
	// https://ssl-config.mozilla.org/#server=go&version=1.14.4&config=intermediate&guideline=5.6
	MinVersion: tls.VersionTLS12,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	},
}

// GetHSTSHeader returns a HSTS header with includeSubdomains & preload for MainDomainSuffix and RawDomain, or an empty
// string for custom domains.
func GetHSTSHeader(host []byte) string {
	if bytes.HasSuffix(host, MainDomainSuffix) || bytes.Equal(host, RawDomain) {
		return "max-age=63072000; includeSubdomains; preload"
	} else {
		return ""
	}
}

var fallbackCertificate *tls.Certificate
// FallbackCertificate generates a new self-signed TLS certificate on demand.
func FallbackCertificate() *tls.Certificate {
	if fallbackCertificate != nil {
		return fallbackCertificate
	}

	fallbackSerial, err := rand.Int(rand.Reader, (&big.Int{}).Lsh(big.NewInt(1), 159))
	if err != nil {
		panic(err)
	}

	fallbackCertKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}

	fallbackCertSpecification := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: strings.TrimPrefix(string(MainDomainSuffix), "."),
		},
		SerialNumber: fallbackSerial,
		NotBefore: time.Now(),
		NotAfter: time.Now().AddDate(100, 0, 0),
	}

	fallbackCertBytes, err := x509.CreateCertificate(
		rand.Reader,
		fallbackCertSpecification,
		fallbackCertSpecification,
		fallbackCertKey.Public(),
		fallbackCertKey,
	)
	if err != nil {
		panic(err)
	}

	fallbackCert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{
		Bytes: fallbackCertBytes,
		Type: "CERTIFICATE",
	}), pem.EncodeToMemory(&pem.Block{
		Bytes: x509.MarshalPKCS1PrivateKey(fallbackCertKey),
		Type: "RSA PRIVATE KEY",
	}))
	if err != nil {
		panic(err)
	}

	fallbackCertificate = &fallbackCert
	return fallbackCertificate
}
