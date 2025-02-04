//go:build integration
// +build integration

package integration

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/urfave/cli/v2"

	cmd "codeberg.org/codeberg/pages/cli"
	"codeberg.org/codeberg/pages/server"
)

func TestMain(m *testing.M) {
	log.Println("=== TestMain: START Server ===")
	serverCtx, serverCancel := context.WithCancel(context.Background())
	if err := startServer(serverCtx); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
	defer func() {
		serverCancel()
		log.Println("=== TestMain: Server STOPPED ===")
	}()

	time.Sleep(10 * time.Second)

	m.Run()
}

func startServer(ctx context.Context) error {
	args := []string{"integration"}
	setEnvIfNotSet("ACME_API", "https://acme.mock.directory")
	setEnvIfNotSet("PAGES_DOMAIN", "localhost.mock.directory")
	setEnvIfNotSet("RAW_DOMAIN", "raw.localhost.mock.directory")
	setEnvIfNotSet("PAGES_BRANCHES", "pages,main,master")
	setEnvIfNotSet("PORT", "4430")
	setEnvIfNotSet("HTTP_PORT", "8880")
	setEnvIfNotSet("ENABLE_HTTP_SERVER", "true")
	setEnvIfNotSet("DB_TYPE", "sqlite3")
	setEnvIfNotSet("GITEA_ROOT", "https://codeberg.org")
	setEnvIfNotSet("LOG_LEVEL", "trace")
	setEnvIfNotSet("ENABLE_LFS_SUPPORT", "true")
	setEnvIfNotSet("ENABLE_SYMLINK_SUPPORT", "true")
	setEnvIfNotSet("ACME_ACCOUNT_CONFIG", "integration/acme-account.json")

	app := cli.NewApp()
	app.Name = "pages-server"
	app.Action = server.Serve
	app.Flags = cmd.ServerFlags

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
