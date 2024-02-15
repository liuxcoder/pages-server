package acme

import (
	"errors"
	"fmt"

	"codeberg.org/codeberg/pages/config"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
)

var ErrAcmeMissConfig = errors.New("ACME client has wrong config")

func CreateAcmeClient(cfg config.ACMEConfig, enableHTTPServer bool, challengeCache cache.ICache) (*certificates.AcmeClient, error) {
	// check config
	if (!cfg.AcceptTerms || cfg.DNSProvider == "") && cfg.APIEndpoint != "https://acme.mock.directory" {
		return nil, fmt.Errorf("%w: you must set $ACME_ACCEPT_TERMS and $DNS_PROVIDER, unless $ACME_API is set to https://acme.mock.directory", ErrAcmeMissConfig)
	}
	if cfg.EAB_HMAC != "" && cfg.EAB_KID == "" {
		return nil, fmt.Errorf("%w: ACME_EAB_HMAC also needs ACME_EAB_KID to be set", ErrAcmeMissConfig)
	} else if cfg.EAB_HMAC == "" && cfg.EAB_KID != "" {
		return nil, fmt.Errorf("%w: ACME_EAB_KID also needs ACME_EAB_HMAC to be set", ErrAcmeMissConfig)
	}

	return certificates.NewAcmeClient(cfg, enableHTTPServer, challengeCache)
}
