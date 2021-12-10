package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/server/database"
)

var Certs = &cli.Command{
	Name:   "certs",
	Usage:  "manage certs manually",
	Action: certs,
}

func certs(ctx *cli.Context) error {
	if ctx.Args().Len() >= 1 && ctx.Args().First() == "--remove-certificate" {
		if ctx.Args().Len() == 1 {
			println("--remove-certificate requires at least one domain as an argument")
			os.Exit(1)
		}

		domains := ctx.Args().Slice()[2:]

		// TODO: make "key-database.pogreb" set via flag
		keyDatabase, err := database.New("key-database.pogreb")
		if err != nil {
			return fmt.Errorf("could not create database: %v", err)
		}

		for _, domain := range domains {
			if err := keyDatabase.Delete([]byte(domain)); err != nil {
				panic(err)
			}
		}
		if err := keyDatabase.Close(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}
	return nil
}
