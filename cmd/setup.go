package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/database"
)

func openCertDB(ctx *cli.Context) (certDB database.CertDB, closeFn func(), err error) {
	if ctx.String("db-type") != "" {
		log.Trace().Msg("use xorm mode")
		certDB, err = database.NewXormDB(ctx.String("db-type"), ctx.String("db-conn"))
		if err != nil {
			return nil, nil, fmt.Errorf("could not connect to database: %w", err)
		}
	} else {
		// TODO: remove in next version
		fmt.Println(`
######################
## W A R N I N G !!! #
######################

You use "pogreb" witch is deprecated and will be removed in the next version.
Please switch to sqlite, mysql or postgres !!!

The simplest way is, to use './pages certs migrate' and set environment var DB_TYPE to 'sqlite' on next start.`)
		log.Error().Msg("depricated \"pogreb\" used\n")

		certDB, err = database.NewPogreb(ctx.String("db-pogreb"))
		if err != nil {
			return nil, nil, fmt.Errorf("could not create database: %w", err)
		}
	}

	closeFn = func() {
		if err := certDB.Close(); err != nil {
			log.Error().Err(err)
		}
	}

	return certDB, closeFn, nil
}
