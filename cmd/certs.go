package cmd

import (
	"os"

	"github.com/urfave/cli/v2"

	pages_server "codeberg.org/codeberg/pages/server"
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

		if pages_server.KeyDatabaseErr != nil {
			panic(pages_server.KeyDatabaseErr)
		}
		for _, domain := range domains {
			if err := pages_server.KeyDatabase.Delete([]byte(domain)); err != nil {
				panic(err)
			}
		}
		if err := pages_server.KeyDatabase.Sync(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}
	return nil
}
