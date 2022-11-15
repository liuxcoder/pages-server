//go:build integration
// +build integration

package integration

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"codeberg.org/codeberg/pages/cmd"

	"github.com/urfave/cli/v2"
)

func TestMain(m *testing.M) {
	log.Println("=== TestMain: START Server ===")
	serverCtx, serverCancel := context.WithCancel(context.Background())
	if err := startServer(serverCtx); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
	defer func() {
		serverCancel()
		log.Println("=== TestMain: Server STOPED ===")
	}()

	time.Sleep(10 * time.Second)

	m.Run()
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
			log.Fatalf("run server error: %v", err)
		}
	}()

	return nil
}

func setEnvIfNotSet(key, value string) {
	if _, set := os.LookupEnv(key); !set {
		os.Setenv(key, value)
	}
}
