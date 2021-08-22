package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/OrlovEvgeny/go-mcache"
	"github.com/akrylysov/pogreb/fs"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/providers/dns"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/akrylysov/pogreb"
	"github.com/reugn/equalizer"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// tlsConfig contains the configuration for generating, serving and cleaning up Let's Encrypt certificates.
var tlsConfig = &tls.Config{
	// check DNS name & get certificate from Let's Encrypt
	GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if os.Getenv("ACME_ACCEPT_TERMS") != "true" {
			return FallbackCertificate(), nil
		}

		sni := strings.ToLower(strings.TrimSpace(info.ServerName))
		sniBytes := []byte(sni)
		if len(sni) < 1 {
			return nil, errors.New("missing sni")
		}

		if info.SupportedProtos != nil {
			for _, proto := range info.SupportedProtos {
				if proto == tlsalpn01.ACMETLS1Protocol {
					challenge, ok := challengeCache.Get(sni)
					if !ok {
						return nil, errors.New("no challenge for this domain")
					}
					cert, err := tlsalpn01.ChallengeCert(sni, challenge.(string))
					if err != nil {
						return nil, err
					}
					return cert, nil
				}
			}
		}

		targetOwner := ""
		if bytes.HasSuffix(sniBytes, MainDomainSuffix) {
			// deliver default certificate for the main domain (*.codeberg.page)
			sniBytes = MainDomainSuffix
			sni = string(sniBytes)
		} else {
			var targetRepo, targetBranch string
			targetOwner, targetRepo, targetBranch = getTargetFromDNS(sni)
			if targetOwner == "" {
				// DNS not set up, return a self-signed certificate to redirect to the docs
				return FallbackCertificate(), nil
			}

			// TODO: use .domains file to list all domains, to keep users from getting rate-limited
			_, _ = targetRepo, targetBranch
			/*canonicalDomain := checkCanonicalDomain(targetOwner, targetRepo, targetBranch)
			if sni != canonicalDomain {
				return FallbackCertificate(), nil
			}*/
		}

		// limit users to 1 certificate per week

		var cert, key []byte
		if tlsCertificate, ok := keyCache.Get(sni); ok {
			// we can use an existing certificate object
			return tlsCertificate.(*tls.Certificate), nil
		} else if ok, err := keyDatabase.Has(sniBytes); err != nil {
			// key database is not working
			panic(err)
		} else if ok {
			// parse certificate from database

			cert, err = keyDatabase.Get(sniBytes)
			if err != nil {
				// key database is not working
				panic(err)
			}
			key, err = keyDatabase.Get(append(sniBytes, '/', 'k', 'e', 'y'))
			if err != nil {
				// key database is not working or key doesn't exist
				panic(err)
			}
		} else {
			// request a new certificate

			if bytes.Equal(sniBytes, MainDomainSuffix) {
				return nil, errors.New("won't request certificate for main domain, something really bad has happened")
			}

			log.Printf("Requesting new certificate for %s", sni)
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				return nil, err
			}
			key = x509.MarshalPKCS1PrivateKey(privateKey)
			acmeClient, err := acmeClientFromPool(targetOwner)
			if err != nil {
				// TODO
			}
			res, err := acmeClient.Certificate.Obtain(certificate.ObtainRequest{
				Domains:        []string{sni},
				PrivateKey:     key,
				Bundle:         true,
				MustStaple:     true,
			})
			if err != nil {
				return nil, err
			}
			log.Printf("Obtained certificate for %s", sni)
			err = keyDatabase.Put(append(sniBytes, '/', 'k', 'e', 'y'), key)
			if err != nil {
				return nil, err
			}
			err = keyDatabase.Put(sniBytes, res.Certificate)
			if err != nil {
				_ = keyDatabase.Delete(append(sniBytes, '/', 'k', 'e', 'y'))
				return nil, err
			}
			cert = res.Certificate
		}
		tlsCertificate, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{
			Bytes: cert,
			Type: "CERTIFICATE",
		}), pem.EncodeToMemory(&pem.Block{
			Bytes: key,
			Type: "RSA PRIVATE KEY",
		}))
		if err != nil {
			panic(err)
		}

		err = keyCache.Set(sni, &tlsCertificate, 15 * time.Minute)
		if err != nil {
			panic(err)
		}
		return &tlsCertificate, nil
	},
	PreferServerCipherSuites: true,
	NextProtos: []string{
		tlsalpn01.ACMETLS1Protocol,
	},

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

var challengeCache = mcache.New()
var keyCache = mcache.New()
var keyDatabase *pogreb.DB

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

type AcmeAccount struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}
func (u *AcmeAccount) GetEmail() string {
	return u.Email
}
func (u AcmeAccount) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *AcmeAccount) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// rate-limit certificates per owner, based on LE Rate Limits:
// - 300 new orders per account per 3 hours
// - 20 requests per second
// - 10 Accounts per IP per 3 hours
var acmeClientPool []*lego.Client
var lastAcmeClient = 0
var acmeClientRequestLimit = equalizer.NewTokenBucket(10, time.Second) // LE allows 20 requests per second, but we want to give other applications a chancem so we want 10 here at most.
var acmeClientRegistrationLimit = equalizer.NewTokenBucket(5, time.Hour * 3) // LE allows 10 registrations in 3 hours per IP, we want at most 5 of them.
var acmeClientCertificateLimitPerRegistration = []*equalizer.TokenBucket{}
var acmeClientCertificateLimitPerUser = map[string]*equalizer.TokenBucket{}

type AcmeTLSChallengeProvider struct{}
var _ challenge.Provider = AcmeTLSChallengeProvider{}
func (a AcmeTLSChallengeProvider) Present(domain, _, keyAuth string) error {
	return challengeCache.Set(domain, keyAuth, 1*time.Hour)
}
func (a AcmeTLSChallengeProvider) CleanUp(domain, _, _ string) error {
	challengeCache.Remove(domain)
	return nil
}

func acmeClientFromPool(user string) (*lego.Client, error) {
	userLimit, ok := acmeClientCertificateLimitPerUser[user]
	if !ok {
		// Each Codeberg user can only add 10 new domains per day.
		userLimit = equalizer.NewTokenBucket(10, time.Hour * 24)
		acmeClientCertificateLimitPerUser[user] = userLimit

	}
	if !userLimit.Ask() {
		return nil, errors.New("rate limit exceeded: 10 certificates per user per 24 hours")
	}

	if len(acmeClientPool) < 1 {
		acmeClientPool = append(acmeClientPool, newAcmeClient())
		acmeClientCertificateLimitPerRegistration = append(acmeClientCertificateLimitPerRegistration, equalizer.NewTokenBucket(290, time.Hour * 3))
	}
	if !acmeClientCertificateLimitPerRegistration[(lastAcmeClient + 1) % len(acmeClientPool)].Ask() {

	}
	equalizer.NewTokenBucket(290, time.Hour * 3) // LE allows 300 certificates per account, to be sure to catch it earlier, we limit that to 290.

	// TODO: limit domains by file in repo
}

func newAcmeClient() *lego.Client {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	myUser := AcmeAccount{
		Email: "",
		key:   privateKey,
	}
	config := lego.NewConfig(&myUser)
	config.CADirURL = envOr("ACME_API", "https://acme-v02.api.letsencrypt.org/directory")
	config.Certificate.KeyType = certcrypto.RSA2048
	acmeClient, err := lego.NewClient(config)
	if err != nil {
		panic(err)
	}
	err = acmeClient.Challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{})
	if err != nil {
		panic(err)
	}

	// accept terms
	if os.Getenv("ACME_ACCEPT_TERMS") == "true" {
		reg, err := acmeClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: os.Getenv("ACME_ACCEPT_TERMS") == "true"})
		if err != nil {
			panic(err)
		}
		myUser.Registration = reg
	} else {
		log.Printf("Warning: not using ACME certificates as ACME_ACCEPT_TERMS is false!")
	}
	return acmeClient
}

func init() {
	FallbackCertificate()

	var err error
	keyDatabase, err = pogreb.Open("key-database.pogreb", &pogreb.Options{
		BackgroundSyncInterval:       30 * time.Second,
		BackgroundCompactionInterval: 6 * time.Hour,
		FileSystem:                   fs.OSMMap,
	})
	if err != nil {
		panic(err)
	}

	// generate certificate for main domain
	if os.Getenv("ACME_ACCEPT_TERMS") != "true" || os.Getenv("DNS_PROVIDER") == "" {
		err = keyCache.Set(string(MainDomainSuffix), FallbackCertificate(), mcache.TTL_FOREVER)
		if err != nil {
			panic(err)
		}
	} else {
		log.Printf("Requesting new certificate for *%s", MainDomainSuffix)
		dnsAcmeClient, err := lego.NewClient(config)
		if err != nil {
			panic(err)
		}
		provider, err := dns.NewDNSChallengeProviderByName(os.Getenv("DNS_PROVIDER"))
		if err != nil {
			panic(err)
		}
		err = dnsAcmeClient.Challenge.SetDNS01Provider(provider)
		if err != nil {
			panic(err)
		}
		mainPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		mainKey := x509.MarshalPKCS1PrivateKey(mainPrivateKey)
		res, err := dnsAcmeClient.Certificate.Obtain(certificate.ObtainRequest{
			Domains:    []string{"*" + string(MainDomainSuffix), string(MainDomainSuffix[1:])},
			PrivateKey: mainKey,
			Bundle:     true,
			MustStaple: true,
		})
		if err != nil {
			panic(err)
		}
		err = keyDatabase.Put(append(MainDomainSuffix, '/', 'k', 'e', 'y'), mainKey)
		if err != nil {
			panic(err)
		}
		err = keyDatabase.Put(MainDomainSuffix, res.Certificate)
		if err != nil {
			_ = keyDatabase.Delete(append(MainDomainSuffix, '/', 'k', 'e', 'y'))
			panic(err)
		}
	}
}

// TODO: renew & revoke
