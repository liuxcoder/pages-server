package certificates

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
	"github.com/reugn/equalizer"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/database"
	dnsutils "codeberg.org/codeberg/pages/server/dns"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

// TLSConfig returns the configuration for generating, serving and cleaning up Let's Encrypt certificates.
func TLSConfig(mainDomainSuffix []byte,
	giteaClient *gitea.Client,
	dnsProvider string,
	acmeUseRateLimits bool,
	keyCache, challengeCache, dnsLookupCache, canonicalDomainCache cache.SetGetKey,
	certDB database.CertDB,
) *tls.Config {
	return &tls.Config{
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
			if bytes.HasSuffix(sniBytes, mainDomainSuffix) || bytes.Equal(sniBytes, mainDomainSuffix[1:]) {
				// deliver default certificate for the main domain (*.codeberg.page)
				sniBytes = mainDomainSuffix
				sni = string(sniBytes)
			} else {
				var targetRepo, targetBranch string
				targetOwner, targetRepo, targetBranch = dnsutils.GetTargetFromDNS(sni, string(mainDomainSuffix), dnsLookupCache)
				if targetOwner == "" {
					// DNS not set up, return main certificate to redirect to the docs
					sniBytes = mainDomainSuffix
					sni = string(sniBytes)
				} else {
					_, _ = targetRepo, targetBranch
					_, valid := upstream.CheckCanonicalDomain(giteaClient, targetOwner, targetRepo, targetBranch, sni, string(mainDomainSuffix), canonicalDomainCache)
					if !valid {
						sniBytes = mainDomainSuffix
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
			if tlsCertificate, ok = retrieveCertFromDB(sniBytes, mainDomainSuffix, dnsProvider, acmeUseRateLimits, certDB); !ok {
				// request a new certificate
				if bytes.Equal(sniBytes, mainDomainSuffix) {
					return nil, errors.New("won't request certificate for main domain, something really bad has happened")
				}

				tlsCertificate, err = obtainCert(acmeClient, []string{sni}, nil, targetOwner, dnsProvider, mainDomainSuffix, acmeUseRateLimits, certDB)
				if err != nil {
					return nil, err
				}
			}

			if err := keyCache.Set(sni, &tlsCertificate, 15*time.Minute); err != nil {
				return nil, err
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
}

func checkUserLimit(user string) error {
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

var (
	acmeClient, mainDomainAcmeClient  *lego.Client
	acmeClientCertificateLimitPerUser = map[string]*equalizer.TokenBucket{}
)

// rate limit is 300 / 3 hours, we want 200 / 2 hours but to refill more often, so that's 25 new domains every 15 minutes
// TODO: when this is used a lot, we probably have to think of a somewhat better solution?
var acmeClientOrderLimit = equalizer.NewTokenBucket(25, 15*time.Minute)

// rate limit is 20 / second, we want 5 / second (especially as one cert takes at least two requests)
var acmeClientRequestLimit = equalizer.NewTokenBucket(5, 1*time.Second)

type AcmeTLSChallengeProvider struct {
	challengeCache cache.SetGetKey
}

// make sure AcmeTLSChallengeProvider match Provider interface
var _ challenge.Provider = AcmeTLSChallengeProvider{}

func (a AcmeTLSChallengeProvider) Present(domain, _, keyAuth string) error {
	return a.challengeCache.Set(domain, keyAuth, 1*time.Hour)
}

func (a AcmeTLSChallengeProvider) CleanUp(domain, _, _ string) error {
	a.challengeCache.Remove(domain)
	return nil
}

type AcmeHTTPChallengeProvider struct {
	challengeCache cache.SetGetKey
}

// make sure AcmeHTTPChallengeProvider match Provider interface
var _ challenge.Provider = AcmeHTTPChallengeProvider{}

func (a AcmeHTTPChallengeProvider) Present(domain, token, keyAuth string) error {
	return a.challengeCache.Set(domain+"/"+token, keyAuth, 1*time.Hour)
}

func (a AcmeHTTPChallengeProvider) CleanUp(domain, token, _ string) error {
	a.challengeCache.Remove(domain + "/" + token)
	return nil
}

func retrieveCertFromDB(sni, mainDomainSuffix []byte, dnsProvider string, acmeUseRateLimits bool, certDB database.CertDB) (tls.Certificate, bool) {
	// parse certificate from database
	res, err := certDB.Get(string(sni))
	if err != nil {
		panic(err) // TODO: no panic
	}
	if res == nil {
		return tls.Certificate{}, false
	}

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		panic(err)
	}

	// TODO: document & put into own function
	if !bytes.Equal(sni, mainDomainSuffix) {
		tlsCertificate.Leaf, err = x509.ParseCertificate(tlsCertificate.Certificate[0])
		if err != nil {
			panic(err)
		}

		// renew certificates 7 days before they expire
		if !tlsCertificate.Leaf.NotAfter.After(time.Now().Add(7 * 24 * time.Hour)) {
			// TODO: add ValidUntil to custom res struct
			if res.CSR != nil && len(res.CSR) > 0 {
				// CSR stores the time when the renewal shall be tried again
				nextTryUnix, err := strconv.ParseInt(string(res.CSR), 10, 64)
				if err == nil && time.Now().Before(time.Unix(nextTryUnix, 0)) {
					return tlsCertificate, true
				}
			}
			go (func() {
				res.CSR = nil // acme client doesn't like CSR to be set
				tlsCertificate, err = obtainCert(acmeClient, []string{string(sni)}, res, "", dnsProvider, mainDomainSuffix, acmeUseRateLimits, certDB)
				if err != nil {
					log.Error().Msgf("Couldn't renew certificate for %s: %v", string(sni), err)
				}
			})()
		}
	}

	return tlsCertificate, true
}

var obtainLocks = sync.Map{}

func obtainCert(acmeClient *lego.Client, domains []string, renew *certificate.Resource, user, dnsProvider string, mainDomainSuffix []byte, acmeUseRateLimits bool, keyDatabase database.CertDB) (tls.Certificate, error) {
	name := strings.TrimPrefix(domains[0], "*")
	if dnsProvider == "" && len(domains[0]) > 0 && domains[0][0] == '*' {
		domains = domains[1:]
	}

	// lock to avoid simultaneous requests
	_, working := obtainLocks.LoadOrStore(name, struct{}{})
	if working {
		for working {
			time.Sleep(100 * time.Millisecond)
			_, working = obtainLocks.Load(name)
		}
		cert, ok := retrieveCertFromDB([]byte(name), mainDomainSuffix, dnsProvider, acmeUseRateLimits, keyDatabase)
		if !ok {
			return tls.Certificate{}, errors.New("certificate failed in synchronous request")
		}
		return cert, nil
	}
	defer obtainLocks.Delete(name)

	if acmeClient == nil {
		return mockCert(domains[0], "ACME client uninitialized. This is a server error, please report!", string(mainDomainSuffix), keyDatabase), nil
	}

	// request actual cert
	var res *certificate.Resource
	var err error
	if renew != nil && renew.CertURL != "" {
		if acmeUseRateLimits {
			acmeClientRequestLimit.Take()
		}
		log.Debug().Msgf("Renewing certificate for: %v", domains)
		res, err = acmeClient.Certificate.Renew(*renew, true, false, "")
		if err != nil {
			log.Error().Err(err).Msgf("Couldn't renew certificate for %v, trying to request a new one", domains)
			res = nil
		}
	}
	if res == nil {
		if user != "" {
			if err := checkUserLimit(user); err != nil {
				return tls.Certificate{}, err
			}
		}

		if acmeUseRateLimits {
			acmeClientOrderLimit.Take()
			acmeClientRequestLimit.Take()
		}
		log.Debug().Msgf("Re-requesting new certificate for %v", domains)
		res, err = acmeClient.Certificate.Obtain(certificate.ObtainRequest{
			Domains:    domains,
			Bundle:     true,
			MustStaple: false,
		})
	}
	if err != nil {
		log.Error().Err(err).Msgf("Couldn't obtain again a certificate or %v", domains)
		if renew != nil && renew.CertURL != "" {
			tlsCertificate, err := tls.X509KeyPair(renew.Certificate, renew.PrivateKey)
			if err == nil && tlsCertificate.Leaf.NotAfter.After(time.Now()) {
				// avoid sending a mock cert instead of a still valid cert, instead abuse CSR field to store time to try again at
				renew.CSR = []byte(strconv.FormatInt(time.Now().Add(6*time.Hour).Unix(), 10))
				if err := keyDatabase.Put(name, renew); err != nil {
					return mockCert(domains[0], err.Error(), string(mainDomainSuffix), keyDatabase), err
				}
				return tlsCertificate, nil
			}
		}
		return mockCert(domains[0], err.Error(), string(mainDomainSuffix), keyDatabase), err
	}
	log.Debug().Msgf("Obtained certificate for %v", domains)

	if err := keyDatabase.Put(name, res); err != nil {
		return tls.Certificate{}, err
	}
	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tlsCertificate, nil
}

func SetupAcmeConfig(acmeAPI, acmeMail, acmeEabHmac, acmeEabKID string, acmeAcceptTerms bool) (*lego.Config, error) {
	const configFile = "acme-account.json"
	var myAcmeAccount AcmeAccount
	var myAcmeConfig *lego.Config

	if account, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(account, &myAcmeAccount); err != nil {
			return nil, err
		}
		myAcmeAccount.Key, err = certcrypto.ParsePEMPrivateKey([]byte(myAcmeAccount.KeyPEM))
		if err != nil {
			return nil, err
		}
		myAcmeConfig = lego.NewConfig(&myAcmeAccount)
		myAcmeConfig.CADirURL = acmeAPI
		myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048

		// Validate Config
		_, err := lego.NewClient(myAcmeConfig)
		if err != nil {
			// TODO: should we fail hard instead?
			log.Error().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
		}
		return myAcmeConfig, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	myAcmeAccount = AcmeAccount{
		Email:  acmeMail,
		Key:    privateKey,
		KeyPEM: string(certcrypto.PEMEncode(privateKey)),
	}
	myAcmeConfig = lego.NewConfig(&myAcmeAccount)
	myAcmeConfig.CADirURL = acmeAPI
	myAcmeConfig.Certificate.KeyType = certcrypto.RSA2048
	tempClient, err := lego.NewClient(myAcmeConfig)
	if err != nil {
		log.Error().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
	} else {
		// accept terms & log in to EAB
		if acmeEabKID == "" || acmeEabHmac == "" {
			reg, err := tempClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: acmeAcceptTerms})
			if err != nil {
				log.Error().Err(err).Msg("Can't register ACME account, continuing with mock certs only")
			} else {
				myAcmeAccount.Registration = reg
			}
		} else {
			reg, err := tempClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
				TermsOfServiceAgreed: acmeAcceptTerms,
				Kid:                  acmeEabKID,
				HmacEncoded:          acmeEabHmac,
			})
			if err != nil {
				log.Error().Err(err).Msg("Can't register ACME account, continuing with mock certs only")
			} else {
				myAcmeAccount.Registration = reg
			}
		}

		if myAcmeAccount.Registration != nil {
			acmeAccountJSON, err := json.Marshal(myAcmeAccount)
			if err != nil {
				log.Error().Err(err).Msg("json.Marshalfailed, waiting for manual restart to avoid rate limits")
				select {}
			}
			err = os.WriteFile(configFile, acmeAccountJSON, 0o600)
			if err != nil {
				log.Error().Err(err).Msg("os.WriteFile failed, waiting for manual restart to avoid rate limits")
				select {}
			}
		}
	}

	return myAcmeConfig, nil
}

func SetupCertificates(mainDomainSuffix []byte, dnsProvider string, acmeConfig *lego.Config, acmeUseRateLimits, enableHTTPServer bool, challengeCache cache.SetGetKey, certDB database.CertDB) error {
	// getting main cert before ACME account so that we can fail here without hitting rate limits
	mainCertBytes, err := certDB.Get(string(mainDomainSuffix))
	if err != nil {
		return fmt.Errorf("cert database is not working")
	}

	acmeClient, err = lego.NewClient(acmeConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
	} else {
		err = acmeClient.Challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{challengeCache})
		if err != nil {
			log.Error().Err(err).Msg("Can't create TLS-ALPN-01 provider")
		}
		if enableHTTPServer {
			err = acmeClient.Challenge.SetHTTP01Provider(AcmeHTTPChallengeProvider{challengeCache})
			if err != nil {
				log.Error().Err(err).Msg("Can't create HTTP-01 provider")
			}
		}
	}

	mainDomainAcmeClient, err = lego.NewClient(acmeConfig)
	if err != nil {
		log.Error().Err(err).Msg("Can't create ACME client, continuing with mock certs only")
	} else {
		if dnsProvider == "" {
			// using mock server, don't use wildcard certs
			err := mainDomainAcmeClient.Challenge.SetTLSALPN01Provider(AcmeTLSChallengeProvider{challengeCache})
			if err != nil {
				log.Error().Err(err).Msg("Can't create TLS-ALPN-01 provider")
			}
		} else {
			provider, err := dns.NewDNSChallengeProviderByName(dnsProvider)
			if err != nil {
				log.Error().Err(err).Msg("Can't create DNS Challenge provider")
			}
			err = mainDomainAcmeClient.Challenge.SetDNS01Provider(provider)
			if err != nil {
				log.Error().Err(err).Msg("Can't create DNS-01 provider")
			}
		}
	}

	if mainCertBytes == nil {
		_, err = obtainCert(mainDomainAcmeClient, []string{"*" + string(mainDomainSuffix), string(mainDomainSuffix[1:])}, nil, "", dnsProvider, mainDomainSuffix, acmeUseRateLimits, certDB)
		if err != nil {
			log.Error().Err(err).Msg("Couldn't renew main domain certificate, continuing with mock certs only")
		}
	}

	return nil
}

func MaintainCertDB(ctx context.Context, interval time.Duration, mainDomainSuffix []byte, dnsProvider string, acmeUseRateLimits bool, certDB database.CertDB) {
	for {
		// clean up expired certs
		now := time.Now()
		expiredCertCount := 0
		keyDatabaseIterator := certDB.Items()
		key, resBytes, err := keyDatabaseIterator.Next()
		for err == nil {
			if !bytes.Equal(key, mainDomainSuffix) {
				resGob := bytes.NewBuffer(resBytes)
				resDec := gob.NewDecoder(resGob)
				res := &certificate.Resource{}
				err = resDec.Decode(res)
				if err != nil {
					panic(err)
				}

				tlsCertificates, err := certcrypto.ParsePEMBundle(res.Certificate)
				if err != nil || !tlsCertificates[0].NotAfter.After(now) {
					err := certDB.Delete(string(key))
					if err != nil {
						log.Error().Err(err).Msgf("Deleting expired certificate for %q failed", string(key))
					} else {
						expiredCertCount++
					}
				}
			}
			key, resBytes, err = keyDatabaseIterator.Next()
		}
		log.Debug().Msgf("Removed %d expired certificates from the database", expiredCertCount)

		// compact the database
		msg, err := certDB.Compact()
		if err != nil {
			log.Error().Err(err).Msg("Compacting key database failed")
		} else {
			log.Debug().Msgf("Compacted key database: %s", msg)
		}

		// update main cert
		res, err := certDB.Get(string(mainDomainSuffix))
		if err != nil {
			log.Error().Msgf("Couldn't get cert for domain %q", mainDomainSuffix)
		} else if res == nil {
			log.Error().Msgf("Couldn't renew certificate for main domain %q expected main domain cert to exist, but it's missing - seems like the database is corrupted", string(mainDomainSuffix))
		} else {
			tlsCertificates, err := certcrypto.ParsePEMBundle(res.Certificate)

			// renew main certificate 30 days before it expires
			if !tlsCertificates[0].NotAfter.After(time.Now().Add(30 * 24 * time.Hour)) {
				go (func() {
					_, err = obtainCert(mainDomainAcmeClient, []string{"*" + string(mainDomainSuffix), string(mainDomainSuffix[1:])}, res, "", dnsProvider, mainDomainSuffix, acmeUseRateLimits, certDB)
					if err != nil {
						log.Error().Err(err).Msg("Couldn't renew certificate for main domain")
					}
				})()
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
