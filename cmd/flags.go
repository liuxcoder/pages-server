package cmd

import (
	"github.com/urfave/cli/v2"
)

var ServeFlags = []cli.Flag{
	// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
	// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
	// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
	&cli.StringFlag{
		Name:    "pages-domain",
		Usage:   "specifies the main domain (starting with a dot) for which subdomains shall be served as static pages",
		EnvVars: []string{"PAGES_DOMAIN"},
		Value:   "codeberg.page",
	},
	// GiteaRoot specifies the root URL of the Gitea instance, without a trailing slash.
	&cli.StringFlag{
		Name:    "gitea-root",
		Usage:   "specifies the root URL of the Gitea instance, without a trailing slash.",
		EnvVars: []string{"GITEA_ROOT"},
		Value:   "https://codeberg.org",
	},
	// GiteaApiToken specifies an api token for the Gitea instance
	&cli.StringFlag{
		Name:    "gitea-api-token",
		Usage:   "specifies an api token for the Gitea instance",
		EnvVars: []string{"GITEA_API_TOKEN"},
		Value:   "",
	},
	// RawDomain specifies the domain from which raw repository content shall be served in the following format:
	// https://{RawDomain}/{owner}/{repo}[/{branch|tag|commit}/{version}]/{filepath...}
	// (set to []byte(nil) to disable raw content hosting)
	&cli.StringFlag{
		Name:    "raw-domain",
		Usage:   "specifies the domain from which raw repository content shall be served, not set disable raw content hosting",
		EnvVars: []string{"RAW_DOMAIN"},
		Value:   "raw.codeberg.org",
	},
	// RawInfoPage will be shown (with a redirect) when trying to access RawDomain directly (or without owner/repo/path).
	&cli.StringFlag{
		Name:    "raw-info-page",
		Usage:   "will be shown (with a redirect) when trying to access $RAW_DOMAIN directly (or without owner/repo/path)",
		EnvVars: []string{"RAW_INFO_PAGE"},
		Value:   "https://docs.codeberg.org/pages/raw-content/",
	},

	// Server
	&cli.StringFlag{
		Name:    "host",
		Usage:   "specifies host of listening address",
		EnvVars: []string{"HOST"},
		Value:   "[::]",
	},
	&cli.StringFlag{
		Name:    "port",
		Usage:   "specifies port of listening address",
		EnvVars: []string{"PORT"},
		Value:   "443",
	},
	&cli.BoolFlag{
		Name: "enable-http-server",
		// TODO: desc
		EnvVars: []string{"ENABLE_HTTP_SERVER"},
	},

	// ACME
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
		Name: "acme-accept-terms",
		// TODO: Usage
		EnvVars: []string{"ACME_ACCEPT_TERMS"},
	},
	&cli.StringFlag{
		Name: "acme-eab-kid",
		// TODO: Usage
		EnvVars: []string{"ACME_EAB_KID"},
	},
	&cli.StringFlag{
		Name: "acme-eab-hmac",
		// TODO: Usage
		EnvVars: []string{"ACME_EAB_HMAC"},
	},
	&cli.StringFlag{
		Name: "dns-provider",
		// TODO: Usage
		EnvVars: []string{"DNS_PROVIDER"},
	},
}
