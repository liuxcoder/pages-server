package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"github.com/OrlovEvgeny/go-mcache"
	"github.com/akrylysov/pogreb/fs"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/resolver"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/providers/dns"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
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
		if bytes.HasSuffix(sniBytes, MainDomainSuffix) || bytes.Equal(sniBytes, MainDomainSuffix[1:]) {
			// deliver default certificate for the main domain (*.codeberg.page)
			sniBytes = MainDomainSuffix
			sni = string(sniBytes)
		} else {
			var targetRepo, targetBranch string
			targetOwner, targetRepo, targetBranch = getTargetFromDNS(sni)
			if targetOwner == "" {
				// DNS not set up, return main certificate to redirect to the docs
				sniBytes = MainDomainSuffix
				sni = string(sniBytes)
			} else {
				_, _ = targetRepo, targetBranch
				_, valid := checkCanonicalDomain(targetOwner, targetRepo, targetBranch, sni)
				if !valid {
					sniBytes = MainDomainSuffix
					sni = string(sniBytes)
				}
			}
		}

		if tlsCertificate, ok := keyCache.Get(sni); ok {
			// we can use an existing certificate object
			return tlsCertificate.(*tls.Certificate), nil
		}

		var tlsCertificate tls.Certificate
		var err error
		var ok bool
		if tlsCertificate, ok = retrieveCertFromDB(sniBytes); ok {
			tlsCertificate.Leaf, err = x509.ParseCertificate(tlsCertificate.Certificate[0])
			if err != nil {
				panic(err)
			}

			if !bytes.Equal(sniBytes, MainDomainSuffix) && !tlsCertificate.Leaf.NotAfter.After(time.Now().Add(-7 * 24 * time.Hour)) {
				go (func() {
					tlsCertificate, err = obtainCert(acmeClient, []string{sni})
					if err != nil {
						log.Printf("Couldn't renew certificate.")
					}
				})()
			}
		}
		if tlsCertificate.Certificate == nil || !tlsCertificate.Leaf.NotAfter.After(time.Now().Add(-5 * time.Minute)) {
			// request a new certificate
			if bytes.Equal(sniBytes, MainDomainSuffix) {
				return nil, errors.New("won't request certificate for main domain, something really bad has happened")
			}

			err = CheckUserLimit(targetOwner)
			if err != nil {
				return nil, err
			}

			tlsCertificate, err = obtainCert(acmeClient, []string{sni})
			if err != nil {
				return nil, err
			}
		}

		err = keyCache.Set(sni, &tlsCertificate, 15 * time.Minute)
		if err != nil {
			panic(err)
		}
		return &tlsCertificate, nil
	},
	PreferServerCipherSuites: true,
	NextProtos: []string{
		"http/1.1",
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

var challengeCache = mcache.New()
var keyCache = mcache.New()
var keyDatabase *pogreb.DB

func CheckUserLimit(user string) (error) {
	userLimit, ok := acmeClientCertificateLimitPerUser[user]
	if !ok {
		// Each Codeberg user can only add 10 new domains per day.
		userLimit = equalizer.NewTokenBucket(10, time.Hour * 24)
		acmeClientCertificateLimitPerUser[user] = userLimit
	}
	if !userLimit.Ask() {
		return errors.New("rate limit exceeded: 10 certificates per user per 24 hours")
	}
	return nil
}

var myAcmeAccount AcmeAccount
var myAcmeConfig *lego.Config

type AcmeAccount struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey `json:"-"`
	KeyPEM       string `json:"Key"`
}
func (u *AcmeAccount) GetEmail() string {
	return u.Email
}
func (u AcmeAccount) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *AcmeAccount) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

func newAcmeClient(configureChallenge func(*resolver.SolverManager) error) *lego.Client {
	acmeClient, err := lego.NewClient(myAcmeConfig)
	if err != nil {
		panic(err)
	}
	err = configureChallenge(acmeClient.Challenge)
	if err != nil {
		panic(err)
	}
	return acmeClient
}

var acmeClient, mainDomainAcmeClient *lego.Client
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

func retrieveCertFromDB(sni []byte) (tls.Certificate, bool) {
	// parse certificate from database
	certPem, err := keyDatabase.Get(sni)
	if err != nil {
		// key database is not working
		panic(err)
	}
	if certPem == nil {
		return tls.Certificate{}, false
	}
	keyPem, err := keyDatabase.Get(append(sni, '/', 'k', 'e', 'y'))
	if err != nil {
		// key database is not working or key doesn't exist
		panic(err)
	}

	tlsCertificate, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		panic(err)
	}
	return tlsCertificate, true
}

var obtainLocks = sync.Map{}
func obtainCert(acmeClient *lego.Client, domains []string) (tls.Certificate, error) {
	name := strings.TrimPrefix(domains[0], "*")
	if os.Getenv("DNS_PROVIDER") == "" && len(domains[0]) > 0 && domains[0][0] == '*' {
		domains = domains[1:]
	}

	// lock to avoid simultaneous requests
	_, working := obtainLocks.LoadOrStore(name, struct{}{})
	if working {
		for working {
			time.Sleep(100 * time.Millisecond)
			_, working = obtainLocks.Load(name)
		}
		cert, ok := retrieveCertFromDB([]byte(name))
		if !ok {
			return tls.Certificate{}, errors.New("certificate failed in synchronous request")
		}
		return cert, nil
	}
	defer obtainLocks.Delete(name)

	log.Printf("Requesting new certificate for %v", domains)
	res, err := acmeClient.Certificate.Obtain(certificate.ObtainRequest{
		Domains:    domains,
		Bundle:     true,
		MustStaple: true,
	})
	if err != nil {
		log.Printf("Couldn't obtain certificate for %v: %s", domains, err)
		return tls.Certificate{}, err
	}
	log.Printf("Obtained certificate for %v", domains)

	err = keyDatabase.Put([]byte(name + "/key"), res.PrivateKey)
	if err != nil {
		obtainLocks.Delete(name)
		panic(err)
	}
	err = keyDatabase.Put([]byte(name), res.Certificate)
	if err != nil {
		_ = keyDatabase.Delete([]byte(name + "/key"))
		obtainLocks.Delete(name)
		panic(err)
	}

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tlsCertificate, nil
}

func setupCertificates() {
	var err error
	keyDatabase, err = pogreb.Open("key-database.pogreb", &pogreb.Options{
		BackgroundSyncInterval:       30 * time.Second,
		BackgroundCompactionInterval: 6 * time.Hour,
		FileSystem:                   fs.OSMMap,
	})
	if err != nil {
		panic(err)
	}

	if os.Getenv("ACME_ACCEPT_TERMS") != "true" || (os.Getenv("DNS_PROVIDER") == "" && os.Getenv("ACME_API") != "https://acme.mock.directory") {
		panic(errors.New("you must set ACME_ACCEPT_TERMS and DNS_PROVIDER, unless ACME_API is set to https://acme.mock.directory"))
	}

	if account, err := ioutil.ReadFile("acme-account.json"); err == nil {
		err = json.Unmarshal(account, &myAcmeAccount)
		if err != nil {
			panic(err)
		}
		myAcmeAccount.Key, err = certcrypto.ParsePEMPrivateKey([]byte(myAcmeAccount.KeyPEM))
		if err != nil {
			panic(err)
		}
		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = envOr("ACME_API", "https://acme.zerossl.com/v2/DV90")
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
		newAcmeClient(func(manager *resolver.SolverManager) error { return nil })
	} else if os.IsNotExist(err) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}
		myAcmeAccount = AcmeAccount{
			Email: envOr("ACME_EMAIL", "noreply@example.email"),
			Key:   privateKey,
			KeyPEM: string(certcrypto.PEMEncode(privateKey)),
		}
		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = envOr("ACME_API", "https://acme.zerossl.com/v2/DV90")
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
		tempClient := newAcmeClient(func(manager *resolver.SolverManager) error { return nil })

		// accept terms & log in to EAB
		if os.Getenv("ACME_EAB_KID") == "" || os.Getenv("ACME_EAB_HMAC") == "" {
			reg, err := tempClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: os.Getenv("ACME_ACCEPT_TERMS") == "true"})
			if err != nil {
				panic(err)
			}
			myAcmeAccount.Registration = reg
		} else {
			reg, err := tempClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
				TermsOfServiceAgreed: os.Getenv("ACME_ACCEPT_TERMS") == "true",
				Kid: os.Getenv("ACME_EAB_KID"),
				HmacEncoded: os.Getenv("ACME_EAB_HMAC"),
			})
			if err != nil {
				panic(err)
			}
			myAcmeAccount.Registration = reg
		}

		acmeAccountJson, err := json.Marshal(myAcmeAccount)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile("acme-account.json", acmeAccountJson, 0600)
		if err != nil {
			panic(err)
		}
	} else {
		panic(err)
	}

	acmeClient = newAcmeClient(func(challenge *resolver.SolverManager) error {
		return challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{})
	})
	mainDomainAcmeClient = newAcmeClient(func(challenge *resolver.SolverManager) error {
		if os.Getenv("DNS_PROVIDER") == "" {
			// using mock server, don't use wildcard certs
			return challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{})
		}
		provider, err := dns.NewDNSChallengeProviderByName(os.Getenv("DNS_PROVIDER"))
		if err != nil {
			return err
		}
		return challenge.SetDNS01Provider(provider)
	})

	go (func() {
		for {
			err := keyDatabase.Sync()
			if err != nil {
				log.Printf("Syncinc key database failed: %s", err)
			}
			time.Sleep(5 * time.Minute)
		}
	})()
	go (func() {
		for {
			// clean up expired certs
			keySuffix := []byte("/key")
			now := time.Now()
			expiredCertCount := 0
			key, value, err := keyDatabase.Items().Next()
			for err == nil {
				if !bytes.HasSuffix(key, keySuffix) {
					tlsCertificates, err := certcrypto.ParsePEMBundle(value)
					if err != nil || !tlsCertificates[0].NotAfter.After(now) {
						err := keyDatabase.Delete(key)
						if err != nil {
							log.Printf("Deleting expired certificate for %s failed: %s", string(key), err)
						} else {
							expiredCertCount++
						}
					}
				}
				key, value, err = keyDatabase.Items().Next()
			}
			log.Printf("Removed %d expired certificates from the database", expiredCertCount)

			// compact the database
			result, err := keyDatabase.Compact()
			if err != nil {
				log.Printf("Compacting key database failed: %s", err)
			} else {
				log.Printf("Compacted key database (%+v)", result)
			}

			// update main cert
			certPem, err := keyDatabase.Get(MainDomainSuffix)
			if err != nil {
				// key database is not working
				panic(err)
			}
			tlsCertificates, err := certcrypto.ParsePEMBundle(certPem)
			if err != nil || !tlsCertificates[0].NotAfter.After(time.Now().Add(-48 * time.Hour)) {
				_, _ = obtainCert(mainDomainAcmeClient, []string{"*" + string(MainDomainSuffix), string(MainDomainSuffix[1:])})
			}

			time.Sleep(12 * time.Hour)
		}
	})()
}
