package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"codeberg.org/codeberg/pages/cmd"
)

var (
	// can be changed with -X on compile
	version = "dev"
)

func main() {
	app := cli.NewApp()
	app.Name = "pages-server"
	app.Version = version
	app.Usage = "pages server"
	app.Action = cmd.Serve
	app.Flags = cmd.ServeFlags
	app.Commands = []*cli.Command{
		cmd.Certs,
	}

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
