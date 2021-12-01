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
	"encoding/gob"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/OrlovEvgeny/go-mcache"
	"github.com/akrylysov/pogreb/fs"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/providers/dns"
	"io/ioutil"
	"log"
	"math/big"
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
		if tlsCertificate, ok = retrieveCertFromDB(sniBytes); !ok {
			// request a new certificate
			if bytes.Equal(sniBytes, MainDomainSuffix) {
				return nil, errors.New("won't request certificate for main domain, something really bad has happened")
			}

			tlsCertificate, err = obtainCert(acmeClient, []string{sni}, nil, targetOwner)
			if err != nil {
				return nil, err
			}
		}

		err = keyCache.Set(sni, &tlsCertificate, 15*time.Minute)
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

var keyCache = mcache.New()
var keyDatabase, keyDatabaseErr = pogreb.Open("key-database.pogreb", &pogreb.Options{
	BackgroundSyncInterval:       30 * time.Second,
	BackgroundCompactionInterval: 6 * time.Hour,
	FileSystem:                   fs.OSMMap,
})

func CheckUserLimit(user string) error {
	userLimit, ok := acmeClientCertificateLimitPerUser[user]
	if !ok {
		// Each Codeberg user can only add 10 new domains per day.
		userLimit = equalizer.NewTokenBucket(10, time.Hour*24)
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
	KeyPEM       string            `json:"Key"`
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

var acmeClient, mainDomainAcmeClient *lego.Client
var acmeClientCertificateLimitPerUser = map[string]*equalizer.TokenBucket{}

// rate limit is 300 / 3 hours, we want 200 / 2 hours but to refill more often, so that's 25 new domains every 15 minutes
// TODO: when this is used a lot, we probably have to think of a somewhat better solution?
var acmeClientOrderLimit = equalizer.NewTokenBucket(25, 15*time.Minute)

// rate limit is 20 / second, we want 5 / second (especially as one cert takes at least two requests)
var acmeClientRequestLimit = equalizer.NewTokenBucket(5, 1*time.Second)

var challengeCache = mcache.New()

type AcmeTLSChallengeProvider struct{}

var _ challenge.Provider = AcmeTLSChallengeProvider{}

func (a AcmeTLSChallengeProvider) Present(domain, _, keyAuth string) error {
	return challengeCache.Set(domain, keyAuth, 1*time.Hour)
}
func (a AcmeTLSChallengeProvider) CleanUp(domain, _, _ string) error {
	challengeCache.Remove(domain)
	return nil
}

type AcmeHTTPChallengeProvider struct{}

var _ challenge.Provider = AcmeHTTPChallengeProvider{}

func (a AcmeHTTPChallengeProvider) Present(domain, token, keyAuth string) error {
	return challengeCache.Set(domain+"/"+token, keyAuth, 1*time.Hour)
}
func (a AcmeHTTPChallengeProvider) CleanUp(domain, token, _ string) error {
	challengeCache.Remove(domain + "/" + token)
	return nil
}

func retrieveCertFromDB(sni []byte) (tls.Certificate, bool) {
	// parse certificate from database
	resBytes, err := keyDatabase.Get(sni)
	if err != nil {
		// key database is not working
		panic(err)
	}
	if resBytes == nil {
		return tls.Certificate{}, false
	}

	resGob := bytes.NewBuffer(resBytes)
	resDec := gob.NewDecoder(resGob)
	res := &certificate.Resource{}
	err = resDec.Decode(res)
	if err != nil {
		panic(err)
	}

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		panic(err)
	}

	if !bytes.Equal(sni, MainDomainSuffix) {
		tlsCertificate.Leaf, err = x509.ParseCertificate(tlsCertificate.Certificate[0])
		if err != nil {
			panic(err)
		}

		// renew certificates 7 days before they expire
		if !tlsCertificate.Leaf.NotAfter.After(time.Now().Add(-7 * 24 * time.Hour)) {
			go (func() {
				tlsCertificate, err = obtainCert(acmeClient, []string{string(sni)}, res, "")
				if err != nil {
					log.Printf("Couldn't renew certificate for %s: %s", sni, err)
				}
			})()
		}
	}

	return tlsCertificate, true
}

var obtainLocks = sync.Map{}

func obtainCert(acmeClient *lego.Client, domains []string, renew *certificate.Resource, user string) (tls.Certificate, error) {
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

	if acmeClient == nil {
		return mockCert(domains[0], "ACME client uninitialized. This is a server error, please report!"), nil
	}

	// request actual cert
	var res *certificate.Resource
	var err error
	if renew != nil && renew.CertURL != "" {
		if os.Getenv("ACME_USE_RATE_LIMITS") != "false" {
			acmeClientRequestLimit.Take()
		}
		log.Printf("Renewing certificate for %v", domains)
		res, err = acmeClient.Certificate.Renew(*renew, true, false, "")
		if err != nil {
			log.Printf("Couldn't renew certificate for %v, trying to request a new one: %s", domains, err)
			res = nil
		}
	}
	if res == nil {
		if user != "" {
			if err := CheckUserLimit(user); err != nil {
				return tls.Certificate{}, err
			}
		}

		if os.Getenv("ACME_USE_RATE_LIMITS") != "false" {
			acmeClientOrderLimit.Take()
			acmeClientRequestLimit.Take()
		}
		log.Printf("Requesting new certificate for %v", domains)
		res, err = acmeClient.Certificate.Obtain(certificate.ObtainRequest{
			Domains:    domains,
			Bundle:     true,
			MustStaple: false,
		})
	}
	if err != nil {
		log.Printf("Couldn't obtain certificate for %v: %s", domains, err)
		return mockCert(domains[0], err.Error()), err
	}
	log.Printf("Obtained certificate for %v", domains)

	var resGob bytes.Buffer
	resEnc := gob.NewEncoder(&resGob)
	err = resEnc.Encode(res)
	if err != nil {
		panic(err)
	}
	err = keyDatabase.Put([]byte(name), resGob.Bytes())

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tlsCertificate, nil
}

func mockCert(domain string, msg string) tls.Certificate {
	key, err := certcrypto.GeneratePrivateKey(certcrypto.RSA2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
			Organization: []string{"Codeberg Pages Error Certificate (couldn't obtain ACME certificate)"},
			OrganizationalUnit: []string{
				"Will not try again for 6 hours to avoid hitting rate limits for your domain.",
				"Check https://docs.codeberg.org/codeberg-pages/troubleshooting/ for troubleshooting tips, and feel " +
					"free to create an issue at https://codeberg.org/Codeberg/pages-server if you can't solve it.\n",
				"Error message: " + msg,
			},
		},

		// certificates younger than 7 days are renewed, so this enforces the cert to not be renewed for a 6 hours
		NotAfter:  time.Now().Add(time.Hour * 24 * 7 + time.Hour * 6),
		NotBefore: time.Now(),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.(*rsa.PrivateKey).PublicKey,
		key,
	)
	if err != nil {
		panic(err)
	}

	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{
		Bytes: certBytes,
		Type: "CERTIFICATE",
	})
	if err != nil {
		panic(err)
	}
	outBytes := out.Bytes()
	res := &certificate.Resource{
		PrivateKey: certcrypto.PEMEncode(key),
		Certificate: outBytes,
		IssuerCertificate: outBytes,
		Domain: domain,
	}
	var resGob bytes.Buffer
	resEnc := gob.NewEncoder(&resGob)
	err = resEnc.Encode(res)
	if err != nil {
		panic(err)
	}
	databaseName := domain
	if domain == "*" + string(MainDomainSuffix) || domain == string(MainDomainSuffix[1:]) {
		databaseName = string(MainDomainSuffix)
	}
	err = keyDatabase.Put([]byte(databaseName), resGob.Bytes())
	if err != nil {
		panic(err)
	}

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		panic(err)
	}
	return tlsCertificate
}

func setupCertificates() {
	if keyDatabaseErr != nil {
		panic(keyDatabaseErr)
	}

	if os.Getenv("ACME_ACCEPT_TERMS") != "true" || (os.Getenv("DNS_PROVIDER") == "" && os.Getenv("ACME_API") != "https://acme.mock.directory") {
		panic(errors.New("you must set ACME_ACCEPT_TERMS and DNS_PROVIDER, unless ACME_API is set to https://acme.mock.directory"))
	}

	// getting main cert before ACME account so that we can panic here on database failure without hitting rate limits
	mainCertBytes, err := keyDatabase.Get(MainDomainSuffix)
	if err != nil {
		// key database is not working
		panic(err)
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
		myAcmeConfig.CADirURL = envOr("ACME_API", "https://acme-v02.api.letsencrypt.org/directory")
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
		_, err := lego.NewClient(myAcmeConfig)
		if err != nil {
			log.Printf("[ERROR] Can't create ACME client, continuing with mock certs only: %s", err)
		}
	} else if os.IsNotExist(err) {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}
		myAcmeAccount = AcmeAccount{
			Email:  envOr("ACME_EMAIL", "noreply@example.email"),
			Key:    privateKey,
			KeyPEM: string(certcrypto.PEMEncode(privateKey)),
		}
		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = envOr("ACME_API", "https://acme-v02.api.letsencrypt.org/directory")
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
		tempClient, err := lego.NewClient(myAcmeConfig)
		if err != nil {
			log.Printf("[ERROR] Can't create ACME client, continuing with mock certs only: %s", err)
		} else {
			// accept terms & log in to EAB
			if os.Getenv("ACME_EAB_KID") == "" || os.Getenv("ACME_EAB_HMAC") == "" {
				reg, err := tempClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: os.Getenv("ACME_ACCEPT_TERMS") == "true"})
				if err != nil {
					log.Printf("[ERROR] Can't register ACME account, continuing with mock certs only: %s", err)
				} else {
					myAcmeAccount.Registration = reg
				}
			} else {
				reg, err := tempClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
					TermsOfServiceAgreed: os.Getenv("ACME_ACCEPT_TERMS") == "true",
					Kid:                  os.Getenv("ACME_EAB_KID"),
					HmacEncoded:          os.Getenv("ACME_EAB_HMAC"),
				})
				if err != nil {
					log.Printf("[ERROR] Can't register ACME account, continuing with mock certs only: %s", err)
				} else {
					myAcmeAccount.Registration = reg
				}
			}

			if myAcmeAccount.Registration != nil {
				acmeAccountJson, err := json.Marshal(myAcmeAccount)
				if err != nil {
					log.Printf("[FAIL] Error during json.Marshal(myAcmeAccount), waiting for manual restart to avoid rate limits: %s", err)
					select {}
				}
				err = ioutil.WriteFile("acme-account.json", acmeAccountJson, 0600)
				if err != nil {
					log.Printf("[FAIL] Error during ioutil.WriteFile(\"acme-account.json\"), waiting for manual restart to avoid rate limits: %s", err)
					select {}
				}
			}
		}
	} else {
		panic(err)
	}

	acmeClient, err = lego.NewClient(myAcmeConfig)
	if err != nil {
		log.Printf("[ERROR] Can't create ACME client, continuing with mock certs only: %s", err)
	} else {
		err = acmeClient.Challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{})
		if err != nil {
			log.Printf("[ERROR] Can't create TLS-ALPN-01 provider: %s", err)
		}
		if os.Getenv("ENABLE_HTTP_SERVER") == "true" {
			err = acmeClient.Challenge.SetHTTP01Provider(AcmeHTTPChallengeProvider{})
			if err != nil {
				log.Printf("[ERROR] Can't create HTTP-01 provider: %s", err)
			}
		}
	}

	mainDomainAcmeClient, err = lego.NewClient(myAcmeConfig)
	if err != nil {
		log.Printf("[ERROR] Can't create ACME client, continuing with mock certs only: %s", err)
	} else {
		if os.Getenv("DNS_PROVIDER") == "" {
			// using mock server, don't use wildcard certs
			err := mainDomainAcmeClient.Challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{})
			if err != nil {
				log.Printf("[ERROR] Can't create TLS-ALPN-01 provider: %s", err)
			}
		} else {
			provider, err := dns.NewDNSChallengeProviderByName(os.Getenv("DNS_PROVIDER"))
			if err != nil {
				log.Printf("[ERROR] Can't create DNS Challenge provider: %s", err)
			}
			err = mainDomainAcmeClient.Challenge.SetDNS01Provider(provider)
			if err != nil {
				log.Printf("[ERROR] Can't create DNS-01 provider: %s", err)
			}
		}
	}

	if mainCertBytes == nil {
		_, err = obtainCert(mainDomainAcmeClient, []string{"*" + string(MainDomainSuffix), string(MainDomainSuffix[1:])}, nil, "")
		if err != nil {
			log.Printf("[ERROR] Couldn't renew main domain certificate, continuing with mock certs only: %s", err)
		}
	}

	go (func() {
		for {
			err := keyDatabase.Sync()
			if err != nil {
				log.Printf("[ERROR] Syncinc key database failed: %s", err)
			}
			time.Sleep(5 * time.Minute)
		}
	})()
	go (func() {
		for {
			// clean up expired certs
			now := time.Now()
			expiredCertCount := 0
			keyDatabaseIterator := keyDatabase.Items()
			key, resBytes, err := keyDatabaseIterator.Next()
			for err == nil {
				if !bytes.Equal(key, MainDomainSuffix) {
					resGob := bytes.NewBuffer(resBytes)
					resDec := gob.NewDecoder(resGob)
					res := &certificate.Resource{}
					err = resDec.Decode(res)
					if err != nil {
						panic(err)
					}

					tlsCertificates, err := certcrypto.ParsePEMBundle(res.Certificate)
					if err != nil || !tlsCertificates[0].NotAfter.After(now) {
						err := keyDatabase.Delete(key)
						if err != nil {
							log.Printf("[ERROR] Deleting expired certificate for %s failed: %s", string(key), err)
						} else {
							expiredCertCount++
						}
					}
				}
				key, resBytes, err = keyDatabaseIterator.Next()
			}
			log.Printf("[INFO] Removed %d expired certificates from the database", expiredCertCount)

			// compact the database
			result, err := keyDatabase.Compact()
			if err != nil {
				log.Printf("[ERROR] Compacting key database failed: %s", err)
			} else {
				log.Printf("[INFO] Compacted key database (%+v)", result)
			}

			// update main cert
			resBytes, err = keyDatabase.Get(MainDomainSuffix)
			if err != nil {
				// key database is not working
				panic(err)
			}

			resGob := bytes.NewBuffer(resBytes)
			resDec := gob.NewDecoder(resGob)
			res := &certificate.Resource{}
			err = resDec.Decode(res)
			if err != nil {
				panic(err)
			}

			tlsCertificates, err := certcrypto.ParsePEMBundle(res.Certificate)

			// renew main certificate 30 days before it expires
			if !tlsCertificates[0].NotAfter.After(time.Now().Add(-30 * 24 * time.Hour)) {
				go (func() {
					_, err = obtainCert(mainDomainAcmeClient, []string{"*" + string(MainDomainSuffix), string(MainDomainSuffix[1:])}, res, "")
					if err != nil {
						log.Printf("Couldn't renew certificate for *%s: %s", MainDomainSuffix, err)
					}
				})()
			}

			time.Sleep(12 * time.Hour)
		}
	})()
}
