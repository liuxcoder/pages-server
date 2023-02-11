package certificates

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/reugn/equalizer"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
)

type AcmeClient struct {
	legoClient              *lego.Client
	dnsChallengerLegoClient *lego.Client

	obtainLocks sync.Map

	acmeUseRateLimits bool

	// limiter
	acmeClientOrderLimit              *equalizer.TokenBucket
	acmeClientRequestLimit            *equalizer.TokenBucket
	acmeClientFailLimit               *equalizer.TokenBucket
	acmeClientCertificateLimitPerUser map[string]*equalizer.TokenBucket
}

func NewAcmeClient(acmeAccountConf, acmeAPI, acmeMail, acmeEabHmac, acmeEabKID, dnsProvider string, acmeAcceptTerms, enableHTTPServer, acmeUseRateLimits bool, challengeCache cache.SetGetKey) (*AcmeClient, error) {
	acmeConfig, err := setupAcmeConfig(acmeAccountConf, acmeAPI, acmeMail, acmeEabHmac, acmeEabKID, acmeAcceptTerms)
	if err != nil {
		return nil, err
	}

	acmeClient, err := lego.NewClient(acmeConfig)
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

	mainDomainAcmeClient, err := lego.NewClient(acmeConfig)
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
			// use DNS-Challenge https://go-acme.github.io/lego/dns/
			provider, err := dns.NewDNSChallengeProviderByName(dnsProvider)
			if err != nil {
				return nil, fmt.Errorf("can not create DNS Challenge provider: %w", err)
			}
			if err := mainDomainAcmeClient.Challenge.SetDNS01Provider(provider); err != nil {
				return nil, fmt.Errorf("can not create DNS-01 provider: %w", err)
			}
		}
	}

	return &AcmeClient{
		legoClient:              acmeClient,
		dnsChallengerLegoClient: mainDomainAcmeClient,

		acmeUseRateLimits: acmeUseRateLimits,

		obtainLocks: sync.Map{},

		// limiter

		// rate limit is 300 / 3 hours, we want 200 / 2 hours but to refill more often, so that's 25 new domains every 15 minutes
		// TODO: when this is used a lot, we probably have to think of a somewhat better solution?
		acmeClientOrderLimit: equalizer.NewTokenBucket(25, 15*time.Minute),
		// rate limit is 20 / second, we want 5 / second (especially as one cert takes at least two requests)
		acmeClientRequestLimit: equalizer.NewTokenBucket(5, 1*time.Second),
		// rate limit is 5 / hour https://letsencrypt.org/docs/failed-validation-limit/
		acmeClientFailLimit: equalizer.NewTokenBucket(5, 1*time.Hour),
		// checkUserLimit() use this to rate als per user
		acmeClientCertificateLimitPerUser: map[string]*equalizer.TokenBucket{},
	}, nil
}
