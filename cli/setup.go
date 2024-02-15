package cli

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/database"
	"codeberg.org/codeberg/pages/server/version"
)

func CreatePagesApp() *cli.App {
	app := cli.NewApp()
	app.Name = "pages-server"
	app.Version = version.Version
	app.Usage = "pages server"
	app.Flags = ServerFlags
	app.Commands = []*cli.Command{
		Certs,
	}

	return app
}

func OpenCertDB(ctx *cli.Context) (certDB database.CertDB, closeFn func(), err error) {
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
