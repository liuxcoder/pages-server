package cmd

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
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
	},
	Flags: CertStorageFlags,
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
