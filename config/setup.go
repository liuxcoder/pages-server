package config

import (
	"os"
	"path"

	"github.com/creasty/defaults"
	"github.com/pelletier/go-toml/v2"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var ALWAYS_BLACKLISTED_PATHS = []string{
	"/.well-known/acme-challenge/",
}

func NewDefaultConfig() Config {
	config := Config{}
	if err := defaults.Set(&config); err != nil {
		panic(err)
	}

	// defaults does not support setting arrays from strings
	config.Server.PagesBranches = []string{"main", "master", "pages"}

	return config
}

func ReadConfig(ctx *cli.Context) (*Config, error) {
	config := NewDefaultConfig()
	// if config is not given as argument return empty config
	if !ctx.IsSet("config-file") {
		return &config, nil
	}

	configFile := path.Clean(ctx.String("config-file"))

	log.Debug().Str("config-file", configFile).Msg("reading config file")
	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal(content, &config)
	return &config, err
}

func MergeConfig(ctx *cli.Context, config *Config) {
	if ctx.IsSet("log-level") {
		config.LogLevel = ctx.String("log-level")
	}

	mergeServerConfig(ctx, &config.Server)
	mergeGiteaConfig(ctx, &config.Gitea)
	mergeDatabaseConfig(ctx, &config.Database)
	mergeACMEConfig(ctx, &config.ACME)
}

func mergeServerConfig(ctx *cli.Context, config *ServerConfig) {
	if ctx.IsSet("host") {
		config.Host = ctx.String("host")
	}
	if ctx.IsSet("port") {
		config.Port = uint16(ctx.Uint("port"))
	}
	if ctx.IsSet("http-port") {
		config.HttpPort = uint16(ctx.Uint("http-port"))
	}
	if ctx.IsSet("enable-http-server") {
		config.HttpServerEnabled = ctx.Bool("enable-http-server")
	}
	if ctx.IsSet("pages-domain") {
		config.MainDomain = ctx.String("pages-domain")
	}
	if ctx.IsSet("raw-domain") {
		config.RawDomain = ctx.String("raw-domain")
	}
	if ctx.IsSet("pages-branch") {
		config.PagesBranches = ctx.StringSlice("pages-branch")
	}
	if ctx.IsSet("allowed-cors-domains") {
		config.AllowedCorsDomains = ctx.StringSlice("allowed-cors-domains")
	}
	if ctx.IsSet("blacklisted-paths") {
		config.BlacklistedPaths = ctx.StringSlice("blacklisted-paths")
	}

	// add the paths that should always be blacklisted
	config.BlacklistedPaths = append(config.BlacklistedPaths, ALWAYS_BLACKLISTED_PATHS...)
}

func mergeGiteaConfig(ctx *cli.Context, config *GiteaConfig) {
	if ctx.IsSet("gitea-root") {
		config.Root = ctx.String("gitea-root")
	}
	if ctx.IsSet("gitea-api-token") {
		config.Token = ctx.String("gitea-api-token")
	}
	if ctx.IsSet("enable-lfs-support") {
		config.LFSEnabled = ctx.Bool("enable-lfs-support")
	}
	if ctx.IsSet("enable-symlink-support") {
		config.FollowSymlinks = ctx.Bool("enable-symlink-support")
	}
	if ctx.IsSet("default-mime-type") {
		config.DefaultMimeType = ctx.String("default-mime-type")
	}
	if ctx.IsSet("forbidden-mime-types") {
		config.ForbiddenMimeTypes = ctx.StringSlice("forbidden-mime-types")
	}
}

func mergeDatabaseConfig(ctx *cli.Context, config *DatabaseConfig) {
	if ctx.IsSet("db-type") {
		config.Type = ctx.String("db-type")
	}
	if ctx.IsSet("db-conn") {
		config.Conn = ctx.String("db-conn")
	}
}

func mergeACMEConfig(ctx *cli.Context, config *ACMEConfig) {
	if ctx.IsSet("acme-email") {
		config.Email = ctx.String("acme-email")
	}
	if ctx.IsSet("acme-api-endpoint") {
		config.APIEndpoint = ctx.String("acme-api-endpoint")
	}
	if ctx.IsSet("acme-accept-terms") {
		config.AcceptTerms = ctx.Bool("acme-accept-terms")
	}
	if ctx.IsSet("acme-use-rate-limits") {
		config.UseRateLimits = ctx.Bool("acme-use-rate-limits")
	}
	if ctx.IsSet("acme-eab-hmac") {
		config.EAB_HMAC = ctx.String("acme-eab-hmac")
	}
	if ctx.IsSet("acme-eab-kid") {
		config.EAB_KID = ctx.String("acme-eab-kid")
	}
	if ctx.IsSet("dns-provider") {
		config.DNSProvider = ctx.String("dns-provider")
	}
	if ctx.IsSet("acme-account-config") {
		config.AccountConfigFile = ctx.String("acme-account-config")
	}
}
