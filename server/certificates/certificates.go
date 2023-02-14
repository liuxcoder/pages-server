package certificates

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/reugn/equalizer"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/database"
	dnsutils "codeberg.org/codeberg/pages/server/dns"
	"codeberg.org/codeberg/pages/server/gitea"
	"codeberg.org/codeberg/pages/server/upstream"
)

var ErrUserRateLimitExceeded = errors.New("rate limit exceeded: 10 certificates per user per 24 hours")

// TLSConfig returns the configuration for generating, serving and cleaning up Let's Encrypt certificates.
func TLSConfig(mainDomainSuffix string,
	giteaClient *gitea.Client,
	acmeClient *AcmeClient,
	keyCache, challengeCache, dnsLookupCache, canonicalDomainCache cache.SetGetKey,
	certDB database.CertDB,
) *tls.Config {
	return &tls.Config{
		// check DNS name & get certificate from Let's Encrypt
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			domain := strings.ToLower(strings.TrimSpace(info.ServerName))
			if len(domain) < 1 {
				return nil, errors.New("missing domain info via SNI (RFC 4366, Section 3.1)")
			}

			// https request init is actually a acme challenge
			if info.SupportedProtos != nil {
				for _, proto := range info.SupportedProtos {
					if proto != tlsalpn01.ACMETLS1Protocol {
						continue
					}
					log.Info().Msgf("Detect ACME-TLS1 challenge for '%s'", domain)

					challenge, ok := challengeCache.Get(domain)
					if !ok {
						return nil, errors.New("no challenge for this domain")
					}
					cert, err := tlsalpn01.ChallengeCert(domain, challenge.(string))
					if err != nil {
						return nil, err
					}
					return cert, nil
				}
			}

			targetOwner := ""
			mayObtainCert := true
			if strings.HasSuffix(domain, mainDomainSuffix) || strings.EqualFold(domain, mainDomainSuffix[1:]) {
				// deliver default certificate for the main domain (*.codeberg.page)
				domain = mainDomainSuffix
			} else {
				var targetRepo, targetBranch string
				targetOwner, targetRepo, targetBranch = dnsutils.GetTargetFromDNS(domain, mainDomainSuffix, dnsLookupCache)
				if targetOwner == "" {
					// DNS not set up, return main certificate to redirect to the docs
					domain = mainDomainSuffix
				} else {
					targetOpt := &upstream.Options{
						TargetOwner:  targetOwner,
						TargetRepo:   targetRepo,
						TargetBranch: targetBranch,
					}
					_, valid := targetOpt.CheckCanonicalDomain(giteaClient, domain, mainDomainSuffix, canonicalDomainCache)
					if !valid {
						// We shouldn't obtain a certificate when we cannot check if the
						// repository has specified this domain in the `.domains` file.
						mayObtainCert = false
					}
				}
			}

			if tlsCertificate, ok := keyCache.Get(domain); ok {
				// we can use an existing certificate object
				return tlsCertificate.(*tls.Certificate), nil
			}

			var tlsCertificate *tls.Certificate
			var err error
			if tlsCertificate, err = acmeClient.retrieveCertFromDB(domain, mainDomainSuffix, false, certDB); err != nil {
				if !errors.Is(err, database.ErrNotFound) {
					return nil, err
				}
				// we could not find a cert in db, request a new certificate

				// first check if we are allowed to obtain a cert for this domain
				if strings.EqualFold(domain, mainDomainSuffix) {
					return nil, errors.New("won't request certificate for main domain, something really bad has happened")
				}
				if !mayObtainCert {
					return nil, fmt.Errorf("won't request certificate for %q", domain)
				}

				tlsCertificate, err = acmeClient.obtainCert(acmeClient.legoClient, []string{domain}, nil, targetOwner, false, mainDomainSuffix, certDB)
				if err != nil {
					return nil, err
				}
			}

			if err := keyCache.Set(domain, tlsCertificate, 15*time.Minute); err != nil {
				return nil, err
			}
			return tlsCertificate, nil
		},
		NextProtos: []string{
			"h2",
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

func (c *AcmeClient) checkUserLimit(user string) error {
	userLimit, ok := c.acmeClientCertificateLimitPerUser[user]
	if !ok {
		// Each user can only add 10 new domains per day.
		userLimit = equalizer.NewTokenBucket(10, time.Hour*24)
		c.acmeClientCertificateLimitPerUser[user] = userLimit
	}
	if !userLimit.Ask() {
		return fmt.Errorf("user '%s' error: %w", user, ErrUserRateLimitExceeded)
	}
	return nil
}

func (c *AcmeClient) retrieveCertFromDB(sni, mainDomainSuffix string, useDnsProvider bool, certDB database.CertDB) (*tls.Certificate, error) {
	// parse certificate from database
	res, err := certDB.Get(sni)
	if err != nil {
		return nil, err
	} else if res == nil {
		return nil, database.ErrNotFound
	}

	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return nil, err
	}

	// TODO: document & put into own function
	if !strings.EqualFold(sni, mainDomainSuffix) {
		tlsCertificate.Leaf, err = x509.ParseCertificate(tlsCertificate.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("error parsing leaf tlsCert: %w", err)
		}

		// renew certificates 7 days before they expire
		if tlsCertificate.Leaf.NotAfter.Before(time.Now().Add(7 * 24 * time.Hour)) {
			// TODO: use ValidTill of custom cert struct
			if res.CSR != nil && len(res.CSR) > 0 {
				// CSR stores the time when the renewal shall be tried again
				nextTryUnix, err := strconv.ParseInt(string(res.CSR), 10, 64)
				if err == nil && time.Now().Before(time.Unix(nextTryUnix, 0)) {
					return &tlsCertificate, nil
				}
			}
			// TODO: make a queue ?
			go (func() {
				res.CSR = nil // acme client doesn't like CSR to be set
				if _, err := c.obtainCert(c.legoClient, []string{sni}, res, "", useDnsProvider, mainDomainSuffix, certDB); err != nil {
					log.Error().Msgf("Couldn't renew certificate for %s: %v", sni, err)
				}
			})()
		}
	}

	return &tlsCertificate, nil
}

func (c *AcmeClient) obtainCert(acmeClient *lego.Client, domains []string, renew *certificate.Resource, user string, useDnsProvider bool, mainDomainSuffix string, keyDatabase database.CertDB) (*tls.Certificate, error) {
	name := strings.TrimPrefix(domains[0], "*")
	if useDnsProvider && len(domains[0]) > 0 && domains[0][0] == '*' {
		domains = domains[1:]
	}

	// lock to avoid simultaneous requests
	_, working := c.obtainLocks.LoadOrStore(name, struct{}{})
	if working {
		for working {
			time.Sleep(100 * time.Millisecond)
			_, working = c.obtainLocks.Load(name)
		}
		cert, err := c.retrieveCertFromDB(name, mainDomainSuffix, useDnsProvider, keyDatabase)
		if err != nil {
			return nil, fmt.Errorf("certificate failed in synchronous request: %w", err)
		}
		return cert, nil
	}
	defer c.obtainLocks.Delete(name)

	if acmeClient == nil {
		return mockCert(domains[0], "ACME client uninitialized. This is a server error, please report!", mainDomainSuffix, keyDatabase)
	}

	// request actual cert
	var res *certificate.Resource
	var err error
	if renew != nil && renew.CertURL != "" {
		if c.acmeUseRateLimits {
			c.acmeClientRequestLimit.Take()
		}
		log.Debug().Msgf("Renewing certificate for: %v", domains)
		res, err = acmeClient.Certificate.Renew(*renew, true, false, "")
		if err != nil {
			log.Error().Err(err).Msgf("Couldn't renew certificate for %v, trying to request a new one", domains)
			if c.acmeUseRateLimits {
				c.acmeClientFailLimit.Take()
			}
			res = nil
		}
	}
	if res == nil {
		if user != "" {
			if err := c.checkUserLimit(user); err != nil {
				return nil, err
			}
		}

		if c.acmeUseRateLimits {
			c.acmeClientOrderLimit.Take()
			c.acmeClientRequestLimit.Take()
		}
		log.Debug().Msgf("Re-requesting new certificate for %v", domains)
		res, err = acmeClient.Certificate.Obtain(certificate.ObtainRequest{
			Domains:    domains,
			Bundle:     true,
			MustStaple: false,
		})
		if c.acmeUseRateLimits && err != nil {
			c.acmeClientFailLimit.Take()
		}
	}
	if err != nil {
		log.Error().Err(err).Msgf("Couldn't obtain again a certificate or %v", domains)
		if renew != nil && renew.CertURL != "" {
			tlsCertificate, err := tls.X509KeyPair(renew.Certificate, renew.PrivateKey)
			if err != nil {
				mockC, err2 := mockCert(domains[0], err.Error(), mainDomainSuffix, keyDatabase)
				if err2 != nil {
					return nil, errors.Join(err, err2)
				}
				return mockC, err
			}
			leaf, err := leaf(&tlsCertificate)
			if err == nil && leaf.NotAfter.After(time.Now()) {
				// avoid sending a mock cert instead of a still valid cert, instead abuse CSR field to store time to try again at
				renew.CSR = []byte(strconv.FormatInt(time.Now().Add(6*time.Hour).Unix(), 10))
				if err := keyDatabase.Put(name, renew); err != nil {
					mockC, err2 := mockCert(domains[0], err.Error(), mainDomainSuffix, keyDatabase)
					if err2 != nil {
						return nil, errors.Join(err, err2)
					}
					return mockC, err
				}
				return &tlsCertificate, nil
			}
		}
		return mockCert(domains[0], err.Error(), mainDomainSuffix, keyDatabase)
	}
	log.Debug().Msgf("Obtained certificate for %v", domains)

	if err := keyDatabase.Put(name, res); err != nil {
		return nil, err
	}
	tlsCertificate, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &tlsCertificate, nil
}

func SetupMainDomainCertificates(mainDomainSuffix string, acmeClient *AcmeClient, certDB database.CertDB) error {
	// getting main cert before ACME account so that we can fail here without hitting rate limits
	mainCertBytes, err := certDB.Get(mainDomainSuffix)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("cert database is not working: %w", err)
	}

	if mainCertBytes == nil {
		_, err = acmeClient.obtainCert(acmeClient.dnsChallengerLegoClient, []string{"*" + mainDomainSuffix, mainDomainSuffix[1:]}, nil, "", true, mainDomainSuffix, certDB)
		if err != nil {
			log.Error().Err(err).Msg("Couldn't renew main domain certificate, continuing with mock certs only")
		}
	}

	return nil
}

func MaintainCertDB(ctx context.Context, interval time.Duration, acmeClient *AcmeClient, mainDomainSuffix string, certDB database.CertDB) {
	for {
		// delete expired certs that will be invalid until next clean up
		threshold := time.Now().Add(interval)
		expiredCertCount := 0

		certs, err := certDB.Items(0, 0)
		if err != nil {
			log.Error().Err(err).Msg("could not get certs from list")
		} else {
			for _, cert := range certs {
				if !strings.EqualFold(cert.Domain, strings.TrimPrefix(mainDomainSuffix, ".")) {
					if time.Unix(cert.ValidTill, 0).Before(threshold) {
						err := certDB.Delete(cert.Domain)
						if err != nil {
							log.Error().Err(err).Msgf("Deleting expired certificate for %q failed", cert.Domain)
						} else {
							expiredCertCount++
						}
					}
				}
			}
			log.Debug().Msgf("Removed %d expired certificates from the database", expiredCertCount)
		}

		// update main cert
		res, err := certDB.Get(mainDomainSuffix)
		if err != nil {
			log.Error().Msgf("Couldn't get cert for domain %q", mainDomainSuffix)
		} else if res == nil {
			log.Error().Msgf("Couldn't renew certificate for main domain %q expected main domain cert to exist, but it's missing - seems like the database is corrupted", mainDomainSuffix)
		} else {
			tlsCertificates, err := certcrypto.ParsePEMBundle(res.Certificate)
			if err != nil {
				log.Error().Err(fmt.Errorf("could not parse cert for mainDomainSuffix: %w", err))
			} else if tlsCertificates[0].NotAfter.Before(time.Now().Add(30 * 24 * time.Hour)) {
				// renew main certificate 30 days before it expires
				go (func() {
					_, err = acmeClient.obtainCert(acmeClient.dnsChallengerLegoClient, []string{"*" + mainDomainSuffix, mainDomainSuffix[1:]}, res, "", true, mainDomainSuffix, certDB)
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

// leaf returns the parsed leaf certificate, either from c.leaf or by parsing
// the corresponding c.Certificate[0].
func leaf(c *tls.Certificate) (*x509.Certificate, error) {
	if c.Leaf != nil {
		return c.Leaf, nil
	}
	return x509.ParseCertificate(c.Certificate[0])
}
