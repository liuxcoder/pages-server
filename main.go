package main

import (
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/cmd"
	"codeberg.org/codeberg/pages/server/version"
)

func main() {
	app := cli.NewApp()
	app.Name = "pages-server"
	app.Version = version.Version
	app.Usage = "pages server"
	app.Action = cmd.Serve
	app.Flags = cmd.ServerFlags
	app.Commands = []*cli.Command{
		cmd.Certs,
	}

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
