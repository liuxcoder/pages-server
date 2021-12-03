## Environment

- `HOST` & `PORT` (default: `[::]` & `443`): listen address.
- `PAGES_DOMAIN` (default: `codeberg.page`): main domain for pages.
- `RAW_DOMAIN` (default: `raw.codeberg.org`): domain for raw resources.
- `GITEA_ROOT` (default: `https://codeberg.org`): root of the upstream Gitea instance.
- `GITEA_API_TOKEN` (default: empty): API token for the Gitea instance to access non-public (e.g. limited) repos.
- `REDIRECT_RAW_INFO` (default: https://docs.codeberg.org/pages/raw-content/): info page for raw resources, shown if no resource is provided.
- `ACME_API` (default: https://acme-v02.api.letsencrypt.org/directory): set this to https://acme.mock.director to use invalid certificates without any verification (great for debugging).  
  ZeroSSL might be better in the future as it doesn't have rate limits and doesn't clash with the official Codeberg certificates (which are using Let's Encrypt), but I couldn't get it to work yet.
- `ACME_EMAIL` (default: `noreply@example.email`): Set this to "true" to accept the Terms of Service of your ACME provider.
- `ACME_EAB_KID` &  `ACME_EAB_HMAC` (default: don't use EAB): EAB credentials, for example for ZeroSSL.
- `ACME_ACCEPT_TERMS` (default: use self-signed certificate): Set this to "true" to accept the Terms of Service of your ACME provider.
- `ACME_USE_RATE_LIMITS` (default: true): Set this to false to disable rate limits, e.g. with ZeroSSL.
- `ENABLE_HTTP_SERVER` (default: false): Set this to true to enable the HTTP-01 challenge and redirect all other HTTP requests to HTTPS. Currently only works with port 80.
- `DNS_PROVIDER` (default: use self-signed certificate): Code of the ACME DNS provider for the main domain wildcard.  
  See https://go-acme.github.io/lego/dns/ for available values & additional environment variables.


// Package main is the new Codeberg Pages server, a solution for serving static pages from Gitea repositories.
//
// Mapping custom domains is not static anymore, but can be done with DNS:
//
// 1) add a "domains.txt" text file to your repository, containing the allowed domains, separated by new lines. The
// first line will be the canonical domain/URL; all other occurrences will be redirected to it.
//
// 2) add a CNAME entry to your domain, pointing to "[[{branch}.]{repo}.]{owner}.codeberg.page" (repo defaults to
// "pages", "branch" defaults to the default branch if "repo" is "pages", or to "pages" if "repo" is something else):
//      www.example.org. IN CNAME main.pages.example.codeberg.page.
//
// 3) if a CNAME is set for "www.example.org", you can redirect there from the naked domain by adding an ALIAS record
// for "example.org" (if your provider allows ALIAS or similar records):
//      example.org IN ALIAS codeberg.page.
//
// Certificates are generated, updated and cleaned up automatically via Let's Encrypt through a TLS challenge.
