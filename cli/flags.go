package cli

import (
	"github.com/urfave/cli/v2"
)

var (
	CertStorageFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    "db-type",
			Usage:   "Specify the database driver. Valid options are \"sqlite3\", \"mysql\" and \"postgres\". Read more at https://xorm.io",
			Value:   "sqlite3",
			EnvVars: []string{"DB_TYPE"},
		},
		&cli.StringFlag{
			Name:    "db-conn",
			Usage:   "Specify the database connection. For \"sqlite3\" it's the filepath. Read more at https://go.dev/doc/tutorial/database-access",
			Value:   "certs.sqlite",
			EnvVars: []string{"DB_CONN"},
		},
	}

	ServerFlags = append(CertStorageFlags, []cli.Flag{
		// #############
		// ### Gitea ###
		// #############
		// GiteaRoot specifies the root URL of the Gitea instance, without a trailing slash.
		&cli.StringFlag{
			Name:    "gitea-root",
			Usage:   "specifies the root URL of the Gitea instance, without a trailing slash.",
			EnvVars: []string{"GITEA_ROOT"},
		},
		// GiteaApiToken specifies an api token for the Gitea instance
		&cli.StringFlag{
			Name:    "gitea-api-token",
			Usage:   "specifies an api token for the Gitea instance",
			EnvVars: []string{"GITEA_API_TOKEN"},
		},
		&cli.BoolFlag{
			Name:    "enable-lfs-support",
			Usage:   "enable lfs support, require gitea >= v1.17.0 as backend",
			EnvVars: []string{"ENABLE_LFS_SUPPORT"},
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "enable-symlink-support",
			Usage:   "follow symlinks if enabled, require gitea >= v1.18.0 as backend",
			EnvVars: []string{"ENABLE_SYMLINK_SUPPORT"},
			Value:   false,
		},
		&cli.StringFlag{
			Name:    "default-mime-type",
			Usage:   "specifies the default mime type for files that don't have a specific mime type.",
			EnvVars: []string{"DEFAULT_MIME_TYPE"},
			Value:   "application/octet-stream",
		},
		&cli.StringSliceFlag{
			Name:    "forbidden-mime-types",
			Usage:   "specifies the forbidden mime types. Use this flag multiple times for multiple mime types.",
			EnvVars: []string{"FORBIDDEN_MIME_TYPES"},
		},

		// ###########################
		// ### Page Server Domains ###
		// ###########################
		// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
		// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
		// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
		&cli.StringFlag{
			Name:    "pages-domain",
			Usage:   "specifies the main domain (starting with a dot) for which subdomains shall be served as static pages",
			EnvVars: []string{"PAGES_DOMAIN"},
		},
		// RawDomain specifies the domain from which raw repository content shall be served in the following format:
		// https://{RawDomain}/{owner}/{repo}[/{branch|tag|commit}/{version}]/{filepath...}
		// (set to []byte(nil) to disable raw content hosting)
		&cli.StringFlag{
			Name:    "raw-domain",
			Usage:   "specifies the domain from which raw repository content shall be served, not set disable raw content hosting",
			EnvVars: []string{"RAW_DOMAIN"},
		},

		// #########################
		// ### Page Server Setup ###
		// #########################
		&cli.StringFlag{
			Name:    "host",
			Usage:   "specifies host of listening address",
			EnvVars: []string{"HOST"},
			Value:   "[::]",
		},
		&cli.UintFlag{
			Name:    "port",
			Usage:   "specifies the https port to listen to ssl requests",
			EnvVars: []string{"PORT", "HTTPS_PORT"},
			Value:   443,
		},
		&cli.UintFlag{
			Name:    "http-port",
			Usage:   "specifies the http port, you also have to enable http server via ENABLE_HTTP_SERVER=true",
			EnvVars: []string{"HTTP_PORT"},
			Value:   80,
		},
		&cli.BoolFlag{
			Name:    "enable-http-server",
			Usage:   "start a http server to redirect to https and respond to http acme challenges",
			EnvVars: []string{"ENABLE_HTTP_SERVER"},
			Value:   false,
		},
		// Default branches to fetch assets from
		&cli.StringSliceFlag{
			Name:    "pages-branch",
			Usage:   "define a branch to fetch assets from. Use this flag multiple times for multiple branches.",
			EnvVars: []string{"PAGES_BRANCHES"},
			Value:   cli.NewStringSlice("pages"),
		},

		&cli.StringSliceFlag{
			Name:    "allowed-cors-domains",
			Usage:   "specify allowed CORS domains. Use this flag multiple times for multiple domains.",
			EnvVars: []string{"ALLOWED_CORS_DOMAINS"},
		},
		&cli.StringSliceFlag{
			Name:    "blacklisted-paths",
			Usage:   "return an error on these url paths.Use this flag multiple times for multiple paths.",
			EnvVars: []string{"BLACKLISTED_PATHS"},
		},

		&cli.StringFlag{
			Name:    "log-level",
			Value:   "warn",
			Usage:   "specify at which log level should be logged. Possible options: info, warn, error, fatal",
			EnvVars: []string{"LOG_LEVEL"},
		},
		&cli.StringFlag{
			Name:    "config-file",
			Usage:   "specify the location of the config file",
			Aliases: []string{"config"},
			EnvVars: []string{"CONFIG_FILE"},
		},
		&cli.Uint64Flag{
			Name:    "memory-limit",
			Usage:   "maximum size of memory in KiB to use for caching, default: 512MiB",
			Value:   512 * 1024,
			EnvVars: []string{"MAX_MEMORY_SIZE"},
		},

		// ############################
		// ### ACME Client Settings ###
		// ############################
		&cli.StringFlag{
			Name:    "acme-api-endpoint",
			EnvVars: []string{"ACME_API"},
			Value:   "https://acme-v02.api.letsencrypt.org/directory",
		},
		&cli.StringFlag{
			Name:    "acme-email",
			EnvVars: []string{"ACME_EMAIL"},
			Value:   "noreply@example.email",
		},
		&cli.BoolFlag{
			Name: "acme-use-rate-limits",
			// TODO: Usage
			EnvVars: []string{"ACME_USE_RATE_LIMITS"},
			Value:   true,
		},
		&cli.BoolFlag{
			Name:    "acme-accept-terms",
			Usage:   "To accept the ACME ToS",
			EnvVars: []string{"ACME_ACCEPT_TERMS"},
		},
		&cli.StringFlag{
			Name:    "acme-eab-kid",
			Usage:   "Register the current account to the ACME server with external binding.",
			EnvVars: []string{"ACME_EAB_KID"},
		},
		&cli.StringFlag{
			Name:    "acme-eab-hmac",
			Usage:   "Register the current account to the ACME server with external binding.",
			EnvVars: []string{"ACME_EAB_HMAC"},
		},
		&cli.StringFlag{
			Name:    "dns-provider",
			Usage:   "Use DNS-Challenge for main domain. Read more at: https://go-acme.github.io/lego/dns/",
			EnvVars: []string{"DNS_PROVIDER"},
		},
		&cli.StringFlag{
			Name:    "acme-account-config",
			Usage:   "json file of acme account",
			Value:   "acme-account.json",
			EnvVars: []string{"ACME_ACCOUNT_CONFIG"},
		},
	}...)
)
