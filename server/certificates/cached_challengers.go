package certificates

import (
	"time"

	"github.com/go-acme/lego/v4/challenge"

	"codeberg.org/codeberg/pages/server/cache"
)

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
