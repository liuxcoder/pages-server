//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"codeberg.org/codeberg/pages/cmd"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func TestMain(m *testing.M) {
	log.Printf("=== TestMain: START Server ==\n")
	serverCtx, serverCancel := context.WithCancel(context.Background())
	if err := startServer(serverCtx); err != nil {
		log.Fatal().Msgf("could not start server: %v", err)
	}
	defer func() {
		serverCancel()
		log.Printf("=== TestMain: Server STOPED ==\n")
	}()

	time.Sleep(20 * time.Second)

	os.Exit(m.Run())
}

func startServer(ctx context.Context) error {
	args := []string{
		"--verbose",
		"--acme-accept-terms", "true",
	}
	setEnvIfNotSet("ACME_API", "https://acme.mock.directory")
	setEnvIfNotSet("PAGES_DOMAIN", "localhost.mock.directory")
	setEnvIfNotSet("RAW_DOMAIN", "raw.localhost.mock.directory")
	setEnvIfNotSet("PORT", "4430")

	app := cli.NewApp()
	app.Name = "pages-server"
	app.Action = cmd.Serve
	app.Flags = cmd.ServeFlags

	go func() {
		if err := app.RunContext(ctx, args); err != nil {
			log.Fatal().Msgf("run server error: %v", err)
		}
	}()

	return nil
}

func setEnvIfNotSet(key, value string) {
	if _, set := os.LookupEnv(key); !set {
		os.Setenv(key, value)
	}
}
