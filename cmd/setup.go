package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/certificates"
	"codeberg.org/codeberg/pages/server/database"
)

var ErrAcmeMissConfig = errors.New("ACME client has wrong config")

func openCertDB(ctx *cli.Context) (certDB database.CertDB, closeFn func(), err error) {
	certDB, err = database.NewXormDB(ctx.String("db-type"), ctx.String("db-conn"))
	if err != nil {
		return nil, nil, fmt.Errorf("could not connect to database: %w", err)
	}

	closeFn = func() {
		if err := certDB.Close(); err != nil {
			log.Error().Err(err)
		}
	}

	return certDB, closeFn, nil
}

func createAcmeClient(ctx *cli.Context, enableHTTPServer bool, challengeCache cache.SetGetKey) (*certificates.AcmeClient, error) {
	acmeAPI := ctx.String("acme-api-endpoint")
	acmeMail := ctx.String("acme-email")
	acmeEabHmac := ctx.String("acme-eab-hmac")
	acmeEabKID := ctx.String("acme-eab-kid")
	acmeAcceptTerms := ctx.Bool("acme-accept-terms")
	dnsProvider := ctx.String("dns-provider")
	acmeUseRateLimits := ctx.Bool("acme-use-rate-limits")
	acmeAccountConf := ctx.String("acme-account-config")

	// check config
	if (!acmeAcceptTerms || dnsProvider == "") && acmeAPI != "https://acme.mock.directory" {
		return nil, fmt.Errorf("%w: you must set $ACME_ACCEPT_TERMS and $DNS_PROVIDER, unless $ACME_API is set to https://acme.mock.directory", ErrAcmeMissConfig)
	}

	return certificates.NewAcmeClient(
		acmeAccountConf,
		acmeAPI,
		acmeMail,
		acmeEabHmac,
		acmeEabKID,
		dnsProvider,
		acmeAcceptTerms,
		enableHTTPServer,
		acmeUseRateLimits,
		challengeCache,
	)
}
