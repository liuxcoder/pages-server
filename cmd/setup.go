package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/database"
)

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
