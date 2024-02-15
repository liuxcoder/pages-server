package config

import (
	"context"
	"os"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	cmd "codeberg.org/codeberg/pages/cli"
)

func runApp(t *testing.T, fn func(*cli.Context) error, args []string) {
	app := cmd.CreatePagesApp()
	app.Action = fn

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// os.Args always contains the binary name
	args = append([]string{"testing"}, args...)

	err := app.RunContext(appCtx, args)
	assert.NoError(t, err)
}

// fixArrayFromCtx fixes the number of "changed" strings in a string slice according to the number of values in the context.
// This is a workaround because the cli library has a bug where the number of values in the context gets bigger the more tests are run.
func fixArrayFromCtx(ctx *cli.Context, key string, expected []string) []string {
	if ctx.IsSet(key) {
		ctxSlice := ctx.StringSlice(key)

		if len(ctxSlice) > 1 {
			for i := 1; i < len(ctxSlice); i++ {
				expected = append([]string{"changed"}, expected...)
			}
		}
	}

	return expected
}

func readTestConfig() (*Config, error) {
	content, err := os.ReadFile("assets/test_config.toml")
	if err != nil {
		return nil, err
	}

	expectedConfig := NewDefaultConfig()
	err = toml.Unmarshal(content, &expectedConfig)
	if err != nil {
		return nil, err
	}

	return &expectedConfig, nil
}

func TestReadConfigShouldReturnEmptyConfigWhenConfigArgEmpty(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg, err := ReadConfig(ctx)
			expected := NewDefaultConfig()
			assert.Equal(t, &expected, cfg)

			return err
		},
		[]string{},
	)
}

func TestReadConfigShouldReturnConfigFromFileWhenConfigArgPresent(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg, err := ReadConfig(ctx)
			if err != nil {
				return err
			}

			expectedConfig, err := readTestConfig()
			if err != nil {
				return err
			}

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{"--config-file", "assets/test_config.toml"},
	)
}

func TestValuesReadFromConfigFileShouldBeOverwrittenByArgs(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg, err := ReadConfig(ctx)
			if err != nil {
				return err
			}

			MergeConfig(ctx, cfg)

			expectedConfig, err := readTestConfig()
			if err != nil {
				return err
			}

			expectedConfig.LogLevel = "debug"
			expectedConfig.Gitea.Root = "not-codeberg.org"
			expectedConfig.ACME.AcceptTerms = true
			expectedConfig.Server.Host = "172.17.0.2"
			expectedConfig.Server.BlacklistedPaths = append(expectedConfig.Server.BlacklistedPaths, ALWAYS_BLACKLISTED_PATHS...)

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{
			"--config-file", "assets/test_config.toml",
			"--log-level", "debug",
			"--gitea-root", "not-codeberg.org",
			"--acme-accept-terms",
			"--host", "172.17.0.2",
		},
	)
}

func TestMergeConfigShouldReplaceAllExistingValuesGivenAllArgsExist(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg := &Config{
				LogLevel: "original",
				Server: ServerConfig{
					Host:               "original",
					Port:               8080,
					HttpPort:           80,
					HttpServerEnabled:  false,
					MainDomain:         "original",
					RawDomain:          "original",
					PagesBranches:      []string{"original"},
					AllowedCorsDomains: []string{"original"},
					BlacklistedPaths:   []string{"original"},
				},
				Gitea: GiteaConfig{
					Root:               "original",
					Token:              "original",
					LFSEnabled:         false,
					FollowSymlinks:     false,
					DefaultMimeType:    "original",
					ForbiddenMimeTypes: []string{"original"},
				},
				Database: DatabaseConfig{
					Type: "original",
					Conn: "original",
				},
				ACME: ACMEConfig{
					Email:             "original",
					APIEndpoint:       "original",
					AcceptTerms:       false,
					UseRateLimits:     false,
					EAB_HMAC:          "original",
					EAB_KID:           "original",
					DNSProvider:       "original",
					AccountConfigFile: "original",
				},
			}

			MergeConfig(ctx, cfg)

			expectedConfig := &Config{
				LogLevel: "changed",
				Server: ServerConfig{
					Host:               "changed",
					Port:               8443,
					HttpPort:           443,
					HttpServerEnabled:  true,
					MainDomain:         "changed",
					RawDomain:          "changed",
					PagesBranches:      []string{"changed"},
					AllowedCorsDomains: []string{"changed"},
					BlacklistedPaths:   append([]string{"changed"}, ALWAYS_BLACKLISTED_PATHS...),
				},
				Gitea: GiteaConfig{
					Root:               "changed",
					Token:              "changed",
					LFSEnabled:         true,
					FollowSymlinks:     true,
					DefaultMimeType:    "changed",
					ForbiddenMimeTypes: []string{"changed"},
				},
				Database: DatabaseConfig{
					Type: "changed",
					Conn: "changed",
				},
				ACME: ACMEConfig{
					Email:             "changed",
					APIEndpoint:       "changed",
					AcceptTerms:       true,
					UseRateLimits:     true,
					EAB_HMAC:          "changed",
					EAB_KID:           "changed",
					DNSProvider:       "changed",
					AccountConfigFile: "changed",
				},
			}

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{
			"--log-level", "changed",
			// Server
			"--pages-domain", "changed",
			"--raw-domain", "changed",
			"--allowed-cors-domains", "changed",
			"--blacklisted-paths", "changed",
			"--pages-branch", "changed",
			"--host", "changed",
			"--port", "8443",
			"--http-port", "443",
			"--enable-http-server",
			// Gitea
			"--gitea-root", "changed",
			"--gitea-api-token", "changed",
			"--enable-lfs-support",
			"--enable-symlink-support",
			"--default-mime-type", "changed",
			"--forbidden-mime-types", "changed",
			// Database
			"--db-type", "changed",
			"--db-conn", "changed",
			// ACME
			"--acme-email", "changed",
			"--acme-api-endpoint", "changed",
			"--acme-accept-terms",
			"--acme-use-rate-limits",
			"--acme-eab-hmac", "changed",
			"--acme-eab-kid", "changed",
			"--dns-provider", "changed",
			"--acme-account-config", "changed",
		},
	)
}

func TestMergeServerConfigShouldAddDefaultBlacklistedPathsToBlacklistedPaths(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg := &ServerConfig{}
			mergeServerConfig(ctx, cfg)

			expected := ALWAYS_BLACKLISTED_PATHS
			assert.Equal(t, expected, cfg.BlacklistedPaths)

			return nil
		},
		[]string{},
	)
}

func TestMergeServerConfigShouldReplaceAllExistingValuesGivenAllArgsExist(t *testing.T) {
	for range []uint8{0, 1} {
		runApp(
			t,
			func(ctx *cli.Context) error {
				cfg := &ServerConfig{
					Host:               "original",
					Port:               8080,
					HttpPort:           80,
					HttpServerEnabled:  false,
					MainDomain:         "original",
					RawDomain:          "original",
					AllowedCorsDomains: []string{"original"},
					BlacklistedPaths:   []string{"original"},
				}

				mergeServerConfig(ctx, cfg)

				expectedConfig := &ServerConfig{
					Host:               "changed",
					Port:               8443,
					HttpPort:           443,
					HttpServerEnabled:  true,
					MainDomain:         "changed",
					RawDomain:          "changed",
					AllowedCorsDomains: fixArrayFromCtx(ctx, "allowed-cors-domains", []string{"changed"}),
					BlacklistedPaths:   fixArrayFromCtx(ctx, "blacklisted-paths", append([]string{"changed"}, ALWAYS_BLACKLISTED_PATHS...)),
				}

				assert.Equal(t, expectedConfig, cfg)

				return nil
			},
			[]string{
				"--pages-domain", "changed",
				"--raw-domain", "changed",
				"--allowed-cors-domains", "changed",
				"--blacklisted-paths", "changed",
				"--host", "changed",
				"--port", "8443",
				"--http-port", "443",
				"--enable-http-server",
			},
		)
	}
}

func TestMergeServerConfigShouldReplaceOnlyOneValueExistingValueGivenOnlyOneArgExists(t *testing.T) {
	type testValuePair struct {
		args     []string
		callback func(*ServerConfig)
	}
	testValuePairs := []testValuePair{
		{args: []string{"--host", "changed"}, callback: func(sc *ServerConfig) { sc.Host = "changed" }},
		{args: []string{"--port", "8443"}, callback: func(sc *ServerConfig) { sc.Port = 8443 }},
		{args: []string{"--http-port", "443"}, callback: func(sc *ServerConfig) { sc.HttpPort = 443 }},
		{args: []string{"--enable-http-server"}, callback: func(sc *ServerConfig) { sc.HttpServerEnabled = true }},
		{args: []string{"--pages-domain", "changed"}, callback: func(sc *ServerConfig) { sc.MainDomain = "changed" }},
		{args: []string{"--raw-domain", "changed"}, callback: func(sc *ServerConfig) { sc.RawDomain = "changed" }},
		{args: []string{"--pages-branch", "changed"}, callback: func(sc *ServerConfig) { sc.PagesBranches = []string{"changed"} }},
		{args: []string{"--allowed-cors-domains", "changed"}, callback: func(sc *ServerConfig) { sc.AllowedCorsDomains = []string{"changed"} }},
		{args: []string{"--blacklisted-paths", "changed"}, callback: func(sc *ServerConfig) { sc.BlacklistedPaths = []string{"changed"} }},
	}

	for _, pair := range testValuePairs {
		runApp(
			t,
			func(ctx *cli.Context) error {
				cfg := ServerConfig{
					Host:               "original",
					Port:               8080,
					HttpPort:           80,
					HttpServerEnabled:  false,
					MainDomain:         "original",
					RawDomain:          "original",
					PagesBranches:      []string{"original"},
					AllowedCorsDomains: []string{"original"},
					BlacklistedPaths:   []string{"original"},
				}

				expectedConfig := cfg
				pair.callback(&expectedConfig)
				expectedConfig.BlacklistedPaths = append(expectedConfig.BlacklistedPaths, ALWAYS_BLACKLISTED_PATHS...)

				expectedConfig.PagesBranches = fixArrayFromCtx(ctx, "pages-branch", expectedConfig.PagesBranches)
				expectedConfig.AllowedCorsDomains = fixArrayFromCtx(ctx, "allowed-cors-domains", expectedConfig.AllowedCorsDomains)
				expectedConfig.BlacklistedPaths = fixArrayFromCtx(ctx, "blacklisted-paths", expectedConfig.BlacklistedPaths)

				mergeServerConfig(ctx, &cfg)

				assert.Equal(t, expectedConfig, cfg)

				return nil
			},
			pair.args,
		)
	}
}

func TestMergeGiteaConfigShouldReplaceAllExistingValuesGivenAllArgsExist(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg := &GiteaConfig{
				Root:               "original",
				Token:              "original",
				LFSEnabled:         false,
				FollowSymlinks:     false,
				DefaultMimeType:    "original",
				ForbiddenMimeTypes: []string{"original"},
			}

			mergeGiteaConfig(ctx, cfg)

			expectedConfig := &GiteaConfig{
				Root:               "changed",
				Token:              "changed",
				LFSEnabled:         true,
				FollowSymlinks:     true,
				DefaultMimeType:    "changed",
				ForbiddenMimeTypes: fixArrayFromCtx(ctx, "forbidden-mime-types", []string{"changed"}),
			}

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{
			"--gitea-root", "changed",
			"--gitea-api-token", "changed",
			"--enable-lfs-support",
			"--enable-symlink-support",
			"--default-mime-type", "changed",
			"--forbidden-mime-types", "changed",
		},
	)
}

func TestMergeGiteaConfigShouldReplaceOnlyOneValueExistingValueGivenOnlyOneArgExists(t *testing.T) {
	type testValuePair struct {
		args     []string
		callback func(*GiteaConfig)
	}
	testValuePairs := []testValuePair{
		{args: []string{"--gitea-root", "changed"}, callback: func(gc *GiteaConfig) { gc.Root = "changed" }},
		{args: []string{"--gitea-api-token", "changed"}, callback: func(gc *GiteaConfig) { gc.Token = "changed" }},
		{args: []string{"--enable-lfs-support"}, callback: func(gc *GiteaConfig) { gc.LFSEnabled = true }},
		{args: []string{"--enable-symlink-support"}, callback: func(gc *GiteaConfig) { gc.FollowSymlinks = true }},
		{args: []string{"--default-mime-type", "changed"}, callback: func(gc *GiteaConfig) { gc.DefaultMimeType = "changed" }},
		{args: []string{"--forbidden-mime-types", "changed"}, callback: func(gc *GiteaConfig) { gc.ForbiddenMimeTypes = []string{"changed"} }},
	}

	for _, pair := range testValuePairs {
		runApp(
			t,
			func(ctx *cli.Context) error {
				cfg := GiteaConfig{
					Root:               "original",
					Token:              "original",
					LFSEnabled:         false,
					FollowSymlinks:     false,
					DefaultMimeType:    "original",
					ForbiddenMimeTypes: []string{"original"},
				}

				expectedConfig := cfg
				pair.callback(&expectedConfig)

				mergeGiteaConfig(ctx, &cfg)

				expectedConfig.ForbiddenMimeTypes = fixArrayFromCtx(ctx, "forbidden-mime-types", expectedConfig.ForbiddenMimeTypes)

				assert.Equal(t, expectedConfig, cfg)

				return nil
			},
			pair.args,
		)
	}
}

func TestMergeDatabaseConfigShouldReplaceAllExistingValuesGivenAllArgsExist(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg := &DatabaseConfig{
				Type: "original",
				Conn: "original",
			}

			mergeDatabaseConfig(ctx, cfg)

			expectedConfig := &DatabaseConfig{
				Type: "changed",
				Conn: "changed",
			}

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{
			"--db-type", "changed",
			"--db-conn", "changed",
		},
	)
}

func TestMergeDatabaseConfigShouldReplaceOnlyOneValueExistingValueGivenOnlyOneArgExists(t *testing.T) {
	type testValuePair struct {
		args     []string
		callback func(*DatabaseConfig)
	}
	testValuePairs := []testValuePair{
		{args: []string{"--db-type", "changed"}, callback: func(gc *DatabaseConfig) { gc.Type = "changed" }},
		{args: []string{"--db-conn", "changed"}, callback: func(gc *DatabaseConfig) { gc.Conn = "changed" }},
	}

	for _, pair := range testValuePairs {
		runApp(
			t,
			func(ctx *cli.Context) error {
				cfg := DatabaseConfig{
					Type: "original",
					Conn: "original",
				}

				expectedConfig := cfg
				pair.callback(&expectedConfig)

				mergeDatabaseConfig(ctx, &cfg)

				assert.Equal(t, expectedConfig, cfg)

				return nil
			},
			pair.args,
		)
	}
}

func TestMergeACMEConfigShouldReplaceAllExistingValuesGivenAllArgsExist(t *testing.T) {
	runApp(
		t,
		func(ctx *cli.Context) error {
			cfg := &ACMEConfig{
				Email:             "original",
				APIEndpoint:       "original",
				AcceptTerms:       false,
				UseRateLimits:     false,
				EAB_HMAC:          "original",
				EAB_KID:           "original",
				DNSProvider:       "original",
				AccountConfigFile: "original",
			}

			mergeACMEConfig(ctx, cfg)

			expectedConfig := &ACMEConfig{
				Email:             "changed",
				APIEndpoint:       "changed",
				AcceptTerms:       true,
				UseRateLimits:     true,
				EAB_HMAC:          "changed",
				EAB_KID:           "changed",
				DNSProvider:       "changed",
				AccountConfigFile: "changed",
			}

			assert.Equal(t, expectedConfig, cfg)

			return nil
		},
		[]string{
			"--acme-email", "changed",
			"--acme-api-endpoint", "changed",
			"--acme-accept-terms",
			"--acme-use-rate-limits",
			"--acme-eab-hmac", "changed",
			"--acme-eab-kid", "changed",
			"--dns-provider", "changed",
			"--acme-account-config", "changed",
		},
	)
}

func TestMergeACMEConfigShouldReplaceOnlyOneValueExistingValueGivenOnlyOneArgExists(t *testing.T) {
	type testValuePair struct {
		args     []string
		callback func(*ACMEConfig)
	}
	testValuePairs := []testValuePair{
		{args: []string{"--acme-email", "changed"}, callback: func(gc *ACMEConfig) { gc.Email = "changed" }},
		{args: []string{"--acme-api-endpoint", "changed"}, callback: func(gc *ACMEConfig) { gc.APIEndpoint = "changed" }},
		{args: []string{"--acme-accept-terms"}, callback: func(gc *ACMEConfig) { gc.AcceptTerms = true }},
		{args: []string{"--acme-use-rate-limits"}, callback: func(gc *ACMEConfig) { gc.UseRateLimits = true }},
		{args: []string{"--acme-eab-hmac", "changed"}, callback: func(gc *ACMEConfig) { gc.EAB_HMAC = "changed" }},
		{args: []string{"--acme-eab-kid", "changed"}, callback: func(gc *ACMEConfig) { gc.EAB_KID = "changed" }},
		{args: []string{"--dns-provider", "changed"}, callback: func(gc *ACMEConfig) { gc.DNSProvider = "changed" }},
		{args: []string{"--acme-account-config", "changed"}, callback: func(gc *ACMEConfig) { gc.AccountConfigFile = "changed" }},
	}

	for _, pair := range testValuePairs {
		runApp(
			t,
			func(ctx *cli.Context) error {
				cfg := ACMEConfig{
					Email:             "original",
					APIEndpoint:       "original",
					AcceptTerms:       false,
					UseRateLimits:     false,
					EAB_HMAC:          "original",
					EAB_KID:           "original",
					DNSProvider:       "original",
					AccountConfigFile: "original",
				}

				expectedConfig := cfg
				pair.callback(&expectedConfig)

				mergeACMEConfig(ctx, &cfg)

				assert.Equal(t, expectedConfig, cfg)

				return nil
			},
			pair.args,
		)
	}
}
