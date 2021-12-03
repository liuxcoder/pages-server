package cmd

import (
	"github.com/urfave/cli/v2"
)

var ServeFlags = []cli.Flag{
	// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
	// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
	// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
	&cli.StringFlag{
		Name:    "main-domain-suffix",
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
		EnvVars: []string{"REDIRECT_RAW_INFO"},
		Value:   "https://docs.codeberg.org/pages/raw-content/",
	},

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

	// ACME_API
	&cli.StringFlag{
		Name:    "acme-api",
		EnvVars: []string{"ACME_API"},
		Value:   "https://acme-v02.api.letsencrypt.org/directory",
	},
	&cli.StringFlag{
		Name:    "acme-email",
		EnvVars: []string{"ACME_EMAIL"},
		Value:   "noreply@example.email",
	},
}
