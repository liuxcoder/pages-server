package cmd

import (
	"fmt"

	"github.com/akrylysov/pogreb"
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
	},
}

func listCerts(ctx *cli.Context) error {
	// TODO: make "key-database.pogreb" set via flag
	keyDatabase, err := database.New("key-database.pogreb")
	if err != nil {
		return fmt.Errorf("could not create database: %v", err)
	}

	items := keyDatabase.Items()
	for domain, _, err := items.Next(); err != pogreb.ErrIterationDone; domain, _, err = items.Next() {
		if err != nil {
			return err
		}
		if domain[0] == '.' {
			fmt.Printf("*")
		}
		fmt.Printf("%s\n", domain)
	}
	return nil
}

func removeCert(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		return fmt.Errorf("'certs remove' requires at least one domain as an argument")
	}

	domains := ctx.Args().Slice()

	// TODO: make "key-database.pogreb" set via flag
	keyDatabase, err := database.New("key-database.pogreb")
	if err != nil {
		return fmt.Errorf("could not create database: %v", err)
	}

	for _, domain := range domains {
		fmt.Printf("Removing domain %s from the database...\n", domain)
		if err := keyDatabase.Delete(domain); err != nil {
			return err
		}
	}
	if err := keyDatabase.Close(); err != nil {
		return err
	}
	return nil
}
