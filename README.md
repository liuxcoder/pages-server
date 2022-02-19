# Codeberg Pages

Gitea lacks the ability to host static pages from Git.
The Codeberg Pages Server addresses this lack by implementing a standalone service
that connects to Gitea via API.
It is suitable to be deployed by other Gitea instances, too, to offer static pages hosting to their users.

**End user documentation** can mainly be found at the [Wiki](https://codeberg.org/Codeberg/pages-server/wiki/Overview)
and the [Codeberg Documentation](https://docs.codeberg.org/codeberg-pages/).


## Quickstart

This is the new Codeberg Pages server, a solution for serving static pages from Gitea repositories.
Mapping custom domains is not static anymore, but can be done with DNS:

1) add a `.domains` text file to your repository, containing the allowed domains, separated by new lines. The
first line will be the canonical domain/URL; all other occurrences will be redirected to it.

2) add a CNAME entry to your domain, pointing to `[[{branch}.]{repo}.]{owner}.codeberg.page` (repo defaults to
"pages", "branch" defaults to the default branch if "repo" is "pages", or to "pages" if "repo" is something else):
	`www.example.org. IN CNAME main.pages.example.codeberg.page.`

3) if a CNAME is set for "www.example.org", you can redirect there from the naked domain by adding an ALIAS record
for "example.org" (if your provider allows ALIAS or similar records, otherwise use A/AAAA), together with a TXT
record that points to your repo (just like the CNAME record):
	`example.org IN ALIAS codeberg.page.`
	`example.org IN TXT main.pages.example.codeberg.page.`

Certificates are generated, updated and cleaned up automatically via Let's Encrypt through a TLS challenge.


## Deployment

**Warning: Some Caveats Apply**  
> Currently, the deployment requires you to have some knowledge of system administration as well as understanding and building code,
> so you can eventually edit non-configurable and codeberg-specific settings.
> In the future, we'll try to reduce these and make hosting Codeberg Pages as easy as setting up Gitea.
> If you consider using Pages in practice, please consider contacting us first,
> we'll then try to share some basic steps and document the current usage for admins
> (might be changing in the current state).

Deploying the software itself is very easy. You can grab a current release binary or build yourself,
configure the environment as described below, and you are done.

The hard part is about adding **custom domain support** if you intend to use it.
SSL certificates (request + renewal) is automatically handled by the Pages Server,
but if you want to run it on a shared IP address (and not a standalone),
you'll need to configure your reverse proxy not to terminate the TLS connections,
but forward the requests on the IP level to the Pages Server.

You can check out a proof of concept in the `haproxy-sni` folder,
and especially have a look at [this section of the haproxy.cfg](https://codeberg.org/Codeberg/pages-server/src/branch/main/haproxy-sni/haproxy.cfg#L38).

### Environment

- `HOST` & `PORT` (default: `[::]` & `443`): listen address.
- `PAGES_DOMAIN` (default: `codeberg.page`): main domain for pages.
- `RAW_DOMAIN` (default: `raw.codeberg.page`): domain for raw resources.
- `GITEA_ROOT` (default: `https://codeberg.org`): root of the upstream Gitea instance.
- `GITEA_API_TOKEN` (default: empty): API token for the Gitea instance to access non-public (e.g. limited) repos.
- `RAW_INFO_PAGE` (default: https://docs.codeberg.org/pages/raw-content/): info page for raw resources, shown if no resource is provided.
- `ACME_API` (default: https://acme-v02.api.letsencrypt.org/directory): set this to https://acme.mock.director to use invalid certificates without any verification (great for debugging).  
  ZeroSSL might be better in the future as it doesn't have rate limits and doesn't clash with the official Codeberg certificates (which are using Let's Encrypt), but I couldn't get it to work yet.
- `ACME_EMAIL` (default: `noreply@example.email`): Set this to "true" to accept the Terms of Service of your ACME provider.
- `ACME_EAB_KID` &  `ACME_EAB_HMAC` (default: don't use EAB): EAB credentials, for example for ZeroSSL.
- `ACME_ACCEPT_TERMS` (default: use self-signed certificate): Set this to "true" to accept the Terms of Service of your ACME provider.
- `ACME_USE_RATE_LIMITS` (default: true): Set this to false to disable rate limits, e.g. with ZeroSSL.
- `ENABLE_HTTP_SERVER` (default: false): Set this to true to enable the HTTP-01 challenge and redirect all other HTTP requests to HTTPS. Currently only works with port 80.
- `DNS_PROVIDER` (default: use self-signed certificate): Code of the ACME DNS provider for the main domain wildcard.  
  See https://go-acme.github.io/lego/dns/ for available values & additional environment variables.
- `DEBUG` (default: false): Set this to true to enable debug logging.


## Contributing to the development

The Codeberg team is very open to your contribution.
Since we are working nicely in a team, it might be hard at times to get started
(still check out the issues, we always aim to have some things to get you started).

If you have any questions, want to work on a feature or could imagine collaborating with us for some time,
feel free to ping us in an issue or in a general Matrix chatgroup.

You can also contact the maintainers of this project:

- [momar](https://codeberg.org/momar) [(Matrix)](https://matrix.to/#/@moritz:wuks.space)
- [6543](https://codeberg.org/6543) [(Matrix)](https://matrix.to/#/@marddl:obermui.de)

### First steps

The code of this repository is split in several modules.
While heavy refactoring work is currently undergo, you can easily understand the basic structure:
The `cmd` folder holds the data necessary for interacting with the service via the cli.
If you are considering to deploy the service yourself, make sure to check it out.
The heart of the software lives in the `server` folder and is split in several modules.
After scanning the code, you should quickly be able to understand their function and start hacking on them.

Again: Feel free to get in touch with us for any questions that might arise.
Thank you very much.
