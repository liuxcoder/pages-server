package certificates

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/utils"
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

func SetupHTTPACMEChallengeServer(challengeCache cache.SetGetKey) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := context.New(w, req)
		if strings.HasPrefix(ctx.Path(), challengePath) {
			challenge, ok := challengeCache.Get(utils.TrimHostPort(ctx.Host()) + "/" + strings.TrimPrefix(ctx.Path(), challengePath))
			if !ok || challenge == nil {
				ctx.String("no challenge for this token", http.StatusNotFound)
			}
			ctx.String(challenge.(string))
		} else {
			ctx.Redirect("https://"+ctx.Host()+ctx.Path(), http.StatusMovedPermanently)
		}
	}
}
