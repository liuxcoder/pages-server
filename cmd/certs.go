package cmd

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/database"
)

var Certs = &cli.Command{
	Name:  "certs",
	Usage: "manage certs manually",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Usage:  "list all certificates in the database",
			Action: listCerts,
		},
		{
			Name:   "remove",
			Usage:  "remove a certificate from the database",
			Action: removeCert,
		},
		{
			Name:   "migrate",
			Usage:  "migrate from \"pogreb\" driver to dbms driver",
			Action: migrateCerts,
		},
	},
	Flags: append(CertStorageFlags, []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Usage:   "print trace info",
			EnvVars: []string{"VERBOSE"},
			Value:   false,
		},
	}...),
}

func migrateCerts(ctx *cli.Context) error {
	dbType := ctx.String("db-type")
	if dbType == "" {
		dbType = "sqlite3"
	}
	dbConn := ctx.String("db-conn")
	dbPogrebConn := ctx.String("db-pogreb")
	verbose := ctx.Bool("verbose")

	log.Level(zerolog.InfoLevel)
	if verbose {
		log.Level(zerolog.TraceLevel)
	}

	xormDB, err := database.NewXormDB(dbType, dbConn)
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}
	defer xormDB.Close()

	pogrebDB, err := database.NewPogreb(dbPogrebConn)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer pogrebDB.Close()

	fmt.Printf("Start migration from \"%s\" to \"%s:%s\" ...\n", dbPogrebConn, dbType, dbConn)

	certs, err := pogrebDB.Items(0, 0)
	if err != nil {
		return err
	}

	for _, cert := range certs {
		if err := xormDB.Put(cert.Domain, cert.Raw()); err != nil {
			return err
		}
	}

	fmt.Println("... done")
	return nil
}

func listCerts(ctx *cli.Context) error {
	certDB, closeFn, err := openCertDB(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	items, err := certDB.Items(0, 0)
	if err != nil {
		return err
	}

	fmt.Printf("Domain\tValidTill\n\n")
	for _, cert := range items {
		fmt.Printf("%s\t%s\n",
			cert.Domain,
			time.Unix(cert.ValidTill, 0).Format(time.RFC3339))
	}
	return nil
}

func removeCert(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		return fmt.Errorf("'certs remove' requires at least one domain as an argument")
	}

	domains := ctx.Args().Slice()

	certDB, closeFn, err := openCertDB(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	for _, domain := range domains {
		fmt.Printf("Removing domain %s from the database...\n", domain)
		if err := certDB.Delete(domain); err != nil {
			return err
		}
	}
	return nil
}
