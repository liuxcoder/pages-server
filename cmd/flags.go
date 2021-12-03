package cmd

import (
	"codeberg.org/codeberg/pages/server"
	"github.com/urfave/cli/v2"
)

// GiteaRoot specifies the root URL of the Gitea instance, without a trailing slash.
var GiteaRoot = []byte(server.EnvOr("GITEA_ROOT", "https://codeberg.org"))

var GiteaApiToken = server.EnvOr("GITEA_API_TOKEN", "")

// RawDomain specifies the domain from which raw repository content shall be served in the following format:
// https://{RawDomain}/{owner}/{repo}[/{branch|tag|commit}/{version}]/{filepath...}
// (set to []byte(nil) to disable raw content hosting)
var RawDomain = []byte(server.EnvOr("RAW_DOMAIN", "raw.codeberg.org"))

// RawInfoPage will be shown (with a redirect) when trying to access RawDomain directly (or without owner/repo/path).
var RawInfoPage = server.EnvOr("REDIRECT_RAW_INFO", "https://docs.codeberg.org/pages/raw-content/")

var ServeFlags = []cli.Flag{
	// MainDomainSuffix specifies the main domain (starting with a dot) for which subdomains shall be served as static
	// pages, or used for comparison in CNAME lookups. Static pages can be accessed through
	// https://{owner}.{MainDomain}[/{repo}], with repo defaulting to "pages".
	// var MainDomainSuffix = []byte("." + server.EnvOr("PAGES_DOMAIN", "codeberg.page"))
	&cli.StringFlag{
		Name:      "main-domain-suffix",
		Aliases:   nil,
		Usage:     "specifies the main domain (starting with a dot) for which subdomains shall be served as static pages",
		EnvVars:   []string{"PAGES_DOMAIN"},
		FilePath:  "",
		Required:  false,
		Hidden:    false,
		TakesFile: false,
		Value:     "codeberg.page",
	},
}
