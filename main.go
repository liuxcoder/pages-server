package main

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"

	"codeberg.org/codeberg/pages/cli"
	"codeberg.org/codeberg/pages/server"
)

func main() {
	app := cli.CreatePagesApp()
	app.Action = server.Serve

	if err := app.Run(os.Args); err != nil {
		log.Error().Err(err).Msg("A fatal error occurred")
		os.Exit(1)
	}
}
