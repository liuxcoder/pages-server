## Environment

- `HOST` & `PORT` (default: `[::]` & `443`): listen address.
- `PAGES_DOMAIN` (default: `codeberg.page`): main domain for pages.
- `RAW_DOMAIN` (default: `raw.codeberg.org`): domain for raw resources.
- `GITEA_ROOT` (default: `https://codeberg.org`): root of the upstream Gitea instance.
- `REDIRECT_BROKEN_DNS` (default: https://docs.codeberg.org/pages/custom-domains/): info page for setting up DNS, shown for invalid DNS setups.
- `REDIRECT_RAW_INFO` (default: https://docs.codeberg.org/pages/raw-content/): info page for raw resources, shown if no resource is provided.
- `ACME_API` (default: https://acme.zerossl.com/v2/DV90): set this to https://acme.mock.director to use invalid certificates without any verification (great for debugging). ZeroSSL is used as it doesn't have rate limits and doesn't clash with the official Codeberg certificates (which are using Let's Encrypt).
- `ACME_EMAIL` (default: `noreply@example.email`): Set this to "true" to accept the Terms of Service of your ACME provider.
- `ACME_ACCEPT_TERMS` (default: use self-signed certificate): Set this to "true" to accept the Terms of Service of your ACME provider.
- `DNS_PROVIDER` (default: use self-signed certificate): Code of the ACME DNS provider for the main domain wildcard.  
  See https://go-acme.github.io/lego/dns/ for available values & additional environment variables.