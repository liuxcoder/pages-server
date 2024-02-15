package certificates

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
)

type AcmeTLSChallengeProvider struct {
	challengeCache cache.ICache
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
	challengeCache cache.ICache
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

func SetupHTTPACMEChallengeServer(challengeCache cache.ICache, sslPort uint) http.HandlerFunc {
	// handle custom-ssl-ports to be added on https redirects
	portPart := ""
	if sslPort != 443 {
		portPart = fmt.Sprintf(":%d", sslPort)
	}

	return func(w http.ResponseWriter, req *http.Request) {
		ctx := context.New(w, req)
		domain := ctx.TrimHostPort()

		// it's an acme request
		if strings.HasPrefix(ctx.Path(), challengePath) {
			challenge, ok := challengeCache.Get(domain + "/" + strings.TrimPrefix(ctx.Path(), challengePath))
			if !ok || challenge == nil {
				log.Info().Msgf("HTTP-ACME challenge for '%s' failed: token not found", domain)
				ctx.String("no challenge for this token", http.StatusNotFound)
			}
			log.Info().Msgf("HTTP-ACME challenge for '%s' succeeded", domain)
			ctx.String(challenge.(string))
			return
		}

		// it's a normal http request that needs to be redirected
		u, err := url.Parse(fmt.Sprintf("https://%s%s%s", domain, portPart, ctx.Path()))
		if err != nil {
			log.Error().Err(err).Msg("could not craft http to https redirect")
			ctx.String("", http.StatusInternalServerError)
		}

		newURL := u.String()
		log.Debug().Msgf("redirect http to https: %s", newURL)
		ctx.Redirect(newURL, http.StatusMovedPermanently)
	}
}
